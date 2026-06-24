# codex-go

Go client SDK for the Codex app-server JSON-RPC protocol.

Version: `v0.2.0` | Protocol schema: `v2` | Go `1.25+`

## Installation

```bash
go get github.com/zealbase/codex-app-server-go
```

## Quickstart

### Stdio transport (local binary)

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	codexgo "github.com/zealbase/codex-app-server-go"
)

func main() {
	cmd := exec.Command("codex", "app-server", "--stdio")
	stdout, _ := cmd.StdoutPipe()
	stdin, _ := cmd.StdinPipe()
	cmd.Stderr = os.Stderr
	_ = cmd.Start()

	client, _ := codexgo.New(codexgo.WithStdioTransport(stdout, stdin))
	defer client.Close()

	ctx := context.Background()
	_, _ = client.Initialize(ctx, codexgo.InitializeRequest{
		ClientInfo:   codexgo.ClientInfo{Name: "my-app", Version: "1.0.0"},
		Capabilities: codexgo.Capabilities{ExperimentalAPI: true},
	})

	thread, _ := client.StartThread(ctx, codexgo.WithThreadModel("gpt-5.4"))
	defer thread.Close()

	result, _ := thread.Run(ctx, "Say hello in one sentence.")
	fmt.Println(result.FinalAgentText())
}
```

### WebSocket transport (remote server)

```go
dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

client, _ := codexgo.New(
    codexgo.WithWSBearerToken("my-api-key"),
    codexgo.WithWSTransport(dialCtx, "http://codex-server.example.com"),
)
defer client.Close()

_, _ = client.Initialize(ctx, codexgo.InitializeRequest{
    ClientInfo:   codexgo.ClientInfo{Name: "my-app", Version: "1.0.0"},
    Capabilities: codexgo.Capabilities{ExperimentalAPI: true},
})
```

## SessionThread

`StartThread` / `ResumeThread` return a `*SessionThread` — a stateful handle that serialises concurrent turn access.

```go
// Start a new thread (blocks until semaphore slot is available if WithMaxThreads is set).
thread, err := client.StartThread(ctx,
    codexgo.WithThreadModel("gpt-5.4"),
    codexgo.WithThreadApprovalMode(codexgo.ApprovalModeNever),
    codexgo.WithInitialInput("You are a Go assistant."),
)
if err != nil {
    log.Fatal(err)
}
defer thread.Close() // release semaphore slot

// Run a synchronous turn.
result, err := thread.Run(ctx, "Explain interfaces in Go.")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.FinalAgentText())
fmt.Printf("input tokens: %d\n", result.Usage.InputTokens)

// Resume an existing thread by ID.
resumed, err := client.ResumeThread(ctx, thread.ID())
defer resumed.Close()
```

## TurnResult

`Run` returns a `*TurnResult`:

```go
type TurnResult struct {
    Turn      Turn        // final turn state from the server
    Items     []Item      // items captured via streaming events
    Usage     *TokenUsage // token usage; may be nil if server does not report it
    Error     *TurnError  // non-nil when turn status is "failed"
    DeltaText string      // reconstructed text from deltas when Items are absent
}

func (r *TurnResult) FinalAgentText() string
```

`FinalAgentText` searches `Items`, `Turn.Items`, and `DeltaText` for the last agent-message text.

## Streaming turns

```go
events, err := thread.RunStreamed(ctx, "Count to five.")
for ev := range events {
    if delta, ok := ev.Raw.(codexgo.ItemAgentMessageDeltaEvent); ok {
        fmt.Print(delta.Text)
    }
}
```

## Event Subscription

`Events()` returns a live notification subscription. Events are not replayed to late subscribers; subscribe before starting a turn to avoid missing events.

```go
sub := client.Events()
defer sub.Close()

for {
    select {
    case event, ok := <-sub.C():
        if !ok {
            return // broker closed
        }
        switch v := event.Value.(type) {
        case codexgo.TurnCompletedEvent:
            fmt.Println("turn completed:", v.TurnID, v.Status)
        case codexgo.ItemAgentMessageDeltaEvent:
            fmt.Print(v.Text)
        }
    case <-ctx.Done():
        return
    }
}
```

## Wait Helpers

```go
// Block until a turn reaches a terminal state.
turn, err := client.WaitForTurn(ctx, threadID, turnID)

// Wait for a turn and extract final assistant text.
text, err := client.WaitForFinalAgentMessage(ctx, threadID, turnID)

// Wait for a turn and unmarshal structured output.
var out MyStruct
turn, err := client.WaitForStructuredOutput(ctx, threadID, turnID, &out)
```

## Approval Handling

```go
type ApprovalHandler interface {
    HandleCommandExecutionApproval(context.Context, CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error)
    HandleFileChangeApproval(context.Context, FileChangeApprovalRequest) (FileChangeApprovalResult, error)
    HandlePermissionsApproval(context.Context, PermissionsApprovalRequest) (PermissionsApprovalResult, error)
    HandleUserInputRequest(context.Context, UserInputRequest) (UserInputResult, error)
}

client, _ := codexgo.New(
    codexgo.WithStdioTransport(stdout, stdin),
    codexgo.WithApprovalHandler(myHandler),
)
```

## Thread Management

```go
// Archive / unarchive
_ = thread.Archive(ctx)
_ = thread.Unarchive(ctx)

// Rename
_ = thread.SetName(ctx, "my-project")

// Fork from current state
forked, _ := thread.Fork(ctx, "" /* latest turn */)
defer forked.Close()

// Rollback specific turns
_ = thread.Rollback(ctx, []string{turnID})

// Goal tracking
_ = thread.SetGoal(ctx, "Implement a REST API in Go")
goal, _ := thread.GetGoal(ctx)
_ = thread.ClearGoal(ctx)

// Context compaction
_ = thread.Compact(ctx)

// List loaded threads
ids, _ := client.ThreadLoadedList(ctx)

// Read a specific turn
turn, _ := client.TurnRead(ctx, threadID, turnID)

// Get a git diff for the latest turn
diff, _ := thread.GitDiff(ctx, "" /* latest */)
```

## Concurrency

Use `WithMaxThreads(n)` to limit concurrent `SessionThread` instances. `StartThread` / `ResumeThread` block until a slot is available (or ctx is cancelled). Call `thread.Close()` to release the slot.

```go
client, _ := codexgo.New(
    codexgo.WithWSTransport(dialCtx, endpoint),
    codexgo.WithMaxThreads(4),
)
```

## Transports

| Option | Description |
|---|---|
| `WithStdioTransport(r, w)` | Reads from r, writes to w (local binary) |
| `WithWSTransport(dialCtx, endpoint)` | WebSocket (recommended for remote servers) |
| `WithHTTPTransport(endpoint)` | **WIP** — HTTP+SSE; requires the ws-http-bridge sidecar |
| `WithReconnectingHTTPTransport(endpoint)` | **WIP** — HTTP with auto-reconnect (same bridge requirement) |
| `WithRetry(cfg)` | Wrap existing transport with retry logic |
| `WithHTTPBearerToken(token)` | Auth header for HTTP transports |
| `WithWSBearerToken(token)` | Auth header for WebSocket transport |

> **HTTP transport is work-in-progress.** The Codex app-server speaks WebSocket only.
> The HTTP+SSE transports need the `ws-http-bridge` sidecar
> (`plugins/codex-server/ws-http-bridge`) in front of the server; without it they
> fail with HTTP 405. Use `WithWSTransport` for remote servers today.

## More

- [API Reference](api-reference.md)
- [llms.txt](../llms.txt)
