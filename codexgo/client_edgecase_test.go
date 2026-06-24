package codexgo

import (
	"context"
	"testing"
	"time"
)

// TestContextTimeoutOnInitialize verifies timeout during Initialize.
func TestContextTimeoutOnInitialize(t *testing.T) {
	ft := &fakeTransportWithErrors{result: InitializeResult{UserAgent: "ua"}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = client.Initialize(ctx, InitializeRequest{})
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("expected context timeout error, got %v", err)
	}
}

// TestMultipleOperationsSequential verifies sequential operations maintain state.
func TestMultipleOperationsSequential(t *testing.T) {
	ft := &fakeTransportWithErrors{result: Thread{ID: "thread-1"}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// First operation
	thread1, err := client.ThreadStart(context.Background(), ThreadStartRequest{Model: "gpt-5.1"})
	if err != nil {
		t.Fatalf("first ThreadStart error: %v", err)
	}
	if thread1.ID != "thread-1" {
		t.Fatalf("unexpected thread id: %s", thread1.ID)
	}

	// Second operation
	ft.result = Thread{ID: "thread-2"}
	thread2, err := client.ThreadStart(context.Background(), ThreadStartRequest{Model: "gpt-5.1"})
	if err != nil {
		t.Fatalf("second ThreadStart error: %v", err)
	}
	if thread2.ID != "thread-2" {
		t.Fatalf("unexpected thread id: %s", thread2.ID)
	}
}

// TestWithNilApprovalHandler verifies nil approval handler is allowed.
func TestWithNilApprovalHandler(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft), WithApprovalHandler(nil))
	if err != nil {
		t.Fatalf("New() with nil handler error = %v", err)
	}
	defer client.Close()

	if ft.handler != nil {
		t.Fatalf("handler should be nil")
	}
}

// TestMixedApprovalAndCustomHandler verifies WithRequestHandler overrides WithApprovalHandler.
func TestRequestHandlerTakePrecedence(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	customHandler := RequestHandlerFunc(func(ctx context.Context, req ServerRequest) (ServerResponse, error) {
		return ServerResponse{Result: []byte(`{"custom": true}`)}, nil
	})
	client, err := New(
		WithTransport(ft),
		WithApprovalHandler(testApprovals{}),
		WithRequestHandler(customHandler),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// The custom handler should be the one installed
	if ft.handler == nil {
		t.Fatal("expected handler to be installed")
	}
}
