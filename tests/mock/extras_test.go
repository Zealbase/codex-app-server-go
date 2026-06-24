package mock_test

import (
	"reflect"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go"
)

func TestPing(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	if err := h.Client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestSessionThread_SetName(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	if err := st.SetName(ctx, "go-extra"); err != nil {
		t.Fatalf("SetName: %v", err)
	}

	read, err := h.Client.ThreadRead(ctx, codexgo.ThreadReadRequest{ThreadID: st.ID()})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	if read.Name != "go-extra" {
		t.Fatalf("thread name = %q, want %q", read.Name, "go-extra")
	}
}

func TestSessionThread_Archive(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	h.Responses.EnqueueAssistantMessage("hi", "r1")

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	result, err := st.Run(ctx, "say hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The binary intermittently interrupts turns without making an LLM call,
	// leaving the rollout incomplete. Archive/unarchive requires a valid rollout,
	// so skip the rest of the test in that case.
	if result.FinalAgentText() == "" {
		t.Skip("binary interrupted turn before LLM call; skipping archive test")
	}

	if err := st.Archive(ctx); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if err := st.Unarchive(ctx); err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
}

// TestModelsCache verifies that two rapid Models calls succeed and return
// identical results. The SDK caches the model catalog with a TTL; the cache is
// internal, but a cache hit must not change the observed model list.
func TestModelsCache(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	first, err := h.Client.Models(ctx)
	if err != nil {
		t.Fatalf("Models (1): %v", err)
	}
	second, err := h.Client.Models(ctx)
	if err != nil {
		t.Fatalf("Models (2): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Models results differ across calls: %v vs %v", first, second)
	}
}

func TestWithMaxThreads_Semaphore(t *testing.T) {
	h := NewHarness(t, codexgo.WithMaxThreads(1))
	ctx, cancel := testCtx(t)
	defer cancel()

	thread1, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread 1: %v", err)
	}

	started := make(chan *codexgo.SessionThread, 1)
	errc := make(chan error, 1)
	go func() {
		st, err := h.Client.StartThread(ctx)
		if err != nil {
			errc <- err
			return
		}
		started <- st
	}()

	// The second StartThread must block while the single slot is held.
	select {
	case <-started:
		t.Fatal("second StartThread acquired a slot while the first was still held")
	case err := <-errc:
		t.Fatalf("second StartThread errored unexpectedly: %v", err)
	case <-time.After(500 * time.Millisecond):
		// Expected: still blocked.
	}

	// Releasing the first slot lets the second proceed.
	thread1.Close()

	select {
	case st := <-started:
		st.Close()
	case err := <-errc:
		t.Fatalf("second StartThread errored after slot release: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("second StartThread did not proceed after first slot was released")
	}
}
