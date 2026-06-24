package transport

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// mockInner is a Transport whose Call returns the next error from errs (or nil
// once exhausted), tracking how many times Call was invoked.
type mockInner struct {
	errs  []error
	calls int32
}

func (m *mockInner) Call(ctx context.Context, method string, params any, result any) error {
	n := atomic.AddInt32(&m.calls, 1)
	idx := int(n) - 1
	if idx < len(m.errs) {
		return m.errs[idx]
	}
	return nil
}

func (m *mockInner) Notify(ctx context.Context, method string, params any) error { return nil }
func (m *mockInner) Requests() <-chan *Request                                   { return nil }
func (m *mockInner) Notifications() <-chan Notification                          { return nil }
func (m *mockInner) Done() <-chan struct{}                                       { return nil }
func (m *mockInner) Close() error                                                { return nil }

func rateLimitErr() error {
	data, _ := json.Marshal(map[string]any{
		"codexErrorInfo": map[string]any{"httpStatusCode": 429},
	})
	return &RPCError{Code: -32000, Message: "rate limited", Data: data}
}

func fastConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Multiplier:  2.0,
		JitterFrac:  0.25,
	}
}

func TestRetryTransport_RetriesOnRateLimit(t *testing.T) {
	inner := &mockInner{errs: []error{rateLimitErr(), rateLimitErr()}}
	rt := NewRetryTransport(inner, fastConfig())

	if err := rt.Call(context.Background(), "m", nil, nil); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&inner.calls); got != 3 {
		t.Fatalf("expected 3 calls (2 failures + 1 success), got %d", got)
	}
}

func TestRetryTransport_NoRetryOnClosed(t *testing.T) {
	inner := &mockInner{errs: []error{ErrClosed}}
	rt := NewRetryTransport(inner, fastConfig())

	err := rt.Call(context.Background(), "m", nil, nil)
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if got := atomic.LoadInt32(&inner.calls); got != 1 {
		t.Fatalf("expected exactly 1 call, got %d", got)
	}
}

func TestRetryTransport_ExceedsMaxAttempts(t *testing.T) {
	errs := make([]error, 10)
	for i := range errs {
		errs[i] = rateLimitErr()
	}
	inner := &mockInner{errs: errs}
	cfg := fastConfig()
	cfg.MaxAttempts = 3
	rt := NewRetryTransport(inner, cfg)

	err := rt.Call(context.Background(), "m", nil, nil)
	if err == nil {
		t.Fatal("expected error after exceeding max attempts")
	}
	if !isRateLimited(err) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if got := atomic.LoadInt32(&inner.calls); got != 3 {
		t.Fatalf("expected exactly 3 calls (MaxAttempts), got %d", got)
	}
}
