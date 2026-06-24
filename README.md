# codex-go

> Start using the Codex app-server in Go — a typed client that implements the official Codex app-server **v2** JSON-RPC spec.

`github.com/nharness/sdk/codex-go` is a Go client SDK for the [Codex](https://github.com/openai/codex)
app-server. It speaks the app-server's JSON-RPC 2.0 protocol over stdio, WebSocket, or HTTP+SSE,
and exposes both a high-level `SessionThread` API and the raw RPC surface.

- **Version:** `v0.2.0` · **Protocol:** Codex app-server **v2** · **Go:** 1.25+
- **Verified against:** codex-cli `0.142.0`

## Install

```bash
go get github.com/nharness/sdk/codex-go
```

```go
import codexgo "github.com/nharness/sdk/codex-go"
```

## Quickstart

### Local binary (stdio)

```go
cmd := exec.Command("codex", "app-server", "--stdio")
stdout, _ := cmd.StdoutPipe()
stdin, _ := cmd.StdinPipe()
_ = cmd.Start()

client, _ := codexgo.New(codexgo.WithStdioTransport(stdout, stdin))
defer client.Close()

ctx := context.Background()
_, _ = client.Initialize(ctx, codexgo.InitializeRequest{
    ClientInfo:   codexgo.ClientInfo{Name: "my-tool", Version: "0.1.0"},
    Capabilities: codexgo.Capabilities{ExperimentalAPI: true},
})

thread, _ := client.StartThread(ctx, codexgo.WithThreadModel("gpt-5.4"))
defer thread.Close()

result, _ := thread.Run(ctx, "Summarize this repo in one sentence.")
fmt.Println(result.FinalAgentText())
```

### Remote server (WebSocket)

```go
dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

client, _ := codexgo.New(
    codexgo.WithWSBearerToken("my-api-key"),
    codexgo.WithWSTransport(dialCtx, "ws://codex-server.example.com"),
)
defer client.Close()
// WS transport auto-initializes; go straight to StartThread.
thread, _ := client.StartThread(ctx, codexgo.WithThreadModel("gpt-5.4"))
```

## Examples

| Topic | What it shows |
|---|---|
| `SessionThread.Run` | synchronous turn + `FinalAgentText()` / `Usage` |
| `SessionThread.RunStreamed` | streaming deltas over a channel |
| `WaitForStructuredOutput` | JSON-schema-constrained output into a Go struct |
| Approvals | `WithApprovalHandler` / `AutoAcceptApprovalHandler` |
| Transports | stdio, WebSocket, HTTP+SSE (WIP), retry/reconnect |
| Interactive exec | `CommandExec` + `CommandExecHandle` (write/resize/terminate) |

A complete multi-agent example lives at
`examples/sdk/codex-go-usage/existing-app-server-multi-agent-search`.

Structured-output sketch:

```go
sub := client.Events(); defer sub.Close()   // subscribe before the turn
turn, _ := client.TurnStart(ctx, codexgo.TurnStartRequest{
    ThreadID:     thread.ID(),
    Input:        "Return {\"answer\":\"PONG\",\"n\":42} as JSON.",
    OutputSchema: schemaJSON,
})
var out struct{ Answer string; N int }
_, _ = client.WaitForStructuredOutput(ctx, thread.ID(), turn.ID, &out)
```

## Protocol coverage

The SDK implements **43 / 84** callable app-server RPC methods (**~51%**) and typed-decodes
**65 / 68** server notifications (**~96%**), as of codex-cli `0.142.0`.

| Subsystem | Coverage |
|---|---|
| Core / Thread / Turn | Initialize · Thread 20/20 callable · Turn 3/3 callable |
| Account / login | login (api-key / chatgpt / device-code), read, logout |
| Goals | set / get / clear |
| Models | list · provider capabilities |
| Config CRUD | read · value-write · batch-write |
| Skills | list · config-write · extra-roots-set |
| Experimental features | list · enablement-set |
| Hooks (server-side) | list |
| Command exec (PTY) | exec · write · resize · terminate (+ streaming) |
| Transports | stdio · WebSocket · HTTP+SSE (WIP) · retry/reconnect |

**Not yet covered:** filesystem ops, plugins/marketplace, MCP server lifecycle, remote control,
and misc editor-backend RPCs. Full breakdown:
[`gen/codex-go-protocol-coverage.md`](../../gen/codex-go-protocol-coverage.md).

## Documentation

- [`docs/index.md`](docs/index.md) — guide, transports, SessionThread, wait helpers
- [`docs/api-reference.md`](docs/api-reference.md) — full type & method reference
- [`llms.txt`](llms.txt) / [`llms-full.txt`](llms-full.txt) — machine-readable references

## Protocol reference

This client targets the **Codex app-server v2** JSON-RPC schema. The canonical schema is
produced by the Codex binary itself:

```bash
codex app-server generate-json-schema --out <dir>
```

A pinned copy ships at
`internal/protocol/schema/codex_app_server_protocol.v2.schemas.json` (title
`CodexAppServerProtocolV2`). Upstream protocol source:
[`openai/codex`](https://github.com/openai/codex) — `codex-rs/app-server-protocol/`.
When targeting a different codex-cli build, regenerate types with `make generate` and
re-run the coverage extraction, since method availability can shift between versions
(use a **similar spec version** to the codex binary you connect to).

## License

Licensed under the **Apache License, Version 2.0**, consistent with upstream Codex.
See the repository `LICENSE` for the full text. Unless required by applicable law or agreed
to in writing, software distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND.
