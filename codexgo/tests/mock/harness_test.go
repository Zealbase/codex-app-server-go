package mock_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// Harness wires the public Go SDK to a real codex app-server process that is in
// turn pointed at an in-process MockResponsesServer. No real API key is needed.
type Harness struct {
	t         *testing.T
	Responses *MockResponsesServer
	Client    *codexgo.Client
	workspace string
}

// Workspace returns the temp directory used as the default working directory.
func (h *Harness) Workspace() string {
	return h.workspace
}

// NewHarness builds a fully-initialised Harness. The test is skipped if CODEX_BIN
// is not set. All resources are torn down via t.Cleanup.
func NewHarness(t *testing.T, opts ...codexgo.Option) *Harness {
	t.Helper()

	bin := os.Getenv("CODEX_BIN")
	if bin == "" {
		t.Skip("CODEX_BIN not set")
	}

	responses := newMockResponsesServer(t)
	t.Cleanup(responses.Close)

	tempRoot, err := os.MkdirTemp("", "codex-go-mock-*")
	if err != nil {
		t.Fatalf("NewHarness: mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempRoot) })

	codexHome := filepath.Join(tempRoot, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("NewHarness: mkdir codex-home: %v", err)
	}
	workspace := filepath.Join(tempRoot, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("NewHarness: mkdir workspace: %v", err)
	}

	configTOML := fmt.Sprintf(`model = "mock-model"
approval_policy = "never"
sandbox_mode = "read-only"
model_provider = "mock_provider"

[model_providers.mock_provider]
name = "Mock provider for Go SDK tests"
base_url = "%s/v1"
wire_api = "responses"
request_max_retries = 0
stream_max_retries = 0
`, responses.URL())

	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(configTOML), 0o600); err != nil {
		t.Fatalf("NewHarness: write config.toml: %v", err)
	}

	cmd := exec.Command(bin, "app-server", "--stdio",
		"--disable", "plugins",
		"-c", "features.plugins=false",
	)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(),
		"CODEX_HOME="+codexHome,
		"CODEX_APP_SERVER_DISABLE_MANAGED_CONFIG=1",
		"RUST_LOG=warn",
	)

	stderrBuf := &safeBuf{}
	cmd.Stderr = stderrBuf

	serverStdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("NewHarness: StdoutPipe: %v", err)
	}
	serverStdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("NewHarness: StdinPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("NewHarness: start %v: %v", cmd.Args, err)
	}

	// From the client's perspective: read = server stdout, write = server stdin.
	clientOpts := append([]codexgo.Option{
		codexgo.WithStdioTransport(serverStdout, serverStdin),
	}, opts...)

	client, err := codexgo.New(clientOpts...)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("NewHarness: new client: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("=== codex app-server stderr (%d bytes) ===\n%s", stderrBuf.Len(), stderrBuf.String())
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx, codexgo.InitializeRequest{
		ClientInfo: codexgo.ClientInfo{
			Name:    "codex-go-sdk-test",
			Version: "0.0.1",
		},
		Capabilities: codexgo.Capabilities{
			ExperimentalAPI: true,
		},
	}); err != nil {
		t.Fatalf("NewHarness: Initialize: %v", err)
	}

	return &Harness{
		t:         t,
		Responses: responses,
		Client:    client,
		workspace: workspace,
	}
}

// safeBuf is a bytes.Buffer safe for concurrent writes (stderr capture).
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuf) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}
