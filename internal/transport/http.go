package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// HTTPTransport implements Transport over HTTP POST (for RPC calls and
// notifications) plus Server-Sent Events (for server push).
type HTTPTransport struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string

	nextID   uint64
	mu       sync.Mutex
	pending  map[uint64]chan callResult
	requests chan *Request
	notes    chan Notification
	done     chan struct{}
	doneOnce sync.Once
	cancel   context.CancelFunc
}

// HTTPOption configures an HTTPTransport.
type HTTPOption func(*HTTPTransport)

// WithHTTPHeader adds a fixed header to every request (e.g. Authorization).
func WithHTTPHeader(key, value string) HTTPOption {
	return func(h *HTTPTransport) { h.headers[key] = value }
}

// WithHTTPBearerToken adds "Authorization: Bearer <token>" to every request.
func WithHTTPBearerToken(token string) HTTPOption {
	return WithHTTPHeader("Authorization", "Bearer "+token)
}

// WithHTTPAPIKey adds "X-API-Key: <key>" to every request.
func WithHTTPAPIKey(key string) HTTPOption {
	return WithHTTPHeader("X-API-Key", key)
}

// WithHTTPClient sets a custom *http.Client (default: http.DefaultClient).
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(h *HTTPTransport) { h.httpClient = c }
}

// NewHTTP creates an HTTPTransport targeting the given base URL and starts
// the SSE reader goroutine.
func NewHTTP(baseURL string, opts ...HTTPOption) *HTTPTransport {
	ctx, cancel := context.WithCancel(context.Background())
	h := &HTTPTransport{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
		headers:    make(map[string]string),
		pending:    make(map[uint64]chan callResult),
		requests:   make(chan *Request, 32),
		notes:      make(chan Notification, 32),
		done:       make(chan struct{}),
		cancel:     cancel,
	}
	for _, opt := range opts {
		opt(h)
	}
	go h.readEvents(ctx)
	return h
}

func (h *HTTPTransport) applyHeaders(req *http.Request) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
}

// post sends a JSON payload to baseURL + "/rpc" and returns the response body.
func (h *HTTPTransport) post(ctx context.Context, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/rpc", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	h.applyHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http transport: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

// Call sends a JSON-RPC request and decodes the result into result.
func (h *HTTPTransport) Call(ctx context.Context, method string, params any, result any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-h.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.mu.Unlock()

	respBody, err := h.post(ctx, requestEnvelope{Version: "2.0", Method: method, ID: id, Params: params})
	if err != nil {
		return err
	}

	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return err
	}
	if env.Error != nil {
		return env.Error
	}
	if result == nil || len(env.Result) == 0 {
		return nil
	}
	return json.Unmarshal(env.Result, result)
}

// Notify sends a JSON-RPC notification (no id, no reply expected).
func (h *HTTPTransport) Notify(ctx context.Context, method string, params any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-h.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := h.post(ctx, notificationEnvelope{Version: "2.0", Method: method, Params: params})
	return err
}

// readEvents connects to baseURL + "/events" and dispatches SSE messages.
func (h *HTTPTransport) readEvents(ctx context.Context) {
	defer h.closeDone()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/events", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	h.applyHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if !h.dispatch(ctx, []byte(data)) {
			return
		}
	}
}

// dispatch routes a single SSE JSON message. Returns false if the transport
// is shutting down.
func (h *HTTPTransport) dispatch(ctx context.Context, data []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return true
	}
	methodRaw, hasMethod := raw["method"]
	if !hasMethod {
		return true
	}
	var method string
	if err := json.Unmarshal(methodRaw, &method); err != nil {
		return true
	}
	params := cloneRaw(raw["params"])

	if idRaw, hasID := raw["id"]; hasID {
		req := &Request{
			replyCh: make(chan replyResult, 1),
			id:      cloneRaw(idRaw),
			method:  method,
			params:  params,
			ctx:     ctx,
		}
		select {
		case h.requests <- req:
		case <-h.done:
			return false
		case <-ctx.Done():
			return false
		}
		go h.awaitReply(ctx, req)
		return true
	}

	select {
	case h.notes <- Notification{Method: method, Params: params}:
		return true
	case <-h.done:
		return false
	case <-ctx.Done():
		return false
	}
}

// awaitReply waits for the handler to reply to a server-initiated request and
// POSTs the response back to baseURL + "/rpc".
func (h *HTTPTransport) awaitReply(ctx context.Context, req *Request) {
	var rr replyResult
	select {
	case rr = <-req.replyCh:
	case <-h.done:
		return
	case <-ctx.Done():
		return
	}

	reply := replyEnvelope{Version: "2.0", ID: cloneRaw(req.id)}
	if rr.isErr {
		rpcErr := &RPCError{Code: rr.code, Message: rr.msg}
		if rr.data != nil {
			if raw, err := json.Marshal(rr.data); err == nil {
				rpcErr.Data = raw
			}
		}
		reply.Error = rpcErr
	} else {
		reply.Result = rr.result
	}
	_, _ = h.post(ctx, reply)
}

// Requests returns inbound server-initiated requests.
func (h *HTTPTransport) Requests() <-chan *Request { return h.requests }

// Notifications returns inbound server notifications.
func (h *HTTPTransport) Notifications() <-chan Notification { return h.notes }

// Done is closed when the transport is permanently terminated.
func (h *HTTPTransport) Done() <-chan struct{} { return h.done }

// Close cancels the SSE stream and waits for the reader goroutine to exit.
func (h *HTTPTransport) Close() error {
	h.cancel()
	<-h.done
	return nil
}

func (h *HTTPTransport) closeDone() {
	h.doneOnce.Do(func() { close(h.done) })
}

// Ensure HTTPTransport satisfies Transport at compile time.
var _ Transport = (*HTTPTransport)(nil)
