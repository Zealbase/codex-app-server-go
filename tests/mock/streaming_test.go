package mock_test

import (
	"testing"
)

func TestRunStreamed_ReceivesTurnCompleted(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	h.Responses.EnqueueSSE(sseAssistantMessage("hi", "r1"))

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	ch, err := st.RunStreamed(ctx, "hi")
	if err != nil {
		t.Fatalf("RunStreamed: %v", err)
	}

	sawCompleted := false
	for ev := range ch {
		if ev.Kind == "turn/completed" {
			sawCompleted = true
		}
	}
	if !sawCompleted {
		t.Fatal("did not receive a turn/completed event before channel closed")
	}
}

func TestRunStreamed_ChannelClosedOnCompletion(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	h.Responses.EnqueueSSE(sseAssistantMessage("hi", "r1"))

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	ch, err := st.RunStreamed(ctx, "hi")
	if err != nil {
		t.Fatalf("RunStreamed: %v", err)
	}

	sawCompleted := false
	for ev := range ch {
		if ev.Kind == "turn/completed" {
			sawCompleted = true
		}
	}
	if !sawCompleted {
		t.Fatal("did not receive a turn/completed event")
	}

	// Channel is drained; confirm it is closed (a second receive yields zero, !ok).
	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed after turn/completed")
	}
}
