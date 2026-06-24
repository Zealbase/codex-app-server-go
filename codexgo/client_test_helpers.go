package codexgo

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
)

// fakeTransportWithErrors wraps fakeTransport to support error injection and metrics.
type fakeTransportWithErrors struct {
	mu             sync.Mutex
	callMethod     string
	callParams     any
	notifyMethod   string
	handler        RequestHandler
	result         any
	callResultFunc func(method string, params any) any
	callErr        error
	notifyErr      error
	closeErr       error
	callCount      int32
	notifyCount    int32
	closeCount     int32
}

func (f *fakeTransportWithErrors) Call(ctx context.Context, method string, params any, result any) error {
	atomic.AddInt32(&f.callCount, 1)
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callMethod = method
	f.callParams = params
	if f.callErr != nil {
		return f.callErr
	}
	if f.callResultFunc != nil {
		if v := f.callResultFunc(method, params); v != nil && result != nil {
			return decodeFakeResult(method, v, result)
		}
	}
	if method == protocol.MethodModelList && result != nil {
		return decodeFakeResult(method, defaultModelListResult(), result)
	}
	if f.result != nil && result != nil {
		return decodeFakeResult(method, f.result, result)
	}
	return nil
}

func (f *fakeTransportWithErrors) Notify(ctx context.Context, method string, params any) error {
	atomic.AddInt32(&f.notifyCount, 1)
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifyMethod = method
	if f.notifyErr != nil {
		return f.notifyErr
	}
	return nil
}

func (f *fakeTransportWithErrors) SetRequestHandler(handler RequestHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = handler
}

func (f *fakeTransportWithErrors) Close() error {
	atomic.AddInt32(&f.closeCount, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closeErr != nil {
		return f.closeErr
	}
	return nil
}

func defaultModelListResult() any {
	return map[string]any{
		"models": []any{
			"gpt-4.1-mini",
			"gpt-5.1",
			"gpt-5.4-mini",
		},
	}
}

func decodeFakeResult(method string, source any, dest any) error {
	if source == nil || dest == nil {
		return nil
	}
	if wrapped, ok := fakeWrappedResult(method, source); ok {
		data, _ := json.Marshal(wrapped)
		return json.Unmarshal(data, dest)
	}
	data, _ := json.Marshal(source)
	return json.Unmarshal(data, dest)
}

func fakeWrappedResult(method string, source any) (any, bool) {
	switch method {
	case protocol.MethodThreadStart, protocol.MethodThreadResume, protocol.MethodThreadRead:
		if thread, ok := source.(Thread); ok {
			return map[string]any{"thread": thread}, true
		}
	case protocol.MethodTurnStart:
		if turn, ok := source.(Turn); ok {
			return map[string]any{"turn": turn}, true
		}
	case protocol.MethodModelList:
		return source, true
	}
	return nil, false
}
