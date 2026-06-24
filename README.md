# codex-app-server-go

> A typed Go client for the Codex app-server **v2** JSON-RPC protocol — drive AI coding agents from Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/zealbase/codex-app-server-go.svg)](https://pkg.go.dev/github.com/zealbase/codex-app-server-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/zealbase/codex-app-server-go)](https://goreportcard.com/report/github.com/zealbase/codex-app-server-go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](go.mod)
[![Protocol](https://img.shields.io/badge/codex--cli-0.142.0-orange)](internal/protocol/schema/version.go)

## What's Implemented

| Subsystem | Covered | Status |
|---|:---:|---|
| Core / Initialize | 1/1 | ✅ |
| Thread | 20/22 | ✅ start · resume · fork · list · read · archive · unarchive · set-name · rollback · loaded-list · compact · goal-set/get/clear · metadata-update · unsubscribe · delete · inject-items · shell-command · approve-guardian |
| Turn | 3/4 | ✅ start · steer · interrupt |
| Account / Login | 3/4 | ✅ api-key · chatgpt · device-code · get-account · cancel-login |
| Models | 2/2 | ✅ list · provider-capabilities |
| Review | 1/1 | ✅ review/start |
| Config CRUD | 3/3 | ✅ read · value-write · batch-write |
| Skills | 3/3 | ✅ list · config-write · extra-roots-set |
| Experimental features | 2/2 | ✅ list · enablement-set |
| Hooks (server-side) | 1/1 | ✅ list |
| Command exec (PTY) | 4/4 | ✅ exec · write · resize · terminate + streaming |
| Transports | — | ✅ stdio · WebSocket · 🚧 HTTP+SSE (WIP) · retry/reconnect |

Server → client notifications: **65 of 68** typed-decoded.

## Install

```bash
go get github.com/zealbase/codex-app-server-go
```

```go
import codexgo "github.com/zealbase/codex-app-server-go"
```

**Version:** `v0.2.0` · **Protocol:** Codex app-server **v2** · **Go:** 1.25+

## How to Use

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

thread, _ := client.StartThread(ctx, codexgo.WithThreadModel("claude-opus-4-8"))
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
thread, _ := client.StartThread(ctx, codexgo.WithThreadModel("claude-opus-4-8"))
```

### Streaming

```go
ch, _ := thread.RunStreamed(ctx, "Explain this codebase.")
for delta := range ch {
    fmt.Print(delta.Text)
}
```

### Structured output

```go
sub := client.Events(); defer sub.Close()
turn, _ := client.TurnStart(ctx, codexgo.TurnStartRequest{
    ThreadID:     thread.ID(),
    Input:        "Return {\"answer\":\"PONG\",\"n\":42} as JSON.",
    OutputSchema: schemaJSON,
})
var out struct{ Answer string; N int }
_, _ = client.WaitForStructuredOutput(ctx, thread.ID(), turn.ID, &out)
```

## Full Protocol Coverage

Verified against **codex-cli `0.142.0`** · schema sha256 `935c753c…`

| Axis | Covered | Total | % |
|---|---:|---:|---:|
| RPC request methods | 43 | 84 | **51%** |
| Server notifications (typed-decoded) | 65 | 68 | **~96%** |
| Full message surface | ~108 | 152 | **~71%** |

**RPC by group:**

| Group | Covered | Total | % |
|---|---:|---:|---:|
| Thread | 20 | 22 | 91% |
| Turn | 3 | 4 | 75% |
| Account | 3 | 4 | 75% |
| Core + Review | 2 | 2 | 100% |
| Models | 2 | 2 | 100% |
| Config CRUD | 3 | 3 | 100% |
| Skills | 3 | 3 | 100% |
| Experimental | 2 | 2 | 100% |
| Hooks | 1 | 1 | 100% |
| Command exec (PTY) | 4 | 4 | 100% |
| Filesystem | 0 | 10 | 0% |
| Plugins / marketplace | 0 | 14 | 0% |
| Misc | 0 | 7 | 0% |
| MCP lifecycle | 0 | 4 | 0% |
| Remote control | 0 | 2 | 0% |

See [`gen/conformance-report.md`](gen/conformance-report.md) for the full method-by-method breakdown.

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
`internal/protocol/schema/codex_app_server_protocol.v2.schemas.json`.
Upstream protocol source: [`openai/codex`](https://github.com/openai/codex) —
`codex-rs/app-server-protocol/`.

## License

Licensed under the **Apache License, Version 2.0**, consistent with upstream Codex.
See [`LICENSE`](LICENSE) for the full text.
