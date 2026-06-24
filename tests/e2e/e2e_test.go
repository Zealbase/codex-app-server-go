package e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

func skipIfNoBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("CODEX_BIN")
	if bin == "" {
		t.Skip("CODEX_BIN not set; skipping E2E tests")
	}
	return bin
}

type autoApprover struct{}

func (a *autoApprover) HandleCommandExecutionApproval(_ context.Context, _ codexgo.CommandExecutionApprovalRequest) (codexgo.CommandExecutionApprovalResult, error) {
	return codexgo.CommandExecutionApprovalResult{Decision: codexgo.ApprovalDecisionAccept}, nil
}

func (a *autoApprover) HandleFileChangeApproval(_ context.Context, _ codexgo.FileChangeApprovalRequest) (codexgo.FileChangeApprovalResult, error) {
	return codexgo.FileChangeApprovalResult{Decision: codexgo.FileChangeApprovalDecisionAccept}, nil
}

func (a *autoApprover) HandlePermissionsApproval(_ context.Context, req codexgo.PermissionsApprovalRequest) (codexgo.PermissionsApprovalResult, error) {
	return codexgo.PermissionsApprovalResult{Permissions: req.Permissions, Scope: req.Scope}, nil
}

func (a *autoApprover) HandleUserInputRequest(_ context.Context, req codexgo.UserInputRequest) (codexgo.UserInputResult, error) {
	answers := make(map[string]string, len(req.Questions))
	for _, q := range req.Questions {
		if len(q.Options) > 0 {
			answers[q.ID] = q.Options[0].Label
		} else {
			answers[q.ID] = "yes"
		}
	}
	return codexgo.UserInputResult{Answers: answers}, nil
}

func TestE2E_ThreadLifecycle(t *testing.T) {
	bin := skipIfNoBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := codexgo.New(
		codexgo.WithStdioProcess(bin, "app-server"),
		codexgo.WithApprovalHandler(&autoApprover{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	thread, err := client.StartThread(ctx, codexgo.WithThreadModel("gpt-4o-mini"))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	result, err := thread.Run(ctx, "echo the word PONG")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("turn error: %v", result.Error)
	}
	t.Logf("Turn completed, items=%d", len(result.Items))
}

func TestE2E_ConformanceLoop(t *testing.T) {
	bin := skipIfNoBin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := codexgo.New(
		codexgo.WithStdioProcess(bin, "app-server"),
		codexgo.WithApprovalHandler(&autoApprover{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	thread, err := client.StartThread(ctx, codexgo.WithThreadModel("gpt-4o-mini"))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	prompts := []string{
		"list files in the current directory",
		"summarize what you found",
	}

	for round, prompt := range prompts {
		eventCh, err := thread.RunStreamed(ctx, prompt)
		if err != nil {
			t.Fatalf("round %d: RunStreamed: %v", round, err)
		}

		var sawError bool
		for event := range eventCh {
			if event.Kind == "error" {
				sawError = true
				t.Errorf("round %d: ERROR event: %v", round, event.Raw)
			}
		}
		if sawError {
			t.Fatalf("round %d: turn emitted ERROR events", round)
		}
		t.Logf("round %d complete", round)
	}
}
