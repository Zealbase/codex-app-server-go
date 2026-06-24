//go:build e2e

// Package e2e contains end-to-end tests for the codex-go SDK.
// They start a real codex app-server process (local or in Docker) and exercise
// the full JSON-RPC protocol through the public SDK surface.
//
// Run with:
//
//	go test -v -count=1 -timeout 300s -tags e2e ./tests/e2e/...
//
// Environment variables:
//
//	E2E_TARGET          "docker" (default) or "local"
//	CODEX_BIN           path to codex binary (local mode only, default "codex")
//	CODEX_E2E_IMAGE     Docker image (default "ghcr.io/openai/codex:latest")
//	OPENAI_API_KEY      forwarded into Docker container when set
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go"
)

// Terminal turn/thread status string constants (mirrors protocol/defs.go).
const (
	turnStatusCompleted   = "completed"
	turnStatusInterrupted = "interrupted"
	turnStatusFailed      = "failed"
	turnStatusInProgress  = "inProgress"
)

// serverHandle wraps a running codex app-server process and a connected SDK client.
type serverHandle struct {
	cmd    *exec.Cmd
	client *codexgo.Client
}

// startServer starts a codex app-server (local or inside Docker) and returns a
// handle with an already-constructed SDK client connected over stdio.
// The server process is killed and the client closed when the test ends.
func startServer(t *testing.T) *serverHandle {
	t.Helper()

	target := envOr("E2E_TARGET", "docker")

	var cmd *exec.Cmd
	switch target {
	case "local":
		codexBin := envOr("CODEX_BIN", "codex")
		homeDir, err := os.MkdirTemp("", "codex-e2e-home-*")
		if err != nil {
			t.Fatalf("startServer: mkdir temp home: %v", err)
		}
		homeDir, err = filepath.Abs(homeDir)
		if err != nil {
			t.Fatalf("startServer: abs temp home: %v", err)
		}
		t.Cleanup(func() {
			_ = os.RemoveAll(homeDir)
		})
		codexHome := filepath.Join(homeDir, ".codex")
		if err := os.MkdirAll(codexHome, 0o755); err != nil {
			t.Fatalf("startServer: mkdir temp codex home: %v", err)
		}
		hostHome, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("startServer: user home: %v", err)
		}
		copyIfExists(t, filepath.Join(hostHome, ".codex", "auth.json"), filepath.Join(codexHome, "auth.json"))
		copyIfExists(t, filepath.Join(hostHome, ".codex", "config.toml"), filepath.Join(codexHome, "config.toml"))
		cmd = exec.Command(codexBin, "--dangerously-bypass-approvals-and-sandbox", "app-server", "--stdio")
		cmd.Env = append(os.Environ(),
			"HOME="+homeDir,
			"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
			"XDG_DATA_HOME="+filepath.Join(homeDir, ".local", "share"),
			"XDG_CACHE_HOME="+filepath.Join(homeDir, ".cache"),
		)
		cmd.Args = append(cmd.Args, "-c", "hooks.enabled=false")

	default: // docker
		image := envOr("CODEX_E2E_IMAGE", "codex-e2e:local")
		home, _ := os.UserHomeDir()
		authFile := filepath.Join(home, ".codex", "auth.json")

		args := []string{
			"run", "--rm", "--interactive",
			// Mount only the host auth.json read-only for credentials.
			"-v", authFile + ":/root/.codex/auth.json:ro",
			// Anonymous Docker volumes for codex state and shell snapshots.
			"-v", "/root/.codex",
			"-v", "/root/.codex/shell_snapshots",
		}
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			args = append(args, "-e", "OPENAI_API_KEY="+key)
		}
		// Override Dockerfile CMD: disable sandbox in the containerized server and
		// run the stdio JSON-RPC app server.
		args = append(args, image,
			"--dangerously-bypass-approvals-and-sandbox",
			"app-server", "--stdio",
			"-c", "hooks.enabled=false",
		)
		cmd = exec.Command("docker", args...)
	}

	// Capture stderr so we can print it on test failure.
	stderrBuf := &safeBuf{}
	cmd.Stderr = stderrBuf

	serverStdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("startServer: StdoutPipe: %v", err)
	}
	serverStdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("startServer: StdinPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("startServer: Start %v: %v", cmd.Args, err)
	}

	// From the client's perspective:
	//   "stdin"  = what we READ  = server's stdout pipe
	//   "stdout" = what we WRITE = server's stdin pipe
	client, err := codexgo.New(
		codexgo.WithStdioTransport(serverStdout, serverStdin),
		codexgo.WithRequestHandler(autoApproveAll()),
	)
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("startServer: New client: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
		if t.Failed() {
			t.Logf("=== codex app-server stderr (%d bytes) ===\n%s", stderrBuf.Len(), stderrBuf.String())
		}
	})

	return &serverHandle{cmd: cmd, client: client}
}

func copyIfExists(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("copy %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// initialize performs the JSON-RPC initialize+initialized handshake.
func initialize(t *testing.T, client *codexgo.Client) codexgo.InitializeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := client.Initialize(ctx, codexgo.InitializeRequest{
		ClientInfo: codexgo.ClientInfo{
			Name:    "codex-go-e2e",
			Version: "0.0.1",
		},
		Capabilities: codexgo.Capabilities{
			ExperimentalAPI: true,
		},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return result
}

// startThread creates a new thread with auto-approval and returns its ID.
func startThread(t *testing.T, client *codexgo.Client) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	thread, err := client.ThreadStart(ctx, codexgo.ThreadStartRequest{
		ApprovalPolicy: "on-request",
		Model:          e2eModel(),
	})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("ThreadStart returned empty thread ID")
	}
	return thread.ID
}

// startTurn starts a turn on the given thread and returns the turn ID.
func startTurn(t *testing.T, client *codexgo.Client, threadID, input string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	turn, err := client.TurnStart(ctx, codexgo.TurnStartRequest{
		ThreadID: threadID,
		Input:    input,
		Model:    e2eModel(),
	})
	if err != nil {
		t.Fatalf("TurnStart(%q): %v", input, err)
	}
	if turn.ID == "" {
		t.Fatal("TurnStart returned empty turn ID")
	}
	return turn.ID
}

// waitForTurnStatus polls ThreadRead until turnID reaches wantStatus or timeout.
func waitForTurnStatus(ctx context.Context, client *codexgo.Client, threadID, turnID, wantStatus string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout + 5*time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		thread, err := client.ThreadRead(readCtx, codexgo.ThreadReadRequest{
			ThreadID:     threadID,
			IncludeTurns: true,
		})
		cancel()
		if err != nil {
			if isMaterializationPendingErr(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if errors.Is(err, context.DeadlineExceeded) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return fmt.Errorf("ThreadRead: %w", err)
		}
		for _, turn := range thread.Turns {
			if turn.ID != turnID {
				continue
			}
			status := string(turn.Status)
			if status == wantStatus {
				return nil
			}
			if status == turnStatusFailed {
				return fmt.Errorf("turn %s failed", turnID)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout: turn %s did not reach %q within %s (+5m grace)", turnID, wantStatus, timeout)
}

// readThreadWithTurns retries ThreadRead until the thread materializes enough to
// include turns. Some server builds briefly reject IncludeTurns reads while the
// rollout is still being materialized.
func readThreadWithTurns(ctx context.Context, client *codexgo.Client, threadID string, timeout time.Duration) (codexgo.Thread, error) {
	deadline := time.Now().Add(timeout + 5*time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return codexgo.Thread{}, ctx.Err()
		}
		thread, err := client.ThreadRead(ctx, codexgo.ThreadReadRequest{
			ThreadID:     threadID,
			IncludeTurns: true,
		})
		if err != nil {
			if isMaterializationPendingErr(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return codexgo.Thread{}, fmt.Errorf("ThreadRead: %w", err)
		}
		return thread, nil
	}
	return codexgo.Thread{}, fmt.Errorf("timeout: thread %s did not materialize within %s (+5m grace)", threadID, timeout)
}

// pollTurnFinished polls until the turn reaches any non-inProgress status and
// returns that status. Useful for interrupt tests where the final status is uncertain.
func pollTurnFinished(ctx context.Context, client *codexgo.Client, threadID, turnID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout + 5*time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		thread, err := client.ThreadRead(readCtx, codexgo.ThreadReadRequest{
			ThreadID:     threadID,
			IncludeTurns: true,
		})
		cancel()
		if err != nil {
			if isMaterializationPendingErr(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if errors.Is(err, context.DeadlineExceeded) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return "", fmt.Errorf("ThreadRead: %w", err)
		}
		for _, turn := range thread.Turns {
			if turn.ID == turnID {
				if s := string(turn.Status); s != turnStatusInProgress {
					return s, nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout: turn %s still inProgress after %s (+5m grace)", turnID, timeout)
}

// autoApproveAll returns a RequestHandler that accepts every server-initiated
// approval request. This prevents turns from blocking on tool-use confirmation.
func autoApproveAll() codexgo.RequestHandler {
	return codexgo.RequestHandlerFunc(func(_ context.Context, _ codexgo.ServerRequest) (codexgo.ServerResponse, error) {
		return codexgo.ServerResponse{Result: []byte(`{"decision":"accept"}`)}, nil
	})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func e2eModel() string {
	return envOr("E2E_MODEL", "gpt-5.1")
}

func isMaterializationPendingErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "includeTurns is unavailable before first user message") ||
		strings.Contains(msg, "not materialized yet") ||
		strings.Contains(msg, "rollout is empty") ||
		strings.Contains(msg, "failed to load thread history") ||
		strings.Contains(msg, "thread-store internal error")
}

// safeBuf is a bytes.Buffer safe for concurrent writes (stderr capture goroutine).
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
