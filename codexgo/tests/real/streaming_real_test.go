package real_test

import (
	"context"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

func TestReal_RunStreamed_Events(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	thread, err := client.StartThread(ctx, codexgo.WithThreadModel(realModel()))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	events, err := thread.RunStreamed(ctx, "say hi")
	if err != nil {
		t.Fatalf("RunStreamed: %v", err)
	}

	var count int
	for range events {
		count++
	}
	if count == 0 {
		t.Fatal("RunStreamed produced no events")
	}
}

func TestReal_RunStreamed_Cancel(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startCtx, startCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer startCancel()
	thread, err := client.StartThread(startCtx, codexgo.WithThreadModel(realModel()))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	events, err := thread.RunStreamed(streamCtx, "count to 1000 slowly")
	if err != nil {
		t.Fatalf("RunStreamed: %v", err)
	}

	// Read one event, then cancel and verify the channel closes promptly.
	select {
	case <-events:
	case <-time.After(60 * time.Second):
		t.Fatal("timed out waiting for first streamed event")
	}

	cancel()

	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // channel closed cleanly
			}
		case <-time.After(5 * time.Second):
			t.Fatal("channel did not close within 5s after cancel")
		}
	}
}
