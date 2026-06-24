package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"nhooyr.io/websocket"
)

// WebSocketTransport implements Transport over a full-duplex JSON-RPC 2.0
// WebSocket connection, as used by the Codex app-server WebSocket transport.
type WebSocketTransport struct {
	url        string
	headers    http.Header
	httpClient *http.Client

	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  uint64
	pending map[uint64]chan callResult

	requests chan *Request
	notes    chan Notification
	done     chan struct{}
	doneOnce sync.Once
	writeMu  sync.Mutex
	cancel   context.CancelFunc
}

// WSOption configures a WebSocketTransport during construction.
type WSOption func(*WebSocketTransport)

// WithWSHeader adds a fixed header to the WebSocket handshake (e.g. Authorization).
func WithWSHeader(key, value string) WSOption {
	return func(w *WebSocketTransport) {
		if w.headers == nil {
			w.headers = http.Header{}
		}
		w.headers.Add(key, value)
	}
}

// WithWSBearerToken adds "Authorization: Bearer <token>" to the WebSocket handshake.
func WithWSBearerToken(token string) WSOption {
	return WithWSHeader("Authorization", "Bearer "+token)
}

// WithWSAPIKey adds "X-API-Key: <key>" to the WebSocket handshake.
func WithWSAPIKey(key string) WSOption {
	return WithWSHeader("X-API-Key", key)
}

// WithWSHTTPClient sets a custom HTTP client for the WebSocket dial (e.g. for Unix socket dialing).
func WithWSHTTPClient(c *http.Client) WSOption {
	return func(w *WebSocketTransport) { w.httpClient = c }
}

// NewWebSocket dials the given ws:// URL and starts the read loop.
func NewWebSocket(ctx context.Context, url string, opts ...WSOption) (*WebSocketTransport, error) {
	w := &WebSocketTransport{
		url:      url,
		pending:  make(map[uint64]chan callResult),
		requests: make(chan *Request, 32),
		notes:    make(chan Notification, 32),
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: w.headers,
		HTTPClient: w.httpClient,
	})
	if err != nil {
		return nil, err
	}
	// Codex JSON-RPC envelopes can exceed the default 32KiB read limit.
	conn.SetReadLimit(-1)
	w.conn = conn

	loopCtx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.readLoop(loopCtx)
	return w, nil
}

func (w *WebSocketTransport) readLoop(ctx context.Context) {
	for {
		_, data, err := w.conn.Read(ctx)
		if err != nil {
			w.terminate()
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			// Skip malformed frames rather than tearing down the connection.
			continue
		}

		methodRaw, hasMethod := raw["method"]
		idRaw, hasID := raw["id"]

		if hasMethod {
			var method string
			if err := json.Unmarshal(methodRaw, &method); err != nil {
				continue
			}
			params := cloneRaw(raw["params"])
			if hasID {
				// Server-initiated request.
				replyCh := make(chan replyResult, 1)
				req := &Request{
					replyCh: replyCh,
					method:  method,
					params:  params,
					ctx:     ctx,
				}
				select {
				case w.requests <- req:
				case <-w.done:
					return
				}
				go w.awaitReply(ctx, cloneRaw(idRaw), replyCh)
			} else {
				// Notification.
				select {
				case w.notes <- Notification{Method: method, Params: params}:
				case <-w.done:
					return
				}
			}
			continue
		}

		if !hasID {
			continue
		}

		id, err := parseID(idRaw)
		if err != nil {
			continue
		}
		res := callResult{raw: cloneRaw(raw["result"])}
		if errRaw, ok := raw["error"]; ok {
			var rpcErr RPCError
			if err := json.Unmarshal(errRaw, &rpcErr); err != nil {
				continue
			}
			res.rpcErr = &rpcErr
		}
		w.deliverResponse(id, res)
	}
}

func (w *WebSocketTransport) awaitReply(ctx context.Context, id json.RawMessage, replyCh chan replyResult) {
	select {
	case rr := <-replyCh:
		env := replyEnvelope{Version: "2.0", ID: id}
		if rr.isErr {
			rpcErr := &RPCError{Code: rr.code, Message: rr.msg}
			if rr.data != nil {
				if raw, err := json.Marshal(rr.data); err == nil {
					rpcErr.Data = raw
				}
			}
			env.Error = rpcErr
		} else {
			env.Result = rr.result
		}
		_ = w.write(ctx, env)
	case <-w.done:
	case <-ctx.Done():
	}
}

func (w *WebSocketTransport) deliverResponse(id uint64, res callResult) {
	w.mu.Lock()
	ch, ok := w.pending[id]
	if ok {
		delete(w.pending, id)
	}
	w.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- res:
	case <-w.done:
	}
}

func (w *WebSocketTransport) write(ctx context.Context, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.conn.Write(ctx, websocket.MessageText, data)
}

// Call sends a JSON-RPC request and decodes the result into result.
func (w *WebSocketTransport) Call(ctx context.Context, method string, params any, result any) error {
	if method == "" {
		return ErrProtocol
	}
	select {
	case <-w.done:
		return ErrClosed
	default:
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	w.mu.Lock()
	w.nextID++
	id := w.nextID
	replyCh := make(chan callResult, 1)
	w.pending[id] = replyCh
	w.mu.Unlock()

	env := requestEnvelope{Version: "2.0", Method: method, ID: id, Params: params}
	if err := w.write(ctx, env); err != nil {
		w.removePending(id)
		return err
	}

	select {
	case <-ctx.Done():
		w.removePending(id)
		return ctx.Err()
	case res := <-replyCh:
		if res.err != nil {
			return res.err
		}
		if res.rpcErr != nil {
			return res.rpcErr
		}
		if result == nil || len(res.raw) == 0 {
			return nil
		}
		return json.Unmarshal(res.raw, result)
	case <-w.done:
		w.removePending(id)
		return ErrClosed
	}
}

// Notify sends a JSON-RPC notification (no reply expected).
func (w *WebSocketTransport) Notify(ctx context.Context, method string, params any) error {
	select {
	case <-w.done:
		return ErrClosed
	default:
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	env := notificationEnvelope{Version: "2.0", Method: method, Params: params}
	return w.write(ctx, env)
}

// Requests returns a channel of server-initiated requests.
func (w *WebSocketTransport) Requests() <-chan *Request { return w.requests }

// Notifications returns a channel of server notifications.
func (w *WebSocketTransport) Notifications() <-chan Notification { return w.notes }

// Done is closed when the transport is permanently terminated.
func (w *WebSocketTransport) Done() <-chan struct{} { return w.done }

// Close shuts down the transport and the underlying WebSocket connection.
func (w *WebSocketTransport) Close() error {
	select {
	case <-w.done:
		return nil
	default:
	}
	if w.cancel != nil {
		w.cancel()
	}
	err := w.conn.Close(websocket.StatusNormalClosure, "")
	w.terminate()
	return err
}

func (w *WebSocketTransport) terminate() {
	w.mu.Lock()
	pending := w.pending
	w.pending = make(map[uint64]chan callResult)
	w.mu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- callResult{err: ErrClosed}:
		default:
		}
	}
	w.doneOnce.Do(func() { close(w.done) })
}

func (w *WebSocketTransport) removePending(id uint64) {
	w.mu.Lock()
	delete(w.pending, id)
	w.mu.Unlock()
}

// Ensure WebSocketTransport satisfies Transport at compile time.
var _ Transport = (*WebSocketTransport)(nil)
