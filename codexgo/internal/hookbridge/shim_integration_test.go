package hookbridge_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/hookbridge"
)

// TestShimIntegration builds the codex-sdk-hook-shim binary, starts a Listener,
// and verifies the shim forwards a payload over the socket and relays the
// response to stdout.
func TestShimIntegration(t *testing.T) {
	tmp := t.TempDir()
	shimBin := filepath.Join(tmp, "codex-sdk-hook-shim")

	build := exec.Command("go", "build", "-o", shimBin, "github.com/zealbase/codex-app-server-go/cmd/codex-sdk-hook-shim")
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build shim: %v\n%s", err, out)
	}

	socket := filepath.Join(tmp, "hook.sock")

	var (
		mu       sync.Mutex
		gotReq   hookbridge.HookRequest
		gotCount int
	)
	handler := func(_ context.Context, req hookbridge.HookRequest) hookbridge.HookResponse {
		mu.Lock()
		gotReq = req
		gotCount++
		mu.Unlock()
		return hookbridge.HookResponse{Decision: hookbridge.DecisionBlock, Reason: "nope"}
	}

	ln, err := hookbridge.New(socket, handler)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	defer ln.Close()

	payload := `{"type":"PreToolUse","tool":"bash","input":{"cmd":"ls"}}`
	cmd := exec.Command(shimBin, "--socket", socket)
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run shim: %v", err)
	}

	var resp hookbridge.HookResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", out, err)
	}
	if resp.Decision != hookbridge.DecisionBlock {
		t.Errorf("decision = %q, want %q", resp.Decision, hookbridge.DecisionBlock)
	}
	if resp.Reason != "nope" {
		t.Errorf("reason = %q, want %q", resp.Reason, "nope")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCount != 1 {
		t.Fatalf("handler called %d times, want 1", gotCount)
	}
	if gotReq.Type != hookbridge.HookPreToolUse {
		t.Errorf("req.Type = %q, want %q", gotReq.Type, hookbridge.HookPreToolUse)
	}
	if gotReq.Tool != "bash" {
		t.Errorf("req.Tool = %q, want %q", gotReq.Tool, "bash")
	}
}
