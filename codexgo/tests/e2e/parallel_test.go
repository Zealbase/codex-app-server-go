//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// TestParallelSessions creates several threads on a single app-server instance
// and starts a turn on each concurrently, verifying that all turns complete.
// This exercises the server's ability to multiplex threads over one connection.
func TestParallelSessions(t *testing.T) {
	const numSessions = 2

	h := startServer(t)
	initialize(t, h.client)

	type sessionResult struct {
		threadID string
		turnID   string
		err      error
	}

	// Phase 1: start all threads and turns concurrently.
	results := make([]sessionResult, numSessions)
	var startWg sync.WaitGroup
	for i := range numSessions {
		i := i
		startWg.Add(1)
		go func() {
			defer startWg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			thread, err := h.client.ThreadStart(ctx, codexgo.ThreadStartRequest{
				ApprovalPolicy: "on-request",
				Model:          e2eModel(),
			})
			if err != nil {
				results[i] = sessionResult{err: fmt.Errorf("session %d ThreadStart: %w", i, err)}
				return
			}

			turn, err := h.client.TurnStart(ctx, codexgo.TurnStartRequest{
				ThreadID: thread.ID,
				Input:    fmt.Sprintf("Number %d only.", i+1),
				Model:    e2eModel(),
			})
			if err != nil {
				results[i] = sessionResult{err: fmt.Errorf("session %d TurnStart: %w", i, err)}
				return
			}

			results[i] = sessionResult{threadID: thread.ID, turnID: turn.ID}
		}()
	}
	startWg.Wait()

	// Fail fast if any session didn't start.
	for i, r := range results {
		if r.err != nil {
			t.Errorf("session %d: %v", i, r.err)
		}
	}
	if t.Failed() {
		return
	}

	// Phase 2: wait for all turns to complete concurrently.
	pollErrs := make([]error, numSessions)
	var pollWg sync.WaitGroup
	ctx := context.Background()

	for i, r := range results {
		i, r := i, r
		pollWg.Add(1)
		go func() {
			defer pollWg.Done()
			pollErrs[i] = waitForTurnStatus(ctx, h.client, r.threadID, r.turnID, turnStatusCompleted, 60*time.Second)
		}()
	}
	pollWg.Wait()

	for i, err := range pollErrs {
		if err != nil {
			t.Errorf("session %d turn did not complete: %v", i, err)
		} else {
			t.Logf("session %d: thread %s / turn %s completed", i, results[i].threadID, results[i].turnID)
		}
	}
}
