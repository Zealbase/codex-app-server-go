package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Back-off bounds for automatic reconnect, overridable in tests via
// setReconnectBackoff. Stored as atomics so test mutation is race-free against
// the watch-loop goroutines that read them.
var (
	reconnectInitialBackoffNanos atomic.Int64
	reconnectMaxBackoffNanos     atomic.Int64
)

func init() {
	reconnectInitialBackoffNanos.Store(int64(1 * time.Second))
	reconnectMaxBackoffNanos.Store(int64(30 * time.Second))
}

func reconnectInitialBackoff() time.Duration {
	return time.Duration(reconnectInitialBackoffNanos.Load())
}

func reconnectMaxBackoff() time.Duration {
	return time.Duration(reconnectMaxBackoffNanos.Load())
}

// ReconnectingHTTP wraps an HTTPTransport and automatically re-establishes the
// SSE connection if it drops. Pending Call operations on a dropped connection
// fail; callers should layer RetryTransport on top to retry at the RPC level.
type ReconnectingHTTP struct {
	baseURL string
	opts    []HTTPOption

	mu      sync.RWMutex
	current *HTTPTransport

	notes    chan Notification
	requests chan *Request
	done     chan struct{}
	doneOnce sync.Once

	closedMu sync.Mutex
	closed   bool
}

// NewReconnectingHTTP creates an HTTP transport that reconnects its SSE stream
// when it drops, using the same baseURL and options for each reconnect.
func NewReconnectingHTTP(baseURL string, opts ...HTTPOption) *ReconnectingHTTP {
	r := &ReconnectingHTTP{
		baseURL:  baseURL,
		opts:     opts,
		current:  NewHTTP(baseURL, opts...),
		notes:    make(chan Notification, 32),
		requests: make(chan *Request, 32),
		done:     make(chan struct{}),
	}
	go r.fanNotifications(r.current)
	go r.fanRequests(r.current)
	go r.watchLoop()
	return r
}

func (r *ReconnectingHTTP) isClosed() bool {
	r.closedMu.Lock()
	defer r.closedMu.Unlock()
	return r.closed
}

func (r *ReconnectingHTTP) watchLoop() {
	backoff := reconnectInitialBackoff()
	for {
		r.mu.RLock()
		cur := r.current
		r.mu.RUnlock()

		select {
		case <-cur.Done():
		case <-r.done:
			return
		}

		if r.isClosed() {
			return
		}

		select {
		case <-time.After(backoff):
		case <-r.done:
			return
		}
		if r.isClosed() {
			return
		}

		next := NewHTTP(r.baseURL, r.opts...)
		r.mu.Lock()
		r.current = next
		r.mu.Unlock()

		go r.fanNotifications(next)
		go r.fanRequests(next)

		backoff *= 2
		if max := reconnectMaxBackoff(); backoff > max {
			backoff = max
		}
	}
}

func (r *ReconnectingHTTP) fanNotifications(t *HTTPTransport) {
	src := t.Notifications()
	for {
		select {
		case n, ok := <-src:
			if !ok {
				return
			}
			select {
			case r.notes <- n:
			case <-r.done:
				return
			}
		case <-t.Done():
			return
		case <-r.done:
			return
		}
	}
}

func (r *ReconnectingHTTP) fanRequests(t *HTTPTransport) {
	src := t.Requests()
	for {
		select {
		case req, ok := <-src:
			if !ok {
				return
			}
			select {
			case r.requests <- req:
			case <-r.done:
				return
			}
		case <-t.Done():
			return
		case <-r.done:
			return
		}
	}
}

// Call delegates to the current inner transport.
func (r *ReconnectingHTTP) Call(ctx context.Context, method string, params any, result any) error {
	r.mu.RLock()
	cur := r.current
	r.mu.RUnlock()
	return cur.Call(ctx, method, params, result)
}

// Notify delegates to the current inner transport.
func (r *ReconnectingHTTP) Notify(ctx context.Context, method string, params any) error {
	r.mu.RLock()
	cur := r.current
	r.mu.RUnlock()
	return cur.Notify(ctx, method, params)
}

// Requests returns the stable channel of server-initiated requests.
func (r *ReconnectingHTTP) Requests() <-chan *Request { return r.requests }

// Notifications returns the stable channel of server notifications.
func (r *ReconnectingHTTP) Notifications() <-chan Notification { return r.notes }

// Done is closed when the reconnecting transport is permanently closed.
func (r *ReconnectingHTTP) Done() <-chan struct{} { return r.done }

// Close permanently shuts down the transport and the current inner connection.
func (r *ReconnectingHTTP) Close() error {
	r.closedMu.Lock()
	if r.closed {
		r.closedMu.Unlock()
		return nil
	}
	r.closed = true
	r.closedMu.Unlock()

	r.doneOnce.Do(func() { close(r.done) })

	r.mu.RLock()
	cur := r.current
	r.mu.RUnlock()
	return cur.Close()
}

// Ensure ReconnectingHTTP satisfies Transport at compile time.
var _ Transport = (*ReconnectingHTTP)(nil)
