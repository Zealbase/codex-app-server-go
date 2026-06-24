// simple demonstrates basic usage of the codex-go SDK: finding the codex
// binary, creating a client, starting a thread, and running a prompt.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

func main() {
	bin, err := codexgo.FindBinary()
	if err != nil {
		log.Fatalf("codex binary not found: %v\nSet CODEX_BIN or install codex in PATH", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := codexgo.New(
		codexgo.WithStdioProcess(bin, "app-server"),
		codexgo.WithApprovalHandler(&autoApprover{}),
	)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}
	defer client.Close()

	prompt := "echo the word PONG"
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	thread, err := client.StartThread(ctx, codexgo.WithInitialInput(prompt))
	if err != nil {
		log.Fatalf("start thread: %v", err)
	}
	defer thread.Close()

	fmt.Printf("Thread %s: done\n", thread.ID())
}

type autoApprover struct{}

func (a *autoApprover) HandleCommandExecutionApproval(_ context.Context, _ codexgo.CommandExecutionApprovalRequest) (codexgo.CommandExecutionApprovalResult, error) {
	return codexgo.CommandExecutionApprovalResult{Decision: codexgo.ApprovalDecisionAccept}, nil
}

func (a *autoApprover) HandleFileChangeApproval(_ context.Context, _ codexgo.FileChangeApprovalRequest) (codexgo.FileChangeApprovalResult, error) {
	return codexgo.FileChangeApprovalResult{Decision: codexgo.FileChangeApprovalDecisionAccept}, nil
}

func (a *autoApprover) HandlePermissionsApproval(_ context.Context, _ codexgo.PermissionsApprovalRequest) (codexgo.PermissionsApprovalResult, error) {
	return codexgo.PermissionsApprovalResult{}, nil
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
