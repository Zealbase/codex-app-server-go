# codex-app-server-go Protocol Coverage

**Report date:** 2026-06-23
**Scope:** Coverage of the Codex app-server JSON-RPC v2 protocol by the `codexgo` SDK,
cross-referenced against the official OpenAI Python SDK.

---

## 1. Summary

The Codex app-server protocol (v2 schema) defines **84 RPC request methods** (client → server
calls) and **68 server-push notifications**. The `codexgo` SDK implements the
thread/turn/account/goal/login **agent-driving core** and intentionally omits the
"codex-as-IDE-backend" subsystems (filesystem, interactive PTY exec, plugin/skill/MCP
management).

| Axis | Covered | Total | % |
|---|---:|---:|---:|
| **RPC request methods** (callable app-server methods) | 43 | 84 | **51%** |
| **Notifications** (server → client, typed-decoded) | 65 | 68 | **~96%** |
| **Full message surface** (requests + notifications) | ~108 | 152 | **~71%** |

### Key findings

1. **51% RPC coverage.** The 41 still-uncovered RPC methods belong to editor-backend
   subsystems (filesystem, plugins/marketplace, MCP lifecycle, remote control, misc). The
   agent-conversation core, Config/Skills/Experimental/Hooks/CommandExec, and
   Thread/Turn/Models extras are fully implemented.

2. **Go exceeds the official Python SDK's callable surface.** Python's `api.py`/`client.py`
   implements only the thread/turn/account/goal/login set; Config CRUD, Skills, Experimental
   features, Hooks/list, and interactive CommandExec exist in Python **only as generated
   wire-types** with no callable methods. The Go SDK drives all five as real RPCs.

3. **On the implemented surface, Go is at parity-or-ahead of Python.** Go additionally ships:
   thread rollback, review mode, structured-output collector, HTTP/WS transports,
   transport retry/reconnect, and a Unix-socket hook bridge.

4. **Conversation use cases are functionally complete.** Start/resume/fork threads, run/steer/
   interrupt turns, stream events, handle approvals, drive goals, log in, roll back, and
   extract structured output are all supported.

---

## 2. Detailed protocol table — RPC methods (84)

Legend: ✅ implemented · ⬜ not implemented · 🔒 server→client request the SDK *answers* (not a client call).

### Covered subsystems

| Subsystem | Covered | Methods |
|---|---:|---|
| **Core** | 1/1 | ✅ Initialize |
| **Thread** | 20/22 | ✅ ThreadStart, ThreadResume, ThreadFork, ThreadList, ThreadRead, ThreadArchive, ThreadUnarchive, ThreadSetName, ThreadRollback, ThreadLoadedList, ThreadCompactStart, ThreadGoalSet, ThreadGoalGet, ThreadGoalClear, ThreadMetadataUpdate, ThreadUnsubscribe, ThreadDelete, ThreadInjectItems, ThreadShellCommand, ThreadApproveGuardianDeniedAction · ⬜ ThreadMetadataGitInfoUpdate, ThreadResumeInitialTurnsPage *(nested sub-types, not callable RPCs)* |
| **Turn** | 3/4 | ✅ TurnStart, TurnSteer, TurnInterrupt · ⬜ TurnEnvironment *(not a registered RPC in 0.142.0)* |
| **Account** | 3/4 | ✅ LoginAccount, CancelLoginAccount, GetAccount · ⬜ ConsumeAccountRateLimitResetCredit |
| **Models** | 2/2 | ✅ ModelList, ModelProviderCapabilitiesRead |
| **Review** | 1/1 | ✅ ReviewStart |
| **Config CRUD** | 3/3 | ✅ ConfigRead, ConfigValueWrite, ConfigBatchWrite |
| **Skills** | 3/3 | ✅ SkillsList, SkillsConfigWrite, SkillsExtraRootsSet |
| **Experimental** | 2/2 | ✅ ExperimentalFeatureList, ExperimentalFeatureEnablementSet |
| **Hooks (server-side)** | 1/1 | ✅ HooksList |
| **Command exec (PTY)** | 4/4 | ✅ CommandExec, CommandExecWrite, CommandExecResize, CommandExecTerminate |

### Uncovered subsystems

| Subsystem | Total | Methods |
|---|---:|---|
| **Filesystem** | 10 | ⬜ FsReadFile, FsWriteFile, FsReadDirectory, FsCreateDirectory, FsCopy, FsRemove, FsGetMetadata, FsWatch, FsUnwatch, FuzzyFileSearch |
| **Plugins / marketplace** | 14 | ⬜ PluginInstall, PluginInstalled, PluginList, PluginRead, PluginUninstall, PluginShareCheckout, PluginShareDelete, PluginShareList, PluginShareSave, PluginShareUpdateTargets, PluginSkillRead, MarketplaceAdd, MarketplaceRemove, MarketplaceUpgrade |
| **Misc** | 7 | ⬜ AppsList, FeedbackUpload, ExternalAgentConfigDetect, ExternalAgentConfigImport, PermissionProfileList, SendAddCreditsNudgeEmail, WindowsSandboxSetupStart |
| **MCP server lifecycle** | 4 | ⬜ ListMcpServerStatus, McpServerToolCall, McpResourceRead, McpServerOauthLogin |
| **Remote control** | 2 | ⬜ RemoteControlEnable, RemoteControlDisable |

### Incoming server-requests the SDK answers (🔒)

Handled by the approval `Dispatcher`, not counted in the 84:

- 🔒 `item/commandExecution/requestApproval`
- 🔒 `item/fileChange/requestApproval`
- 🔒 `item/mcp/requestApproval`
- 🔒 `item/permissions/requestApproval`
- 🔒 `item/tool/call` (dynamic tool call)
- 🔒 `item/tool/requestUserInput`

---

## 3. Notifications (68) — coverage overview

The SDK typed-decodes **65 of 68** notifications; only 3 schema-only (non-wire-registered)
notifications fall through to `RawNotificationEvent`. Original 27 are wired in `events.go` /
`events_extra.go`; the 38 added in phase 3 live in `events_extra.go`.

**Phase-3 additions (events_extra.go):**
- *Thread/account:* `thread/deleted`, `thread/name/updated`, `thread/compacted`,
  `thread/settings/updated`, `account/updated`, `account/rateLimits/updated`
- *Warnings/model:* `warning`, `configWarning`, `deprecationNotice`, `guardianWarning`,
  `windows/worldWritableWarning`, `windowsSandbox/setupCompleted`, `model/rerouted`,
  `model/verification`, `turn/moderationMetadata`
- *Process/exec/hooks:* `process/exited`, `process/outputDelta`,
  `item/commandExecution/terminalInteraction`, `item/mcpToolCall/progress`, `hook/started`,
  `hook/completed`, `rawResponseItem/completed`
- *Subsystem status:* `mcpServer/startupStatus/updated`, `mcpServer/oauthLogin/completed`,
  `skills/changed`, `fs/changed`, `app/list/updated`, `remoteControl/status/changed`,
  `externalAgentConfig/import/completed`, `externalAgentConfig/import/progress`
- *Realtime audio:* 8× `thread/realtime/*` (started, closed, error, itemAdded, sdp,
  outputAudio/delta, transcript/delta, transcript/done)

---

## 4. Version pins

| Component | Version / ref |
|---|---|
| **Go module** | `github.com/zealbase/codex-app-server-go` |
| **SDK version** | `v0.2.0` |
| **Go toolchain** | `go 1.25.0` |
| **WebSocket dep** | `nhooyr.io/websocket v1.8.17` |
| **JSON-RPC dep** | `github.com/creachadair/jrpc2 v1.3.5` |
| **Protocol schema** | `codex_app_server_protocol.v2.schemas.json` (title `CodexAppServerProtocolV2`) |
| **Schema sha256** | `935c753c973a7cba99ba9c7b280218848a61a695b56a3d7512bc4db0fd4027bd` |
| **Schema definitions** | 496 total · 84 `*Params` · 68 `*Notification` · 4 `*Result` |
| **codex binary** | `codex-cli 0.142.0` |

---

## 5. Methodology

- **84 RPC methods**: count of `*Params` definitions in the v2 schema.
- **68 notifications**: count of `*Notification` definitions in the same schema.
- **43 covered RPC methods**: intersection of schema `*Params` names with wire methods the Go
  SDK calls via `Method*` constants in `internal/protocol/*.go`.
- Coverage grew: 23 → 36 (phase 1: Config 3, Skills 3, Experimental 2, Hooks 1, CommandExec 4)
  → 43 (phase 2: Thread extras 6, ModelProviderCapabilitiesRead 1).

---

## 6. Related

- [`docs/api-reference.md`](../docs/api-reference.md) — full implemented API surface
- [`llms-full.txt`](../llms-full.txt) — machine-readable reference
