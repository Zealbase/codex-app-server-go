package real_test

import (
	"context"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

func TestReal_InitializeServerInfo(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestReal_ThreadStartAndRead(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	thread, err := client.ThreadStart(ctx, codexgo.ThreadStartRequest{})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("ThreadStart returned empty thread ID")
	}

	read, err := client.ThreadRead(ctx, codexgo.ThreadReadRequest{ThreadID: thread.ID})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	if read.ID != thread.ID {
		t.Fatalf("ThreadRead returned ID %q, want %q", read.ID, thread.ID)
	}
}

func TestReal_ThreadSetName(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	thread, err := client.ThreadStart(ctx, codexgo.ThreadStartRequest{})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	const name = "go-real-test"
	if err := client.ThreadSetName(ctx, codexgo.ThreadSetNameRequest{ThreadID: thread.ID, Name: name}); err != nil {
		t.Fatalf("ThreadSetName: %v", err)
	}

	read, err := client.ThreadRead(ctx, codexgo.ThreadReadRequest{ThreadID: thread.ID})
	if err != nil {
		t.Fatalf("ThreadRead: %v", err)
	}
	if read.Name != name {
		t.Fatalf("thread name = %q, want %q", read.Name, name)
	}
}

func TestReal_ThreadList(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	thread, err := client.ThreadStart(ctx, codexgo.ThreadStartRequest{})
	if err != nil {
		t.Fatalf("ThreadStart: %v", err)
	}

	// Freshly started threads are in-memory but not yet persisted; use
	// ThreadLoadedList which returns live in-memory thread IDs.
	ids, err := client.ThreadLoadedList(ctx)
	if err != nil {
		t.Fatalf("ThreadLoadedList: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == thread.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ThreadLoadedList does not contain new thread %q; got %v", thread.ID, ids)
	}
}

func TestReal_ThreadArchiveAndUnarchive(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// A thread must have a rollout (at least one turn) before it can be archived.
	thread, err := client.StartThread(ctx, codexgo.WithThreadModel(realModel()))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	if _, err := thread.Run(ctx, "say ok"); err != nil {
		t.Fatalf("Run (pre-archive): %v", err)
	}
	id := thread.ID()

	if err := client.ThreadArchive(ctx, codexgo.ThreadArchiveRequest{ThreadID: id}); err != nil {
		t.Fatalf("ThreadArchive: %v", err)
	}

	// After archiving, the thread should no longer be in the loaded list.
	ids, err := client.ThreadLoadedList(ctx)
	if err != nil {
		t.Fatalf("ThreadLoadedList after archive: %v", err)
	}
	for _, loaded := range ids {
		if loaded == id {
			t.Fatalf("archived thread %q still present in loaded list", id)
		}
	}

	if err := client.ThreadUnarchive(ctx, codexgo.ThreadUnarchiveRequest{ThreadID: id}); err != nil {
		t.Fatalf("ThreadUnarchive: %v", err)
	}
}

func TestReal_Models(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	models, err := client.Models(ctx)
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("Models returned an empty list")
	}
}

func containsThreadID(threads []codexgo.Thread, id string) bool {
	for _, th := range threads {
		if th.ID == id {
			return true
		}
	}
	return false
}
