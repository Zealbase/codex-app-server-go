package codexgo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

// errorApprovals is an ApprovalHandler that always returns an error.
type errorApprovals struct{}

func (errorApprovals) HandleCommandExecutionApproval(_ context.Context, _ CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error) {
	return CommandExecutionApprovalResult{}, errors.New("approval denied")
}

func (errorApprovals) HandleFileChangeApproval(_ context.Context, _ FileChangeApprovalRequest) (FileChangeApprovalResult, error) {
	return FileChangeApprovalResult{}, errors.New("approval denied")
}

func (errorApprovals) HandlePermissionsApproval(_ context.Context, _ PermissionsApprovalRequest) (PermissionsApprovalResult, error) {
	return PermissionsApprovalResult{}, errors.New("approval denied")
}

func (errorApprovals) HandleUserInputRequest(_ context.Context, _ UserInputRequest) (UserInputResult, error) {
	return UserInputResult{}, errors.New("approval denied")
}

// TestApprovalAdapterCommandExecution verifies routing of the command execution
// approval method through the adapter.
func TestApprovalAdapterCommandExecution(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/commandExecution/requestApproval",
		Params: mustJSON(t, CommandExecutionApprovalRequest{Command: "rm -rf /", ThreadID: "t1"}),
	})
	if err != nil {
		t.Fatalf("HandleServerRequest() error = %v", err)
	}
	var result CommandExecutionApprovalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if string(result.Decision) != ApprovalDecisionAccept {
		t.Fatalf("unexpected decision: %s", result.Decision)
	}
}

// TestApprovalAdapterFileChange verifies routing of the file change approval
// method through the adapter.
func TestApprovalAdapterFileChange(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/fileChange/requestApproval",
		Params: mustJSON(t, FileChangeApprovalRequest{ThreadID: "t2", GrantRoot: "/home/user"}),
	})
	if err != nil {
		t.Fatalf("HandleServerRequest() error = %v", err)
	}
	var result FileChangeApprovalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if string(result.Decision) != FileChangeApprovalDecisionAccept {
		t.Fatalf("unexpected decision: %s", result.Decision)
	}
}

// TestApprovalAdapterPermissions verifies routing of the permissions approval
// method through the adapter.
func TestApprovalAdapterPermissions(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/permissions/requestApproval",
		Params: mustJSON(t, PermissionsApprovalRequest{ThreadID: "t3", Permissions: []string{"read", "write"}}),
	})
	if err != nil {
		t.Fatalf("HandleServerRequest() error = %v", err)
	}
	var result PermissionsApprovalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Permissions) == 0 || result.Permissions[0] != "read" {
		t.Fatalf("unexpected permissions: %v", result.Permissions)
	}
}

// TestApprovalAdapterUnknownMethod verifies that an unsupported server request
// method returns ErrUnsupportedServerRequest.
func TestApprovalAdapterUnknownMethod(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/unknown/method",
		Params: mustJSON(t, map[string]any{}),
	})
	if !errors.Is(err, protocol.ErrUnsupportedServerRequest) {
		t.Fatalf("expected ErrUnsupportedServerRequest, got %v", err)
	}
}

// TestApprovalAdapterHandlerError verifies that errors returned by the handler
// are propagated back to the caller.
func TestApprovalAdapterHandlerError(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(errorApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/commandExecution/requestApproval",
		Params: mustJSON(t, CommandExecutionApprovalRequest{Command: "ls"}),
	})
	if err == nil {
		t.Fatal("expected error from handler, got nil")
	}
	if err.Error() != "approval denied" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestApprovalAdapterMalformedParams verifies that malformed JSON params
// cause a decode error before the handler is called.
func TestApprovalAdapterMalformedParams(t *testing.T) {
	ft := &fakeTransportWithErrors{}
	_, err := New(WithTransport(ft), WithApprovalHandler(testApprovals{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = ft.handler.HandleServerRequest(context.Background(), ServerRequest{
		Method: "item/commandExecution/requestApproval",
		Params: json.RawMessage(`{invalid json`),
	})
	if err == nil {
		t.Fatal("expected error from malformed params, got nil")
	}
}
