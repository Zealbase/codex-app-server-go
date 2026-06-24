package codexgo

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

// makeStdioPipes creates two io.Pipe pairs suitable for NewStdioTransport.
// The caller writes JSON-RPC messages to stdinW and reads responses from stdoutR.
func makeStdioPipes(t *testing.T) (stdinR *io.PipeReader, stdinW *io.PipeWriter, stdoutR *io.PipeReader, stdoutW *io.PipeWriter) {
	t.Helper()
	stdinR, stdinW = io.Pipe()
	stdoutR, stdoutW = io.Pipe()
	return
}

// decodeWithTimeout decodes one JSON value from dec into v.  It fails the test
// if the decode takes longer than 2 seconds.
func decodeWithTimeout(t *testing.T, dec *json.Decoder, v any) {
	t.Helper()
	errc := make(chan error, 1)
	go func() { errc <- dec.Decode(v) }()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for JSON response from stdio transport")
	}
}

// TestStdioTransportServerRequestRouting verifies that a server-initiated
// request is routed to the registered handler and the reply is written back
// through the stdout pipe.
func TestStdioTransportServerRequestRouting(t *testing.T) {
	stdinR, stdinW, stdoutR, stdoutW := makeStdioPipes(t)
	tr := NewStdioTransport(stdinR, stdoutW)
	defer func() {
		tr.Close()
		_ = stdinW.Close()
		_ = stdoutR.Close()
	}()

	tr.SetRequestHandler(RequestHandlerFunc(func(_ context.Context, _ ServerRequest) (ServerResponse, error) {
		return ServerResponse{Result: json.RawMessage(`{"decision":"accept"}`)}, nil
	}))

	enc := json.NewEncoder(stdinW)
	enc.SetEscapeHTML(false)
	dec := json.NewDecoder(stdoutR)

	// Send an inbound server→client request.
	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "item/commandExecution/requestApproval",
		"id":      1,
		"params":  map[string]any{"command": "ls"},
	}); err != nil {
		t.Fatalf("encode inbound request: %v", err)
	}

	var resp map[string]json.RawMessage
	decodeWithTimeout(t, dec, &resp)

	// Validate id and result.
	var id uint64
	if err := json.Unmarshal(resp["id"], &id); err != nil || id != 1 {
		t.Fatalf("unexpected response id: %s", resp["id"])
	}
	// error field must be absent (omitempty) or null.
	if errRaw, ok := resp["error"]; ok && string(errRaw) != "null" {
		t.Fatalf("unexpected error in response: %s", errRaw)
	}
	var result struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Decision != "accept" {
		t.Fatalf("expected decision=accept, got %q", result.Decision)
	}
}

// TestStdioTransportNilHandlerReturnsError verifies that when no request
// handler is installed, the transport replies with a JSON-RPC -32601 error.
func TestStdioTransportNilHandlerReturnsError(t *testing.T) {
	stdinR, stdinW, stdoutR, stdoutW := makeStdioPipes(t)
	tr := NewStdioTransport(stdinR, stdoutW)
	defer func() {
		tr.Close()
		_ = stdinW.Close()
		_ = stdoutR.Close()
	}()
	// Intentionally no handler installed.

	enc := json.NewEncoder(stdinW)
	enc.SetEscapeHTML(false)
	dec := json.NewDecoder(stdoutR)

	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "item/tool/requestUserInput",
		"id":      42,
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("encode inbound request: %v", err)
	}

	var resp map[string]json.RawMessage
	decodeWithTimeout(t, dec, &resp)

	var id uint64
	if err := json.Unmarshal(resp["id"], &id); err != nil || id != 42 {
		t.Fatalf("unexpected response id: %s", resp["id"])
	}
	if len(resp["error"]) == 0 {
		t.Fatal("expected 'error' field in response when no handler is installed")
	}
	var rpcErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp["error"], &rpcErr); err != nil {
		t.Fatalf("unmarshal error field: %v", err)
	}
	if rpcErr.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", rpcErr.Code)
	}
}

// TestStdioTransportCloseRejectsSubsequentCalls verifies that after Close,
// outgoing Call attempts fail immediately rather than blocking.
func TestStdioTransportCloseRejectsSubsequentCalls(t *testing.T) {
	stdinR, stdinW, stdoutR, stdoutW := makeStdioPipes(t)
	defer func() {
		_ = stdinW.Close()
		_ = stdoutR.Close()
	}()
	tr := NewStdioTransport(stdinR, stdoutW)

	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := tr.Call(context.Background(), "ping", nil, nil); err == nil {
		t.Fatal("expected an error from Call after Close, got nil")
	}
}

// TestWithStdioTransportOption verifies that the WithStdioTransport Option
// creates a fully wired transport: the installed request handler is reachable
// via the stdio pipes.
func TestWithStdioTransportOption(t *testing.T) {
	stdinR, stdinW, stdoutR, stdoutW := makeStdioPipes(t)
	defer func() {
		_ = stdinW.Close()
		_ = stdoutR.Close()
	}()

	enc := json.NewEncoder(stdinW)
	enc.SetEscapeHTML(false)
	dec := json.NewDecoder(stdoutR)

	client, err := New(
		WithStdioTransport(stdinR, stdoutW),
		WithRequestHandler(RequestHandlerFunc(func(_ context.Context, _ ServerRequest) (ServerResponse, error) {
			return ServerResponse{Result: json.RawMessage(`{"ok":true}`)}, nil
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if err := enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "item/permissions/requestApproval",
		"id":      99,
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("encode inbound request: %v", err)
	}

	var resp map[string]json.RawMessage
	decodeWithTimeout(t, dec, &resp)

	var id uint64
	if err := json.Unmarshal(resp["id"], &id); err != nil || id != 99 {
		t.Fatalf("unexpected response id: %s", resp["id"])
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil || !result.OK {
		t.Fatalf("unexpected result: %s", resp["result"])
	}
}
