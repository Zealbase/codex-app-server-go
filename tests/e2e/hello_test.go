//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestHelloWorld is the simplest smoke-test: start the app-server, send a
// single-turn prompt asking for "hello", and verify the turn completes.
func TestHelloWorld(t *testing.T) {
	h := startServer(t)
	initialize(t, h.client)

	threadID := startThread(t, h.client)
	turnID := startTurn(t, h.client, threadID, "Say hello.")

	ctx := context.Background()
	if err := waitForTurnStatus(ctx, h.client, threadID, turnID, turnStatusCompleted, 90*time.Second); err != nil {
		t.Fatalf("HelloWorld turn did not complete: %v", err)
	}
	t.Logf("HelloWorld: turn %s completed on thread %s", turnID, threadID)
}
