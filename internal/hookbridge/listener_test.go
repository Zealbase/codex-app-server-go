package hookbridge_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/zealbase/codex-app-server-go/internal/hookbridge"
)

func TestListenerAllowRoundTrip(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook.sock")

	// 1. Start a Listener with a simple allow-all handler.
	l, err := hookbridge.New(socketPath, func(_ context.Context, req hookbridge.HookRequest) hookbridge.HookResponse {
		return hookbridge.HookResponse{Decision: hookbridge.DecisionAllow}
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	// 2. Dial the socket, send a PreToolUse request, read the response.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := hookbridge.HookRequest{
		Type:     hookbridge.HookPreToolUse,
		Tool:     "shell",
		Input:    json.RawMessage(`{"cmd":"ls"}`),
		ThreadID: "thread-1",
		TurnID:   "turn-1",
	}
	reqBytes, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", reqBytes)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	var resp hookbridge.HookResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Decision != hookbridge.DecisionAllow {
		t.Errorf("expected decision %q, got %q", hookbridge.DecisionAllow, resp.Decision)
	}

	// 3. Close the listener and verify Close() returns nil.
	conn.Close()
	if err := l.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestListenerBlock(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook-block.sock")

	l, err := hookbridge.New(socketPath, func(_ context.Context, req hookbridge.HookRequest) hookbridge.HookResponse {
		return hookbridge.HookResponse{
			Decision: hookbridge.DecisionBlock,
			Reason:   "forbidden command",
		}
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := hookbridge.HookRequest{
		Type:  hookbridge.HookPreToolUse,
		Tool:  "shell",
		Input: json.RawMessage(`{"cmd":"rm -rf /"}`),
	}
	reqBytes, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", reqBytes)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	var resp hookbridge.HookResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Decision != hookbridge.DecisionBlock {
		t.Errorf("expected decision %q, got %q", hookbridge.DecisionBlock, resp.Decision)
	}
	if resp.Reason != "forbidden command" {
		t.Errorf("expected reason %q, got %q", "forbidden command", resp.Reason)
	}
}

func TestListenerSocketPath(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook-path.sock")
	l, err := hookbridge.New(socketPath, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	if l.SocketPath() != socketPath {
		t.Errorf("SocketPath() = %q, want %q", l.SocketPath(), socketPath)
	}
}

func TestListenerConfigSnippet(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook-cfg.sock")
	l, err := hookbridge.New(socketPath, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	snippet := l.ConfigSnippet()
	if len(snippet) == 0 {
		t.Error("ConfigSnippet returned empty string")
	}
	// Should contain the socket path.
	if !containsStr(snippet, socketPath) {
		t.Errorf("ConfigSnippet does not contain socket path %q:\n%s", socketPath, snippet)
	}
}

func TestListenerRemovesStaleSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook-stale.sock")
	// Pre-create a file at the socket path to simulate a stale socket.
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("create stale file: %v", err)
	}
	f.Close()

	l, err := hookbridge.New(socketPath, nil)
	if err != nil {
		t.Fatalf("New should succeed after removing stale socket: %v", err)
	}
	l.Close()
}

func TestListenerNilHandlerDefaultsAllow(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "hook-nil.sock")
	l, err := hookbridge.New(socketPath, nil) // nil handler
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	req := hookbridge.HookRequest{Type: hookbridge.HookSessionStart, ThreadID: "t1"}
	reqBytes, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", reqBytes)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}
	var resp hookbridge.HookResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Decision != hookbridge.DecisionAllow {
		t.Errorf("nil handler: expected allow, got %q", resp.Decision)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSub(s, sub))
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
