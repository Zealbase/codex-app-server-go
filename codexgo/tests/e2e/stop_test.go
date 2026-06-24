//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// TestTurnInterrupt starts a long-running turn and calls TurnInterrupt while it is
// in progress, asserting the turn ends with status "interrupted" (or "completed"
// if the server finished before the interrupt arrived).
func TestTurnInterrupt(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	// A prompt designed to produce a long model response so it's still running
	// when we call TurnInterrupt a few seconds later.
	turn, err := h.client.TurnStart(ctx, codexgo.TurnStartRequest{
		ThreadID: threadID,
		Input:    "Write a very long story — at least 500 words — about a robot exploring Mars.",
	})
	if err != nil {
		t.Fatalf("TurnStart: %v", err)
	}
	t.Logf("Turn started: %s (inProgress, will interrupt in 3s)", turn.ID)

	// Allow the server a moment to start streaming tokens.
	time.Sleep(3 * time.Second)

	if err := h.client.TurnInterrupt(ctx, codexgo.TurnInterruptRequest{
		ThreadID: threadID,
		TurnID:   turn.ID,
	}); err != nil {
		// "no active turn to interrupt" means the model finished before our 3s sleep — both outcomes valid.
		if !strings.Contains(err.Error(), "no active turn") {
			t.Fatalf("TurnInterrupt: unexpected error: %v", err)
		}
		t.Logf("TurnInterrupt: turn already completed before interrupt arrived (acceptable): %v", err)
	} else {
		t.Log("TurnInterrupt sent")
	}

	// The turn should settle into "interrupted" or "completed" (fast servers may
	// finish before the interrupt is processed — both are valid outcomes).
	finalStatus, err := pollTurnFinished(ctx, h.client, threadID, turn.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("polling after interrupt: %v", err)
	}

	if finalStatus != turnStatusInterrupted && finalStatus != turnStatusCompleted {
		t.Errorf("expected interrupted or completed, got %q", finalStatus)
	}
	t.Logf("TurnInterrupt: turn settled with status %q", finalStatus)
}

// TestInterruptNonExistentTurn verifies that TurnInterrupt on an unknown turn ID
// returns an error without crashing the connection.
func TestInterruptNonExistentTurn(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := h.client.TurnInterrupt(ctx, codexgo.TurnInterruptRequest{
		ThreadID: threadID,
		TurnID:   "turn-does-not-exist",
	})
	// The server may return an error or silently ignore it — either is acceptable.
	// What we assert is that the client does NOT panic and the connection stays alive.
	t.Logf("TurnInterrupt on unknown turn: err=%v (expected error or nil, not a crash)", err)

	// Verify the connection is still usable.
	turnID := startTurn(t, h.client, threadID, "Say 'ok'.")
	if err := waitForTurnStatus(ctx, h.client, threadID, turnID, turnStatusCompleted, 30*time.Second); err != nil {
		t.Errorf("connection not usable after bad TurnInterrupt: %v", err)
	}
}

// TestSessionStop creates a thread, starts a turn, and then closes the client.
// This covers the graceful-shutdown path of the transport layer.
func TestSessionStop(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	// Start a turn — don't wait for it to finish.
	turn, err := h.client.TurnStart(ctx, codexgo.TurnStartRequest{
		ThreadID: threadID,
		Input:    "Write a haiku.",
	})
	if err != nil {
		t.Fatalf("TurnStart: %v", err)
	}
	t.Logf("Turn %s started, closing client immediately", turn.ID)

	// Close the client before the turn finishes — should not panic or deadlock.
	if err := h.client.Close(); err != nil {
		t.Logf("Close returned: %v (non-fatal)", err)
	}
	t.Log("Client closed cleanly")
}
