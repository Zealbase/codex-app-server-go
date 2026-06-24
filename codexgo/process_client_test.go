package codexgo_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// TestHelperProcess is not a real test: when invoked with GO_HELPER_PROCESS=1 it
// acts as a minimal newline-delimited JSON-RPC server over stdio, replying to
// any request with id by echoing an empty result. It exits on stdin EOF.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	w := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		var msg struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		// Notifications (no id) get no reply.
		if len(msg.ID) == 0 || string(msg.ID) == "null" {
			continue
		}
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(msg.ID),
			"result":  map[string]any{},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
		w.WriteByte('\n')
		w.Flush()
	}
}

func helperCommand() (string, []string) {
	return os.Args[0], []string{"-test.run=TestHelperProcess"}
}

// TestWithStdioProcess_EchoServer spawns the helper JSON-RPC server subprocess
// and verifies a real Initialize round-trip works over its stdio.
func TestWithStdioProcess_EchoServer(t *testing.T) {
	// The child inherits the parent env, so set helper mode before spawning.
	os.Setenv("GO_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_HELPER_PROCESS")

	bin, args := helperCommand()
	client, err := codexgo.New(codexgo.WithStdioProcess(bin, args...))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx, codexgo.InitializeRequest{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestWithStdioProcess_Shutdown verifies that Close terminates the subprocess.
func TestWithStdioProcess_Shutdown(t *testing.T) {
	os.Setenv("GO_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_HELPER_PROCESS")

	bin, args := helperCommand()
	client, err := codexgo.New(codexgo.WithStdioProcess(bin, args...))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- client.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s")
	}
}
