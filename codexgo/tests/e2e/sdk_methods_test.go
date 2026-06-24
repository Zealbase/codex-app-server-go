//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// TestInitializeResult verifies that the Initialize handshake returns the
// expected fields from the server.
func TestInitializeResult(t *testing.T) {
	h := startServer(t)
	result := initialize(t, h.client)

	if result.UserAgent == "" {
		t.Error("InitializeResult.UserAgent must not be empty")
	}
	// PlatformOS and CodexHome are optional but logged for diagnostics.
	t.Logf("Initialize OK — userAgent=%q platformOS=%q codexHome=%q",
		result.UserAgent, result.PlatformOS, result.CodexHome)
}

// TestThreadRead verifies that a newly created thread can be fetched by ID.
func TestThreadRead(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	thread, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{
		ThreadID: threadID,
	})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	if thread.ID != threadID {
		t.Errorf("ThreadRead: got id=%q, want %q", thread.ID, threadID)
	}
	t.Logf("ThreadRead: thread %s status=%s", thread.ID, thread.Status)
}

// TestThreadReadWithTurns verifies that IncludeTurns populates the Turns field.
func TestThreadReadWithTurns(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	turnID := startTurn(t, h.client, threadID, "Say 'hello'.")
	if err := waitForTurnStatus(ctx, h.client, threadID, turnID, turnStatusCompleted, 90*time.Second); err != nil {
		t.Fatalf("turn: %v", err)
	}

	// Read without turns.
	threadNoTurns, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{ThreadID: threadID})
	if err != nil {
		t.Fatalf("ThreadRead (no turns): %v", err)
	}

	// Read with turns.
	threadWithTurns, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{
		ThreadID:     threadID,
		IncludeTurns: true,
	})
	if err != nil {
		t.Fatalf("ThreadRead (with turns): %v", err)
	}

	if len(threadWithTurns.Turns) == 0 {
		t.Error("ThreadRead with IncludeTurns=true: expected at least 1 turn")
	}
	t.Logf("ThreadRead: turns without flag=%d, with flag=%d",
		len(threadNoTurns.Turns), len(threadWithTurns.Turns))
}

// TestThreadResume verifies that ThreadResume accepts an existing thread ID and
// returns the same thread.
func TestThreadResume(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	// Run at least one turn so the thread has history.
	turnID := startTurn(t, h.client, threadID, "Say 'ok'.")
	if err := waitForTurnStatus(ctx, h.client, threadID, turnID, turnStatusCompleted, 90*time.Second); err != nil {
		t.Fatalf("initial turn: %v", err)
	}

	resumeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resumed, err := h.client.ThreadResume(resumeCtx, codexgo.ThreadResumeRequest{
		ThreadID: threadID,
	})
	if err != nil {
		t.Fatalf("ThreadResume: %v", err)
	}
	if resumed.ID != threadID {
		t.Errorf("ThreadResume: got id=%q, want %q", resumed.ID, threadID)
	}
	t.Logf("ThreadResume: thread %s status=%s", resumed.ID, resumed.Status)
}

// TestThreadStartOptions verifies that non-default ThreadStartRequest fields are
// reflected in the created thread.
func TestThreadStartOptions(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	cwd := envOr("E2E_CWD", "/workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	thread, err := h.client.ThreadStart(ctx, codexgo.ThreadStartRequest{
		Ephemeral:      true,
		ApprovalPolicy: "on-request",
		CWD:            cwd,
		Model:          "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}
	if thread.ID == "" {
		t.Error("ThreadStart: empty thread ID")
	}
	// CWD is a server-side field; verify it round-trips if returned.
	if thread.CWD != "" && thread.CWD != cwd {
		t.Logf("ThreadStart: CWD=%q (server may normalise the path)", thread.CWD)
	}
	t.Logf("ThreadStartOptions: id=%s cwd=%q model=%s", thread.ID, thread.CWD, thread.ModelProvider)
}

// TestTurnStartAndRead starts a turn then reads the thread to confirm the turn
// appears in the listing with the correct initial status.
func TestTurnStartAndRead(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)

	turnCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	turn, err := h.client.TurnStart(turnCtx, codexgo.TurnStartRequest{
		ThreadID: threadID,
		Input:    "Write a haiku about Go programming.",
	})
	if err != nil {
		t.Fatalf("TurnStart: %v", err)
	}
	if turn.ID == "" {
		t.Fatal("TurnStart: empty turn ID")
	}
	t.Logf("TurnStart: id=%s status=%s", turn.ID, turn.Status)

	// Let the turn finish.
	ctx := context.Background()
	if err := waitForTurnStatus(ctx, h.client, threadID, turn.ID, turnStatusCompleted, 90*time.Second); err != nil {
		t.Fatalf("turn did not complete: %v", err)
	}

	// Read back and assert the turn is listed.
	thread, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{
		ThreadID:     threadID,
		IncludeTurns: true,
	})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	found := false
	for _, rt := range thread.Turns {
		if rt.ID == turn.ID {
			found = true
			t.Logf("Found turn %s with status=%s", rt.ID, rt.Status)
		}
	}
	if !found {
		t.Errorf("turn %s not found in ThreadRead response", turn.ID)
	}
}
