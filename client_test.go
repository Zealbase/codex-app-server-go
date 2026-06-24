package codexgo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

type fakeTransport struct {
	callMethod     string
	callParams     any
	notifyMethod   string
	handler        RequestHandler
	result         any
	callResultFunc func(method string, params any) any
}

func (f *fakeTransport) Call(ctx context.Context, method string, params any, result any) error {
	f.callMethod = method
	f.callParams = params
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

func (f *fakeTransport) Notify(ctx context.Context, method string, params any) error {
	f.notifyMethod = method
	return nil
}

func (f *fakeTransport) SetRequestHandler(handler RequestHandler) {
	f.handler = handler
}

func (f *fakeTransport) Close() error { return nil }

func TestNewWiresApprovalHandler(t *testing.T) {
	ft := &fakeTransport{}
	client, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if ft.handler == nil {
		t.Fatal("expected request handler to be installed")
	}
}

func TestInitializeUsesProtocolMethods(t *testing.T) {
	ft := &fakeTransport{result: InitializeResult{UserAgent: "ua"}}
	client, err := New(WithTransport(ft))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got, err := client.Initialize(context.Background(), InitializeRequest{})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if ft.callMethod != "initialize" {
		t.Fatalf("call method = %q", ft.callMethod)
	}
	if ft.notifyMethod != "initialized" {
		t.Fatalf("notify method = %q", ft.notifyMethod)
	}
	if got.UserAgent != "ua" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestApprovalAdapterRoutesRequests(t *testing.T) {
	ft := &fakeTransport{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/tool/requestUserInput",
		Params: mustJSON(t, UserInputRequest{ThreadID: "t1"}),
	})
	if err != nil {
		t.Fatalf("HandleServerRequest() error = %v", err)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected response payload")
	}
}

type testApprovals struct{}

func (testApprovals) HandleCommandExecutionApproval(context.Context, CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error) {
	return CommandExecutionApprovalResult{Decision: ApprovalDecisionAccept}, nil
}

func (testApprovals) HandleFileChangeApproval(context.Context, FileChangeApprovalRequest) (FileChangeApprovalResult, error) {
	return FileChangeApprovalResult{Decision: FileChangeApprovalDecisionAccept}, nil
}

func (testApprovals) HandlePermissionsApproval(context.Context, PermissionsApprovalRequest) (PermissionsApprovalResult, error) {
	return PermissionsApprovalResult{Permissions: []string{"read"}}, nil
}

func (testApprovals) HandleUserInputRequest(context.Context, UserInputRequest) (UserInputResult, error) {
	return UserInputResult{Answers: map[string]string{"a": "b"}}, nil
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
