//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// TestMultiStepRun sends two sequential turns to the same thread and verifies
// the model retains context across turns (conversation history).
func TestMultiStepRun(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	// Turn 1: plant a number in the model's context.
	turn1ID := startTurn(t, h.client, threadID, "Remember 42. Say ok.")
	if err := waitForTurnStatus(ctx, h.client, threadID, turn1ID, turnStatusCompleted, 180*time.Second); err != nil {
		t.Fatalf("Turn 1: %v", err)
	}
	t.Logf("Turn 1 completed: %s", turn1ID)

	// Turn 2: verify the model remembers the number.
	turn2ID := startTurn(t, h.client, threadID, "What number? Reply with only the number.")
	if err := waitForTurnStatus(ctx, h.client, threadID, turn2ID, turnStatusCompleted, 180*time.Second); err != nil {
		t.Fatalf("Turn 2: %v", err)
	}
	t.Logf("Turn 2 completed: %s", turn2ID)

	// Read the thread and verify both turns are present.
	thread, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{
		ThreadID:     threadID,
		IncludeTurns: true,
	})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	if len(thread.Turns) < 2 {
		t.Errorf("expected >=2 turns in thread, got %d", len(thread.Turns))
	}
	t.Logf("MultiStep: thread %s has %d turns", threadID, len(thread.Turns))
}

// TestMultiStepWithTurnRead exercises ThreadRead after each turn to verify the
// turn list grows incrementally.
func TestMultiStepWithTurnRead(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	ctx := context.Background()

	prompts := []string{
		"Reply with step one.",
		"Reply with step two.",
		"Reply with step three.",
	}

	for i, prompt := range prompts {
		turnID := startTurn(t, h.client, threadID, prompt)
		if err := waitForTurnStatus(ctx, h.client, threadID, turnID, turnStatusCompleted, 180*time.Second); err != nil {
			t.Fatalf("step %d turn: %v", i+1, err)
		}

		thread, err := h.client.ThreadRead(ctx, codexgo.ThreadReadRequest{
			ThreadID:     threadID,
			IncludeTurns: true,
		})
		if err != nil {
			t.Fatalf("step %d ThreadRead: %v", i+1, err)
		}
		wantMin := i + 1
		if len(thread.Turns) < wantMin {
			t.Errorf("after step %d: expected >=%d turns, got %d", i+1, wantMin, len(thread.Turns))
		}
		t.Logf("after step %d: thread has %d turns", i+1, len(thread.Turns))
	}
}
