package mock_test

import (
	"context"
	"testing"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// autoApprover accepts every server-initiated approval request.
type autoApprover struct{}

func (autoApprover) HandleCommandExecutionApproval(_ context.Context, _ codexgo.CommandExecutionApprovalRequest) (codexgo.CommandExecutionApprovalResult, error) {
	return codexgo.CommandExecutionApprovalResult{Decision: codexgo.ApprovalDecisionAccept}, nil
}

func (autoApprover) HandleFileChangeApproval(_ context.Context, _ codexgo.FileChangeApprovalRequest) (codexgo.FileChangeApprovalResult, error) {
	return codexgo.FileChangeApprovalResult{Decision: codexgo.FileChangeApprovalDecisionAccept}, nil
}

func (autoApprover) HandlePermissionsApproval(_ context.Context, _ codexgo.PermissionsApprovalRequest) (codexgo.PermissionsApprovalResult, error) {
	return codexgo.PermissionsApprovalResult{}, nil
}

func (autoApprover) HandleUserInputRequest(_ context.Context, _ codexgo.UserInputRequest) (codexgo.UserInputResult, error) {
	return codexgo.UserInputResult{}, nil
}

func TestWithApprovalHandler_AutoAccept(t *testing.T) {
	h := NewHarness(t, codexgo.WithApprovalHandler(autoApprover{}))
	ctx, cancel := testCtx(t)
	defer cancel()

	h.Responses.EnqueueSSE(sseAssistantMessage("done", "r1"))

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	if _, err := st.Run(ctx, "do something"); err != nil {
		t.Fatalf("Run with approval handler: %v", err)
	}
}

func TestNilHandler_TurnContinues(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := testCtx(t)
	defer cancel()

	h.Responses.EnqueueSSE(sseAssistantMessage("done", "r1"))

	st, err := h.Client.StartThread(ctx)
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer st.Close()

	if _, err := st.Run(ctx, "do something"); err != nil {
		t.Fatalf("Run without approval handler: %v", err)
	}
}
