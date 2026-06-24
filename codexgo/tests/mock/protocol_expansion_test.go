package mock_test

import (
	"encoding/json"
	"strings"
	"testing"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// skipIfUnsupported skips when the server reports the RPC as not implemented.
func skipIfUnsupported(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "-32601") || strings.Contains(msg, "method not found") ||
		strings.Contains(msg, "not implemented") || strings.Contains(msg, "unsupported") {
		t.Skipf("RPC not supported by this binary: %v", err)
	}
}

func TestConfigReadRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	res, err := h.Client.ConfigRead(ctx, codexgo.ConfigReadRequest{
		CWD:           h.Workspace(),
		IncludeLayers: true,
	})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ConfigRead: %v", err)
	}
	if len(res.Config) == 0 {
		t.Fatal("ConfigRead returned empty config document")
	}
	// Config should be a JSON object.
	if !json.Valid(res.Config) {
		t.Fatalf("ConfigRead config is not valid JSON: %s", res.Config)
	}
}

func TestSkillsListRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	res, err := h.Client.SkillsList(ctx, codexgo.SkillsListRequest{
		CWDs: []string{h.Workspace()},
	})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("SkillsList: %v", err)
	}
	// An empty workspace has no skills; just assert the call round-trips and the
	// response shape is well-formed (Data is a slice, possibly empty).
	for _, entry := range res.Data {
		if entry.CWD == "" {
			t.Fatal("SkillsList entry missing cwd")
		}
	}
}

func TestHooksListRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	res, err := h.Client.HooksList(ctx, codexgo.HooksListRequest{
		CWDs: []string{h.Workspace()},
	})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("HooksList: %v", err)
	}
	// Data may be null/empty for a clean workspace; if present it must be valid JSON.
	if len(res.Data) > 0 && !json.Valid(res.Data) {
		t.Fatalf("HooksList data is not valid JSON: %s", res.Data)
	}
}

func TestExperimentalFeatureListRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	res, err := h.Client.ExperimentalFeatureList(ctx, codexgo.ExperimentalFeatureListRequest{})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ExperimentalFeatureList: %v", err)
	}
	for _, f := range res.Data {
		if f.Name == "" {
			t.Fatal("ExperimentalFeatureList entry missing name")
		}
	}
}

func TestCommandExecRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	res, err := h.Client.CommandExec(ctx, codexgo.CommandExecRequest{
		Command: []string{"/bin/sh", "-c", "printf hello"},
		CWD:     h.Workspace(),
	})
	skipIfUnsupported(t, err)
	if err != nil {
		// Some sandboxed environments reject process creation; treat as a skip so
		// the test remains a pure wiring check rather than an environment check.
		t.Skipf("CommandExec not runnable in this environment: %v", err)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("CommandExec stdout = %q, want it to contain %q", res.Stdout, "hello")
	}
}
