// Package testutil provides a lightweight mock JSON-RPC 2.0 server for use in
// SDK integration tests. It uses only encoding/json and bufio — NOT jrpc2 —
// so tests can exercise the full SDK transport stack independently.
package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
)

// rpcMessage is the common envelope for incoming JSON-RPC messages.
type rpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MethodHandler is a function that handles an incoming RPC call.
// It receives the raw params JSON and must return a result value (will be JSON-marshalled)
// or an error. Return (nil, nil) to send a null result.
type MethodHandler func(params json.RawMessage) (any, error)

// MockServer is a mock JSON-RPC 2.0 server that communicates over a net.Conn pair.
// It reads newline-delimited JSON from the client, dispatches registered handlers,
// and writes responses back.
type MockServer struct {
	mu       sync.Mutex
	handlers map[string]MethodHandler
	conn     net.Conn
	writer   *json.Encoder
	writerMu sync.Mutex
	done     chan struct{}
	doneOnce sync.Once
	wg       sync.WaitGroup

	// pendingReplies maps request ID → channel that receives the client's response.
	pendingReplies map[int]chan json.RawMessage
}

// NewMockServer creates a MockServer and returns the client-side net.Conn.
// The server starts reading immediately in a background goroutine.
// Close the returned MockServer to stop it.
func NewMockServer() (*MockServer, net.Conn) {
	clientConn, serverConn := net.Pipe()
	s := &MockServer{
		handlers:       make(map[string]MethodHandler),
		conn:           serverConn,
		done:           make(chan struct{}),
		pendingReplies: make(map[int]chan json.RawMessage),
	}
	enc := json.NewEncoder(serverConn)
	enc.SetEscapeHTML(false)
	s.writer = enc

	s.wg.Add(1)
	go s.readLoop()

	return s, clientConn
}

// Handle registers a handler for the given method name.
// Must be called before the server receives that method (typically before starting).
func (s *MockServer) Handle(method string, h MethodHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

// Notify sends a server-initiated notification (no ID) to the client.
func (s *MockServer) Notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	return s.writeJSON(msg)
}

// Request sends a server-initiated request (with an ID) to the client.
// It does NOT wait for the reply. Returns immediately after sending.
func (s *MockServer) Request(ctx context.Context, id int, method string, params any) (json.RawMessage, error) {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      id,
	}
	if params != nil {
		msg["params"] = params
	}
	return json.RawMessage(`{}`), s.writeJSON(msg)
}

// RequestAndWait sends a server-initiated request to the client and blocks until
// the client replies or ctx is done. Returns the raw result payload.
func (s *MockServer) RequestAndWait(ctx context.Context, id int, method string, params any) (json.RawMessage, error) {
	replyCh := make(chan json.RawMessage, 1)
	s.mu.Lock()
	s.pendingReplies[id] = replyCh
	s.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      id,
	}
	if params != nil {
		msg["params"] = params
	}
	if err := s.writeJSON(msg); err != nil {
		s.mu.Lock()
		delete(s.pendingReplies, id)
		s.mu.Unlock()
		return nil, err
	}

	select {
	case raw := <-replyCh:
		return raw, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pendingReplies, id)
		s.mu.Unlock()
		return nil, ctx.Err()
	case <-s.done:
		return nil, fmt.Errorf("mock server closed")
	}
}

// Close shuts down the server.
func (s *MockServer) Close() {
	s.doneOnce.Do(func() {
		close(s.done)
		s.conn.Close()
	})
	s.wg.Wait()
}

// Done returns a channel that is closed when the server has shut down.
func (s *MockServer) Done() <-chan struct{} {
	return s.done
}

func (s *MockServer) writeJSON(v any) error {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	return s.writer.Encode(v)
}

func (s *MockServer) readLoop() {
	defer s.wg.Done()
	scanner := bufio.NewScanner(s.conn)
	// Increase buffer size for larger messages.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		select {
		case <-s.done:
			return
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip malformed messages.
			continue
		}

		if msg.Method == "" {
			// This is a client-side response to a server request.
			// Check if there's a pending waiter for this ID.
			if len(msg.ID) > 0 {
				var numID int
				if err := json.Unmarshal(msg.ID, &numID); err == nil {
					s.mu.Lock()
					ch, ok := s.pendingReplies[numID]
					if ok {
						delete(s.pendingReplies, numID)
					}
					s.mu.Unlock()
					if ok {
						payload := msg.Result
						if len(payload) == 0 {
							payload = json.RawMessage(`null`)
						}
						select {
						case ch <- payload:
						default:
						}
					}
				}
			}
			continue
		}

		if len(msg.ID) == 0 {
			// Notification — fire-and-forget, no reply needed.
			s.mu.Lock()
			h := s.handlers[msg.Method]
			s.mu.Unlock()
			if h != nil {
				_, _ = h(msg.Params)
			}
			continue
		}

		// It's a request — dispatch and reply.
		s.mu.Lock()
		h := s.handlers[msg.Method]
		s.mu.Unlock()

		if h == nil {
			// Auto-handle session init methods silently with null result.
			if msg.Method == "initialize" || msg.Method == "initialized" || msg.Method == "session/start" {
				_ = s.replyResult(msg.ID, map[string]any{})
				continue
			}
			_ = s.replyError(msg.ID, -32601, fmt.Sprintf("method not found: %s", msg.Method))
			continue
		}

		result, err := h(msg.Params)
		if err != nil {
			_ = s.replyError(msg.ID, -32603, err.Error())
		} else {
			_ = s.replyResult(msg.ID, result)
		}
	}
}

func (s *MockServer) replyResult(id json.RawMessage, result any) error {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}
	return s.writeJSON(resp)
}

func (s *MockServer) replyError(id json.RawMessage, code int, message string) error {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	return s.writeJSON(resp)
}

// MustReadParams is a test helper that unmarshals params into v or panics.
func MustReadParams(params json.RawMessage, v any) {
	if len(params) == 0 {
		return
	}
	if err := json.Unmarshal(params, v); err != nil {
		panic(fmt.Sprintf("testutil.MustReadParams: %v", err))
	}
}

// MustMarshal marshals v to JSON or panics. Useful for building handler results.
func MustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("testutil.MustMarshal: %v", err))
	}
	return data
}

// ReadWriter adapts a net.Conn to io.ReadWriteCloser (it already satisfies the
// interface, but this alias makes intent explicit in tests).
func ReadWriter(conn net.Conn) io.ReadWriteCloser {
	return conn
}
