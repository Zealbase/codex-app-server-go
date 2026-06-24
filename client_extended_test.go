package codexgo

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

// TestInitializeCallFails verifies error handling when Call fails.
func TestInitializeCallFails(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ft.mu.Lock()
	ft.callErr = errors.New("transport failed")
	ft.mu.Unlock()

	_, err = client.Initialize(context.Background(), InitializeRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestInitializeNotifyFails verifies error handling when Notify fails.
func TestInitializeNotifyFails(t *testing.T) {
	ft := &fakeTransportWithErrors{
		result: InitializeResult{UserAgent: "ua"},
	}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ft.mu.Lock()
	ft.notifyErr = errors.New("notify failed")
	ft.mu.Unlock()

	_, err = client.Initialize(context.Background(), InitializeRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestThreadStart verifies ThreadStart calls the correct method.
func TestThreadStart(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Thread Thread `json:"thread"`
	}{Thread: Thread{ID: "thread-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.ThreadStart(context.Background(), ThreadStartRequest{Model: "gpt-5.1"})
	if err != nil {
		t.Fatalf("ThreadStart() error = %v", err)
	}
	if ft.callMethod != "thread/start" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
	if got.ID != "thread-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// TestThreadStartPassesThroughModel verifies the client passes any model name
// through to the server without client-side validation.
func TestThreadStartPassesThroughModel(t *testing.T) {
	ft := &fakeTransportWithErrors{
		callResultFunc: func(method string, _ any) any {
			if method == protocol.MethodThreadStart {
				return map[string]any{"thread": map[string]any{"id": "t-ok"}}
			}
			return nil
		},
	}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.ThreadStart(context.Background(), ThreadStartRequest{Model: "bogus-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "t-ok" {
		t.Fatalf("unexpected thread ID: %q", got.ID)
	}
	if ft.callMethod != protocol.MethodThreadStart {
		t.Fatalf("expected MethodThreadStart, got %q", ft.callMethod)
	}
}

// TestThreadResume verifies ThreadResume calls the correct method.
func TestThreadResume(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Thread Thread `json:"thread"`
	}{Thread: Thread{ID: "thread-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.ThreadResume(context.Background(), ThreadResumeRequest{ThreadID: "thread-1"})
	if err != nil {
		t.Fatalf("ThreadResume() error = %v", err)
	}
	if ft.callMethod != "thread/resume" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
	if got.ID != "thread-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// TestThreadRead verifies ThreadRead calls the correct method.
func TestThreadRead(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Thread Thread `json:"thread"`
	}{Thread: Thread{ID: "thread-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.ThreadRead(context.Background(), ThreadReadRequest{ThreadID: "thread-1"})
	if err != nil {
		t.Fatalf("ThreadRead() error = %v", err)
	}
	if ft.callMethod != "thread/read" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
	if got.ID != "thread-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// TestTurnStart verifies TurnStart calls the correct method.
func TestTurnStart(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Turn Turn `json:"turn"`
	}{Turn: Turn{ID: "turn-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.TurnStart(context.Background(), TurnStartRequest{ThreadID: "thread-1", Input: "hello"})
	if err != nil {
		t.Fatalf("TurnStart() error = %v", err)
	}
	if ft.callMethod != "turn/start" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
	if got.ID != "turn-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// TestTurnInterrupt verifies TurnInterrupt calls the correct method.
func TestTurnInterrupt(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.TurnInterrupt(context.Background(), TurnInterruptRequest{ThreadID: "thread-1", TurnID: "turn-1"})
	if err != nil {
		t.Fatalf("TurnInterrupt() error = %v", err)
	}
	if ft.callMethod != "turn/interrupt" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
}

// TestContextCancellationOnInitialize verifies context cancellation propagates.
func TestContextCancellationOnInitialize(t *testing.T) {
	ft := &fakeTransportWithErrors{result: InitializeResult{UserAgent: "ua"}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Initialize(ctx, InitializeRequest{})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestContextCancellationOnThreadStart verifies context cancellation on ThreadStart.
func TestContextCancellationOnThreadStart(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Thread Thread `json:"thread"`
	}{Thread: Thread{ID: "thread-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.ThreadStart(ctx, ThreadStartRequest{})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestClientCloseError verifies Close returns transport errors.
func TestClientCloseError(t *testing.T) {
	ft := &fakeTransportWithErrors{closeErr: errors.New("close failed")}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.Close()
	if err == nil {
		t.Fatal("expected error from Close")
	}
}

// TestClientCloseIsIdempotent verifies Close can be called multiple times.
func TestClientCloseIsIdempotent(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

// TestClientNilClose verifies that closing a nil client is safe.
func TestClientNilClose(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatalf("nil client Close() error = %v", err)
	}
}

// TestConcurrentCalls verifies the client handles concurrent requests safely.
func TestConcurrentCalls(t *testing.T) {
	ft := &fakeTransportWithErrors{result: struct {
		Thread Thread `json:"thread"`
	}{Thread: Thread{ID: "thread-1"}}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := client.ThreadStart(context.Background(), ThreadStartRequest{Model: "gpt-5.1"})
			if err != nil {
				t.Errorf("ThreadStart error: %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestServerInitializesBeforeHandler verifies request handler is set before use.
func TestServerInitializesBeforeHandler(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if ft.handler == nil {
		t.Fatal("expected handler to be installed")
	}
}

// TestRequestHandlerUpdateAfterInit verifies SetRequestHandler updates the handler.
func TestRequestHandlerUpdateAfterInit(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	client, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	oldHandler := ft.handler
	if oldHandler == nil {
		t.Fatal("expected initial handler")
	}

	newApprovals := testApprovals{}
	client2, _ := New(WithTransport(ft), WithApprovalHandler(newApprovals))
	defer client2.Close()

	if ft.handler == oldHandler {
		t.Fatalf("handler should have been updated")
	}
}
