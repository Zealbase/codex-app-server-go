# codex-go Conformance Report

Date: 2026-06-22

Scope:
- Public Go package: `sdk/codex-go`
- Internal protocol constants: `sdk/codex-go/internal/protocol/envelope.go`
- Internal transport notifications: `sdk/codex-go/internal/transport/jsonrpc.go`
- Public docs: `sdk/codex-go/docs/*.md`

## Summary

The SDK now has a usable schema-backed request/response client, a public live notification API, and public wait helpers for terminal turn state and final output recovery.

Remaining gaps:
- No replayable event history for late subscribers
- No public wrappers yet for `turn/steer`, `review/start`, or `model/list`
- No WebSocket / HTTP transport yet

## RPC Methods By Group

| Group | Method | Internal Protocol Constant | Public Client Method | Implemented |
| --- | --- | --- | --- | --- |
| Session | `initialize` | `MethodInitialize` | `Initialize` | Yes |
| Session | `initialized` | `MethodInitialized` | sent internally by `Initialize` | Yes |
| Thread | `thread/start` | `MethodThreadStart` | `ThreadStart` | Yes |
| Thread | `thread/resume` | `MethodThreadResume` | `ThreadResume` | Yes |
| Thread | `thread/read` | `MethodThreadRead` | `ThreadRead` | Yes |
| Turn | `turn/start` | `MethodTurnStart` | `TurnStart` | Yes |
| Turn | `turn/steer` | `MethodTurnSteer` | none | No |
| Turn | `turn/interrupt` | `MethodTurnInterrupt` | `TurnInterrupt` | Yes |
| Review | `review/start` | `MethodReviewStart` | none | No |
| Models | `model/list` | `MethodModelList` | internal via `loadModelCatalog` | Partial |

Notes:
- `model/list` is only used for client-side validation support, not exposed as a public method.
- `turn/steer` and `review/start` exist in protocol constants but have no public client wrapper.

## Server Notification Methods By Group

| Group | Method | Internal Constant | Typed Internal Struct Exists | Public Typed API | Public Subscription API | Implemented |
| --- | --- | --- | --- | --- | --- | --- |
| Turn | `turn/started` | `MethodTurnStarted` | Yes | Yes | Yes | Yes |
| Turn | `turn/completed` | `MethodTurnCompleted` | Yes | Yes | Yes | Yes |
| Turn | `turn/diff/updated` | `MethodTurnDiffUpdated` | Yes | Yes | Yes | Yes |
| Turn | `turn/plan/updated` | `MethodTurnPlanUpdated` | Yes | Yes | Yes | Yes |
| Thread | `thread/tokenUsage/updated` | `MethodThreadTokenUsageUpdated` | Yes | Yes | Yes | Yes |
| Item | `item/started` | `MethodItemStarted` | Yes | Yes | Yes | Yes |
| Item | `item/completed` | `MethodItemCompleted` | Yes | Yes | Yes | Yes |
| Item | `item/agentMessage/delta` | `MethodItemAgentMessageDelta` | Yes | Yes | Yes | Yes |
| Item | `item/plan/delta` | `MethodItemPlanDelta` | Yes | Yes | Yes | Yes |
| Item | `item/reasoning/summaryTextDelta` | `MethodItemReasoningSummaryTextDelta` | Yes | Yes | Yes | Yes |
| Item | `item/reasoning/summaryPartAdded` | `MethodItemReasoningSummaryPartAdded` | Yes | Yes | Yes | Yes |
| Item | `item/reasoning/textDelta` | `MethodItemReasoningTextDelta` | Yes | Yes | Yes | Yes |
| Item | `item/commandExecution/outputDelta` | `MethodItemCommandExecutionOutputDelta` | Yes | Yes | Yes | Yes |
| Item | `item/fileChange/patchUpdated` | `MethodItemFileChangePatchUpdated` | Yes | Yes | Yes | Yes |
| Item | `item/fileChange/outputDelta` | `MethodItemFileChangeOutputDelta` | Yes | Yes | Yes | Yes |
| Item | `item/autoApprovalReview/started` | `MethodItemAutoApprovalReviewStarted` | Yes | Yes | Yes | Yes |
| Item | `item/autoApprovalReview/completed` | `MethodItemAutoApprovalReviewCompleted` | Yes | Yes | Yes | Yes |
| Error | `error` | `MethodError` | Yes | Yes | Yes | Yes |

Notes:
- The internal transport already receives all notifications through `Notifications()`.
- Only a subset has typed internal structs today.
- None are currently exposed from the public package in a usable runtime API.

## Public Exposed Modules

| File / Module | Public Role | Status |
| --- | --- | --- |
| `client.go` | core client methods, stdio transport wiring | Implemented |
| `types.go` | request/response type aliases | Implemented |
| `approval.go` | approval and server-request handlers | Implemented |
| `options.go` | options and transport abstraction | Implemented |
| `thread.go` | thread docs placeholder | Minimal |
| `turn.go` | turn docs placeholder | Minimal |
| `event.go` | public event payloads and event envelope | Implemented |
| `events_decode.go` | notification decode registry | Implemented |
| `events_stream.go` | subscription broker | Implemented |
| `wait.go` | terminal turn wait helper | Implemented |
| `output.go` | final text and structured output helpers | Implemented |
| `doc.go` | package docs | Implemented |

## Public API Conformance Table

| Surface | Expected Use | Current State | Status |
| --- | --- | --- | --- |
| Core RPC methods | start/read/interact with app-server | available | Yes |
| Public event subscription | consume runtime notifications as they happen | `Events()` + `EventSubscription` | Yes |
| Typed event decoding | decode server notifications by method into typed Go values | public `Event.Value` typed payloads | Yes |
| Wait for terminal turn | block until completed/failed/interrupted | `WaitForTurn` | Yes |
| Wait for final assistant output | collect final message/result reliably | `WaitForFinalAgentMessage` | Yes |
| Wait for structured output | recover schema-constrained final payload | `WaitForStructuredOutput` | Yes |
| Public model listing | inspect app-server model catalog | absent | No |
| Public steer/review methods | full method wrapper parity with constants | absent | No |

## Docs Conformance Table

| Topic | Current Docs State | Conformant |
| --- | --- | --- |
| Core client methods | mostly accurate | Yes |
| Event support | docs describe the public live event API | Yes |
| Practical completion flow | docs and example use wait helpers and event subscriptions | Yes |
| Public module map | docs match the actual public event/wait modules | Yes |

## Modular Implementation Plan

### Phase 1: Public Event Surface

Add a regeneration-safe public event layer in new non-generated files:
- `sdk/codex-go/events.go`
- `sdk/codex-go/events_decode.go`
- `sdk/codex-go/events_stream.go`

Planned API:
- `type Event interface`
- `type Notification struct`
- `type EventStream struct`
- `func (c *Client) Events() *EventStream`
- `func (s *EventStream) C() <-chan Event`
- `func (s *EventStream) Close()`

Design:
- keep generated schema files untouched
- keep internal transport package unchanged except for minimal adapter hooks if needed
- decode each notification by method into typed public event structs where possible
- fall back to a generic notification event for methods that do not yet have a dedicated struct

### Phase 2: Typed Event Coverage

Expose typed public structs in a new public file:
- `TurnStartedEvent`
- `TurnCompletedEvent`
- `ItemStartedEvent`
- `ItemCompletedEvent`
- `ThreadTokenUsageUpdatedEvent`
- `TurnDiffUpdatedEvent`
- `TurnPlanUpdatedEvent`
- `ErrorEvent`
- delta-style item notification structs for currently untyped methods

Design:
- re-export or mirror the internal structs only where the public shape is stable
- add a method-to-decoder registry for modular extension
- ensure unknown notification methods still flow through as generic events

### Phase 3: Wait Helpers

Add new public helpers in new files:
- `sdk/codex-go/wait.go`
- `sdk/codex-go/output.go`

Planned API:
- `func (c *Client) WaitForTurn(ctx context.Context, threadID, turnID string) (Turn, error)`
- `func (c *Client) WaitForFinalAgentMessage(ctx context.Context, threadID, turnID string) (string, error)`
- `func (c *Client) WaitForStructuredOutput(ctx context.Context, threadID, turnID string, out any) (Turn, error)`

Design:
- prefer event-driven completion when notifications are available
- fall back to `ThreadRead` polling when event transport is unavailable or misses history
- search completed turn items and raw payloads conservatively
- return terminal turn state together with extraction results

### Phase 4: Method Parity Follow-Up

After the above:
- expose `ModelList`
- decide whether to expose `TurnSteer`
- decide whether to expose `ReviewStart`

These are second-order gaps and should not block event/wait support.

## Execution Order

1. Introduce the public event stream and notification adapter.
2. Add typed decode coverage and generic fallback events.
3. Add wait helpers built on top of the event stream with polling fallback.
4. Update examples to use only the public package.
5. Update docs to match the actual public API.
6. Run unit tests and e2e tests.

## Acceptance Criteria

- A caller can subscribe to runtime notifications without importing any `internal` package.
- A caller can wait for a turn to finish without writing their own polling loop.
- A caller can retrieve final assistant output and structured output through public helpers.
- Existing request/response API remains backward compatible.
- Generated schema files remain untouched.
