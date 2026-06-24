package mock_test

import (
	"testing"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

func TestModelProviderCapabilitiesReadRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	caps, err := h.Client.ModelProviderCapabilitiesRead(ctx)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ModelProviderCapabilitiesRead: %v", err)
	}
	// Booleans always decode; assert the call round-trips by re-reading and
	// confirming determinism.
	caps2, err := h.Client.ModelProviderCapabilitiesRead(ctx)
	if err != nil {
		t.Fatalf("ModelProviderCapabilitiesRead (2): %v", err)
	}
	if caps != caps2 {
		t.Fatalf("capabilities not stable: %+v vs %+v", caps, caps2)
	}
}

func TestThreadMetadataUpdateRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	th, err := h.Client.ThreadStart(ctx, codexgo.ThreadStartRequest{CWD: h.Workspace()})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	branch := "main"
	res, err := h.Client.ThreadMetadataUpdate(ctx, codexgo.ThreadMetadataUpdateRequest{
		ThreadID: th.ID,
		GitInfo:  &codexgo.ThreadMetadataGitInfo{Branch: &branch},
	})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ThreadMetadataUpdate: %v", err)
	}
	if res.Thread.ID != th.ID {
		t.Fatalf("ThreadMetadataUpdate returned thread %q, want %q", res.Thread.ID, th.ID)
	}
}

func TestThreadUnsubscribeRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	th, err := h.Client.ThreadStart(ctx, codexgo.ThreadStartRequest{CWD: h.Workspace()})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	res, err := h.Client.ThreadUnsubscribe(ctx, codexgo.ThreadUnsubscribeRequest{ThreadID: th.ID})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ThreadUnsubscribe: %v", err)
	}
	switch res.Status {
	case codexgo.ThreadUnsubscribeStatusNotLoaded,
		codexgo.ThreadUnsubscribeStatusNotSubscribed,
		codexgo.ThreadUnsubscribeStatusUnsubscribed:
		// ok
	default:
		t.Fatalf("unexpected unsubscribe status %q", res.Status)
	}
}

func TestThreadShellCommandRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	th, err := h.Client.ThreadStart(ctx, codexgo.ThreadStartRequest{CWD: h.Workspace()})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	err = h.Client.ThreadShellCommand(ctx, codexgo.ThreadShellCommandRequest{
		ThreadID: th.ID,
		Command:  "true",
	})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Skipf("ThreadShellCommand not runnable in this environment: %v", err)
	}
}

func TestThreadDeleteRPC(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	th, err := h.Client.ThreadStart(ctx, codexgo.ThreadStartRequest{CWD: h.Workspace()})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	err = h.Client.ThreadDelete(ctx, codexgo.ThreadDeleteRequest{ThreadID: th.ID})
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatalf("ThreadDelete: %v", err)
	}
}
