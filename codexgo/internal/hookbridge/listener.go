// Package hookbridge implements a Unix-domain socket server that receives
// Codex hook callbacks (PreToolUse, PostToolUse, SessionStart) and dispatches
// them to a registered Go Handler.
package hookbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
)

// HookType identifies which lifecycle hook fired.
type HookType string

const (
	HookPreToolUse   HookType = "PreToolUse"
	HookPostToolUse  HookType = "PostToolUse"
	HookSessionStart HookType = "SessionStart"
)

// HookDecision is the response sent back to Codex.
type HookDecision string

const (
	DecisionAllow HookDecision = "allow"
	DecisionBlock HookDecision = "block"
)

// HookRequest is received from Codex over the socket.
type HookRequest struct {
	Type     HookType        `json:"type"`
	Tool     string          `json:"tool,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Output   json.RawMessage `json:"output,omitempty"`
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
}

// HookResponse is sent back to Codex.
type HookResponse struct {
	Decision       HookDecision `json:"decision"`
	Reason         string       `json:"reason,omitempty"`
	OutputOverride string       `json:"outputOverride,omitempty"`
}

// Handler is called for each hook invocation. It must return quickly.
type Handler func(ctx context.Context, req HookRequest) HookResponse

// Listener is a Unix socket server that receives Codex hook callbacks.
type Listener struct {
	path    string // socket path
	handler Handler
	ln      net.Listener
	wg      sync.WaitGroup
	once    sync.Once
	done    chan struct{}
}

// New creates a Listener on socketPath and starts accepting connections.
// If socketPath already exists it is removed first.
func New(socketPath string, handler Handler) (*Listener, error) {
	// Remove stale socket file if present.
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("hookbridge: remove stale socket %q: %w", socketPath, err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("hookbridge: listen on %q: %w", socketPath, err)
	}

	l := &Listener{
		path:    socketPath,
		handler: handler,
		ln:      ln,
		done:    make(chan struct{}),
	}

	go l.serve()
	return l, nil
}

// SocketPath returns the path of the Unix socket.
func (l *Listener) SocketPath() string { return l.path }

// Close stops the listener and waits for all in-flight connections to finish.
func (l *Listener) Close() error {
	var closeErr error
	l.once.Do(func() {
		closeErr = l.ln.Close()
		close(l.done)
	})
	l.wg.Wait()
	return closeErr
}

// ConfigSnippet returns the TOML config fragment that wires this listener
// into a Codex config.toml so that Codex will call it for every hook event.
func (l *Listener) ConfigSnippet() string {
	return fmt.Sprintf(`[hooks]
  [[hooks.PreToolUse]]
    command = ["codex-sdk-hook-shim", "--socket", %q]
  [[hooks.PostToolUse]]
    command = ["codex-sdk-hook-shim", "--socket", %q]
  [[hooks.SessionStart]]
    command = ["codex-sdk-hook-shim", "--socket", %q]
`, l.path, l.path, l.path)
}

// serve loops accepting connections until the listener is closed.
func (l *Listener) serve() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			// Accept returns an error when the listener is closed; stop serving.
			select {
			case <-l.done:
				return
			default:
				// Transient error — keep accepting.
				continue
			}
		}
		l.wg.Add(1)
		go l.handleConn(conn)
	}
}

// handleConn reads one JSON line from conn, dispatches to the handler, and
// writes back a JSON response line. Hooks are one-shot: one request per
// connection.
func (l *Listener) handleConn(conn net.Conn) {
	defer l.wg.Done()
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		// Empty or broken connection — respond with allow and return.
		writeResponse(conn, HookResponse{Decision: DecisionAllow})
		return
	}

	var req HookRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		// Malformed JSON — default allow to avoid stalling Codex.
		writeResponse(conn, HookResponse{Decision: DecisionAllow})
		return
	}

	resp := l.dispatch(req)
	writeResponse(conn, resp)
}

// dispatch calls the registered handler (if any) and fills defaults.
func (l *Listener) dispatch(req HookRequest) HookResponse {
	if l.handler == nil {
		return HookResponse{Decision: DecisionAllow}
	}
	resp := l.handler(context.Background(), req)
	// Default to allow if the handler returned a zero-value decision.
	if resp.Decision == "" {
		resp.Decision = DecisionAllow
	}
	return resp
}

// writeResponse marshals resp as a single JSON line and writes it to w.
func writeResponse(w net.Conn, resp HookResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		// Should never happen with a well-formed struct.
		data = []byte(`{"decision":"allow"}`)
	}
	data = append(data, '\n')
	_, _ = w.Write(data)
}
