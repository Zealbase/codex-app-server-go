package transport

import (
	"context"
	"sync"
	"time"
)

// ReconnectingWS wraps a WebSocketTransport and automatically re-dials if the
// connection drops. Pending Call operations on a dropped connection fail;
// callers should layer RetryTransport on top to retry at the RPC level.
type ReconnectingWS struct {
	url  string
	opts []WSOption

	mu      sync.RWMutex
	current *WebSocketTransport

	notes    chan Notification
	requests chan *Request
	done     chan struct{}
	doneOnce sync.Once

	closedMu sync.Mutex
	closed   bool
}

// NewReconnectingWS dials url and returns a transport that re-dials on drop,
// reusing the same url and options for each reconnect.
func NewReconnectingWS(ctx context.Context, url string, opts ...WSOption) (*ReconnectingWS, error) {
	first, err := NewWebSocket(ctx, url, opts...)
	if err != nil {
		return nil, err
	}
	r := &ReconnectingWS{
		url:      url,
		opts:     opts,
		current:  first,
		notes:    make(chan Notification, 32),
		requests: make(chan *Request, 32),
		done:     make(chan struct{}),
	}
	go r.fanNotifications(first)
	go r.fanRequests(first)
	go r.watchLoop()
	return r, nil
}

func (r *ReconnectingWS) isClosed() bool {
	r.closedMu.Lock()
	defer r.closedMu.Unlock()
	return r.closed
}

func (r *ReconnectingWS) watchLoop() {
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

		// Re-dial, retrying with back-off until it succeeds or we close.
		var next *WebSocketTransport
		for {
			select {
			case <-time.After(backoff):
			case <-r.done:
				return
			}
			if r.isClosed() {
				return
			}

			backoff *= 2
			if max := reconnectMaxBackoff(); backoff > max {
				backoff = max
			}

			ws, err := NewWebSocket(context.Background(), r.url, r.opts...)
			if err == nil {
				next = ws
				break
			}
		}

		r.mu.Lock()
		r.current = next
		r.mu.Unlock()

		go r.fanNotifications(next)
		go r.fanRequests(next)

		backoff = reconnectInitialBackoff()
	}
}

func (r *ReconnectingWS) fanNotifications(t *WebSocketTransport) {
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

func (r *ReconnectingWS) fanRequests(t *WebSocketTransport) {
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
func (r *ReconnectingWS) Call(ctx context.Context, method string, params any, result any) error {
	r.mu.RLock()
	cur := r.current
	r.mu.RUnlock()
	return cur.Call(ctx, method, params, result)
}

// Notify delegates to the current inner transport.
func (r *ReconnectingWS) Notify(ctx context.Context, method string, params any) error {
	r.mu.RLock()
	cur := r.current
	r.mu.RUnlock()
	return cur.Notify(ctx, method, params)
}

// Requests returns the stable channel of server-initiated requests.
func (r *ReconnectingWS) Requests() <-chan *Request { return r.requests }

// Notifications returns the stable channel of server notifications.
func (r *ReconnectingWS) Notifications() <-chan Notification { return r.notes }

// Done is closed when the reconnecting transport is permanently closed.
func (r *ReconnectingWS) Done() <-chan struct{} { return r.done }

// Close permanently shuts down the transport and the current inner connection.
func (r *ReconnectingWS) Close() error {
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

// Ensure ReconnectingWS satisfies Transport at compile time.
var _ Transport = (*ReconnectingWS)(nil)
