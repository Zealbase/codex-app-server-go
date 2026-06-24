# API Reference

Package: `github.com/zealbase/codex-app-server-go`

## Client

```go
func New(opts ...Option) (*Client, error)
func (c *Client) Close() error
func (c *Client) Initialize(ctx context.Context, req InitializeRequest) (InitializeResult, error)
func (c *Client) Ping(ctx context.Context) error

// Thread lifecycle (low-level)
func (c *Client) ThreadStart(ctx context.Context, req ThreadStartRequest) (Thread, error)
func (c *Client) ThreadResume(ctx context.Context, req ThreadResumeRequest) (Thread, error)
func (c *Client) ThreadRead(ctx context.Context, req ThreadReadRequest) (Thread, error)
func (c *Client) ThreadFork(ctx context.Context, req ThreadForkRequest) (Thread, error)
func (c *Client) ThreadList(ctx context.Context, req ThreadListRequest) ([]Thread, error)
func (c *Client) ThreadLoadedList(ctx context.Context) ([]string, error)
func (c *Client) ThreadArchive(ctx context.Context, req ThreadArchiveRequest) error
func (c *Client) ThreadUnarchive(ctx context.Context, req ThreadUnarchiveRequest) error
func (c *Client) ThreadSetName(ctx context.Context, req ThreadSetNameRequest) error
func (c *Client) ThreadRollback(ctx context.Context, req ThreadRollbackRequest) error

// Turn lifecycle (low-level)
func (c *Client) TurnStart(ctx context.Context, req TurnStartRequest) (Turn, error)
func (c *Client) TurnInterrupt(ctx context.Context, req TurnInterruptRequest) error
func (c *Client) TurnSteer(ctx context.Context, req TurnSteerRequest) error
func (c *Client) TurnDiff(ctx context.Context, req TurnDiffRequest) (TurnDiffResult, error)
func (c *Client) TurnRead(ctx context.Context, threadID, turnID string) (Turn, error)

// Thread goals
func (c *Client) ThreadGoalSet(ctx context.Context, req ThreadGoalSetRequest) (ThreadGoal, error)
func (c *Client) ThreadGoalGet(ctx context.Context, req ThreadGoalGetRequest) (ThreadGoal, error)
func (c *Client) ThreadGoalClear(ctx context.Context, req ThreadGoalClearRequest) error

// Config (slash-command equivalents)
func (c *Client) SetModel(ctx context.Context, model string) error
func (c *Client) SetApprovalPolicy(ctx context.Context, policy string) error
func (c *Client) SetSandboxPolicy(ctx context.Context, policy string) error

// Config CRUD
func (c *Client) ConfigRead(ctx context.Context, req ConfigReadRequest) (ConfigReadResult, error)
func (c *Client) ConfigValueWrite(ctx context.Context, req ConfigValueWriteRequest) error
func (c *Client) ConfigBatchWrite(ctx context.Context, req ConfigBatchWriteRequest) error

// Skills
func (c *Client) SkillsList(ctx context.Context, req SkillsListRequest) (SkillsListResult, error)
func (c *Client) SkillsConfigWrite(ctx context.Context, req SkillsConfigWriteRequest) (SkillsConfigWriteResult, error)
func (c *Client) SkillsExtraRootsSet(ctx context.Context, req SkillsExtraRootsSetRequest) error

// Experimental features
func (c *Client) ExperimentalFeatureList(ctx context.Context, req ExperimentalFeatureListRequest) (ExperimentalFeatureListResult, error)
func (c *Client) ExperimentalFeatureEnablementSet(ctx context.Context, req ExperimentalFeatureEnablementSetRequest) (ExperimentalFeatureEnablementSetResult, error)

// Hooks (server-side; distinct from the client-side hook bridge)
func (c *Client) HooksList(ctx context.Context, req HooksListRequest) (HooksListResult, error)

// Thread extras (metadata / subscription / mutations)
func (c *Client) ThreadMetadataUpdate(ctx context.Context, req ThreadMetadataUpdateRequest) (ThreadMetadataUpdateResult, error)
func (c *Client) ThreadUnsubscribe(ctx context.Context, req ThreadUnsubscribeRequest) (ThreadUnsubscribeResult, error)
func (c *Client) ThreadDelete(ctx context.Context, req ThreadDeleteRequest) error
func (c *Client) ThreadInjectItems(ctx context.Context, req ThreadInjectItemsRequest) error
func (c *Client) ThreadShellCommand(ctx context.Context, req ThreadShellCommandRequest) error
func (c *Client) ThreadApproveGuardianDeniedAction(ctx context.Context, req ThreadApproveGuardianDeniedActionRequest) error

// Model provider capabilities
func (c *Client) ModelProviderCapabilitiesRead(ctx context.Context) (ModelProviderCapabilities, error)

// Interactive command execution (PTY)
func (c *Client) CommandExec(ctx context.Context, req CommandExecRequest) (CommandExecResult, error)
func (c *Client) CommandExecWrite(ctx context.Context, req CommandExecWriteRequest) error
func (c *Client) CommandExecResize(ctx context.Context, req CommandExecResizeRequest) error
func (c *Client) CommandExecTerminate(ctx context.Context, req CommandExecTerminateRequest) error
func (c *Client) CommandExecHandle(processID string) *CommandExecHandle // write/resize/terminate convenience

// Models
func (c *Client) Models(ctx context.Context) ([]string, error)

// High-level session API
func (c *Client) StartThread(ctx context.Context, opts ...ThreadOption) (*SessionThread, error)
func (c *Client) ResumeThread(ctx context.Context, threadID string, opts ...ThreadOption) (*SessionThread, error)

// Events
func (c *Client) Events() *EventSubscription

// Wait helpers
func (c *Client) WaitForTurn(ctx context.Context, threadID, turnID string) (Turn, error)
func (c *Client) WaitForFinalAgentMessage(ctx context.Context, threadID, turnID string) (string, error)
func (c *Client) WaitForStructuredOutput(ctx context.Context, threadID, turnID string, out any) (Turn, error)
```

## SessionThread

`SessionThread` is the recommended way to manage a thread. It serialises concurrent turn access via an internal mutex and optionally limits total concurrent threads via a semaphore.

```go
func (t *SessionThread) ID() string
func (t *SessionThread) Close()

// Synchronous turn — blocks until terminal status.
func (t *SessionThread) Run(ctx context.Context, input string, opts ...TurnOption) (*TurnResult, error)

// Streaming turn — returns channel of events; drain or cancel ctx to release mutex.
func (t *SessionThread) RunStreamed(ctx context.Context, input string, opts ...TurnOption) (<-chan ThreadEvent, error)

// Mid-turn input
func (t *SessionThread) Steer(ctx context.Context, turnID, input string) error
func (t *SessionThread) SteerInputs(ctx context.Context, turnID string, inputs ...TurnInput) error
func (t *SessionThread) Interrupt(ctx context.Context, turnID string) error

// Thread management
func (t *SessionThread) Archive(ctx context.Context) error
func (t *SessionThread) Unarchive(ctx context.Context) error
func (t *SessionThread) SetName(ctx context.Context, name string) error
func (t *SessionThread) Fork(ctx context.Context, turnID string, opts ...ThreadOption) (*SessionThread, error)
func (t *SessionThread) Rollback(ctx context.Context, turnIDs []string) error
func (t *SessionThread) Compact(ctx context.Context) error
func (t *SessionThread) GitDiff(ctx context.Context, turnID string) (string, error)

// Goals
func (t *SessionThread) SetGoal(ctx context.Context, goal string) error
func (t *SessionThread) GetGoal(ctx context.Context) (ThreadGoal, error)
func (t *SessionThread) ClearGoal(ctx context.Context) error
```

## TurnResult

```go
type TurnResult struct {
    Turn      Turn        // final turn state from the server
    Items     []Item      // items captured via streaming events
    Usage     *TokenUsage // token usage; nil when server does not report it
    Error     *TurnError  // non-nil when turn status is "failed"
    DeltaText string      // agent text from streaming deltas (fallback when Items empty)
}

// FinalAgentText returns the last agent-message text, checking Items, Turn.Items,
// and DeltaText in order. Returns "" if no agent text is found.
func (r *TurnResult) FinalAgentText() string
```

## Options

```go
// Transport
func WithStdioTransport(r io.ReadCloser, w io.WriteCloser) Option
func WithTransport(t Transport) Option
func WithWSTransport(dialCtx context.Context, endpoint string) Option
func WithHTTPTransport(endpoint string) Option              // WIP — requires ws-http-bridge sidecar
func WithReconnectingHTTPTransport(endpoint string) Option  // WIP — requires ws-http-bridge sidecar
func WithRetry(cfg RetryConfig) Option
func WithHTTPBearerToken(token string) Option
func WithWSBearerToken(token string) Option

// NOTE: HTTP transport is work-in-progress. The Codex app-server is WebSocket-only;
// WithHTTPTransport / WithReconnectingHTTPTransport require the ws-http-bridge sidecar
// (plugins/codex-server/ws-http-bridge) in front of the server, else requests 405.

// Handlers
func WithApprovalHandler(handler ApprovalHandler) Option
func WithRequestHandler(handler RequestHandler) Option

// Concurrency
func WithMaxThreads(n int) Option // -1 = unlimited; 0 = default (64)
```

## ThreadOption

```go
func WithThreadModel(model string) ThreadOption
func WithThreadApprovalPolicy(policy string) ThreadOption
func WithThreadApprovalMode(mode ApprovalMode) ThreadOption
func WithThreadCWD(cwd string) ThreadOption
func WithThreadEphemeral(ephemeral bool) ThreadOption
func WithInitialInput(input string) ThreadOption // runs first turn immediately after start
```

## TurnOption

```go
func WithModel(model string) TurnOption
func WithApprovalPolicy(policy string) TurnOption
func WithApprovalMode(mode ApprovalMode) TurnOption
func WithSandbox(policy string) TurnOption
func WithSandboxMode(mode SandboxMode) TurnOption
func WithCWD(cwd string) TurnOption
func WithEffort(effort string) TurnOption
func WithSkill(skill string) TurnOption
func WithInputs(inputs ...TurnInput) TurnOption
```

## Transport

```go
type Transport interface {
    Call(ctx context.Context, method string, params any, result any) error
    Notify(ctx context.Context, method string, params any) error
    SetRequestHandler(RequestHandler)
    Close() error
}
```

## Event Streaming

```go
type EventSubscription struct{}

func (s *EventSubscription) C() <-chan Event
func (s *EventSubscription) Close()

type Event struct {
    Method string
    Raw    json.RawMessage
    Value  any // typed payload; see table below
}

func (e Event) Decode(v any) error
```

Typed notification payloads:

```go
type TurnStartedEvent struct { ThreadID, TurnID string; Turn *Turn }
type TurnCompletedEvent struct { ThreadID, TurnID string; Status TurnStatus; Turn *Turn; Usage *TokenUsage }
type ItemStartedEvent struct { ThreadID, TurnID string; Item *Item }
type ItemCompletedEvent struct { ThreadID, TurnID string; Item *Item }
type ItemUpdatedEvent struct { ThreadID, TurnID string; Item *Item }
type ThreadTokenUsageUpdatedEvent struct { ThreadID string; Usage *TokenUsage }
type TurnDiffUpdatedEvent struct { ThreadID, TurnID string; Diff json.RawMessage }
type TurnPlanUpdatedEvent struct { ThreadID, TurnID string; Plan json.RawMessage }
type ErrorEvent struct { ThreadID, TurnID, Message, Code string; Data json.RawMessage }
type ThreadStartedEvent struct { ThreadID string; Thread *Thread }
type ThreadArchivedEvent struct { ThreadID string }
type ThreadUnarchivedEvent struct { ThreadID string }
type ThreadClosedEvent struct { ThreadID string }
type ThreadStatusChangedEvent struct { ThreadID string; Status ThreadStatus }
type ThreadGoalUpdatedEvent struct { ThreadID string; Goal *ThreadGoal }
type ThreadGoalClearedEvent struct { ThreadID string }
type ServerRequestResolvedEvent struct { ThreadID, RequestID string; Result json.RawMessage }

type ItemAgentMessageDeltaEvent struct { ThreadID, TurnID, ItemID, Text string; Delta json.RawMessage }
type ItemPlanDeltaEvent struct { ThreadID, TurnID, ItemID, Text string; Delta json.RawMessage }
type ItemReasoningSummaryTextDeltaEvent struct { ThreadID, TurnID, ItemID, Text string }
type ItemReasoningSummaryPartAddedEvent struct { ThreadID, TurnID, ItemID string; Part json.RawMessage }
type ItemReasoningTextDeltaEvent struct { ThreadID, TurnID, ItemID, Text string }
type ItemCommandExecutionOutputDeltaEvent struct { ThreadID, TurnID, ItemID, Stream, Output, Delta string }
type ItemFileChangePatchUpdatedEvent struct { ThreadID, TurnID, ItemID string; Patch json.RawMessage }
type ItemFileChangeOutputDeltaEvent struct { ThreadID, TurnID, ItemID, Output, Delta string }
type ItemAutoApprovalReviewStartedEvent struct { ThreadID, TurnID, ItemID string }
type ItemAutoApprovalReviewCompletedEvent struct { ThreadID, TurnID, ItemID string }
type RawNotificationEvent struct { ThreadID, TurnID, ItemID string }

// Extra notifications (events_extra.go) — 65/68 notifications are typed.
// NP1 thread/account lifecycle
type ThreadDeletedEvent struct { ThreadID string }
type ThreadNameUpdatedEvent struct { ThreadID, ThreadName string }
type ThreadCompactedEvent struct { ThreadID, TurnID string }
type ThreadSettingsUpdatedEvent struct { ThreadID string; ThreadSettings json.RawMessage }
type AccountUpdatedEvent struct { AuthMode, PlanType string }
type AccountRateLimitsUpdatedEvent struct { RateLimits json.RawMessage }
// NP2 warnings & model diagnostics
type WarningEvent struct { ThreadID, Message string }
type ConfigWarningEvent struct { Summary, Details, Path string; Range json.RawMessage }
type DeprecationNoticeEvent struct { Summary, Details string }
type GuardianWarningEvent struct { ThreadID, Message string }
type WindowsWorldWritableWarningEvent struct { SamplePaths []string; ExtraCount int; FailedScan bool }
type WindowsSandboxSetupCompletedEvent struct { Mode string; Success bool; Error string }
type ModelReroutedEvent struct { ThreadID, TurnID, FromModel, ToModel string; Reason json.RawMessage }
type ModelVerificationEvent struct { ThreadID, TurnID string; Verifications json.RawMessage }
type TurnModerationMetadataEvent struct { ThreadID, TurnID string; Metadata json.RawMessage }
// NP3 process / exec / hooks
type ProcessExitedEvent struct { ProcessHandle string; ExitCode int; Stdout string; StdoutCapReached bool; Stderr string; StderrCapReached bool }
type ProcessOutputDeltaEvent struct { ProcessHandle, Stream, DeltaBase64 string; CapReached bool }
type TerminalInteractionEvent struct { ThreadID, TurnID, ItemID, ProcessID, Stdin string }
type McpToolCallProgressEvent struct { ThreadID, TurnID, ItemID, Message string }
type HookStartedEvent struct { ThreadID, TurnID string; Run json.RawMessage }
type HookCompletedEvent struct { ThreadID, TurnID string; Run json.RawMessage }
type RawResponseItemCompletedEvent struct { ThreadID, TurnID string; Item json.RawMessage }
// NP4 subsystem status
type McpServerStatusUpdatedEvent struct { ThreadID, Name, Status, Error string }
type McpServerOauthLoginCompletedEvent struct { Name string; Success bool; Error string }
type SkillsChangedEvent struct { Cursor string; Limit int; ThreadID string; ForceRefetch bool }
type FsChangedEvent struct { WatchID string; ChangedPaths []string }
type AppListUpdatedEvent struct { Data json.RawMessage }
type RemoteControlStatusChangedEvent struct { Status, ServerName, InstallationID, EnvironmentID string }
type ExternalAgentConfigImportCompletedEvent struct { ImportID string; ItemTypeResults json.RawMessage }
type ExternalAgentConfigImportProgressEvent struct { ImportID string; ItemTypeResults json.RawMessage }
// NP5 realtime (voice/audio)
type ThreadRealtimeStartedEvent struct { ThreadID, RealtimeSessionID, Version string }
type ThreadRealtimeClosedEvent struct { ThreadID, Reason string }
type ThreadRealtimeErrorEvent struct { ThreadID, Message string }
type ThreadRealtimeItemAddedEvent struct { ThreadID string; Item json.RawMessage }
type ThreadRealtimeSdpEvent struct { ThreadID, SDP string }
type ThreadRealtimeOutputAudioDeltaEvent struct { ThreadID string; Audio json.RawMessage }
type ThreadRealtimeTranscriptDeltaEvent struct { ThreadID, Role, Delta string }
type ThreadRealtimeTranscriptDoneEvent struct { ThreadID, Role, Text string }
```

## Core Request/Response Types

```go
type ThreadStartRequest struct {
    Model          string `json:"model,omitempty"`
    CWD            string `json:"cwd,omitempty"`
    ApprovalPolicy string `json:"approvalPolicy,omitempty"`
    Ephemeral      bool   `json:"ephemeral,omitempty"`
}

type TurnStartRequest struct {
    ThreadID       string          `json:"threadId"`
    Input          string          `json:"input,omitempty"`
    CWD            string          `json:"cwd,omitempty"`
    ApprovalPolicy string          `json:"approvalPolicy,omitempty"`
    SandboxPolicy  string          `json:"sandboxPolicy,omitempty"`
    Model          string          `json:"model,omitempty"`
    Effort         string          `json:"effort,omitempty"`
    Skill          string          `json:"skill,omitempty"`
    OutputSchema   json.RawMessage `json:"outputSchema,omitempty"`
}
```

## Core Runtime Types

```go
type Thread struct {
    ID     string
    Name   string
    Status ThreadStatus
    Turns  []Turn
}

type Turn struct {
    ID          string
    Items       []Item
    ItemsView   TurnItemsView
    Status      TurnStatus
    Usage       *TokenUsage
    StartedAt   *time.Time
    CompletedAt *time.Time
    DurationMS  int64
}

type Item struct {
    ID      string
    Kind    ItemKind
    Payload json.RawMessage
}

type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

type ThreadGoal struct {
    Objective string
}
```

## Enums and Constants

```go
type ThreadStatus string  // wire values: "notLoaded", "idle", "systemError", "active"
type TurnStatus string    // wire values: "inProgress", "completed", "interrupted", "failed"
type TurnItemsView string // wire values: "notLoaded", "summary", "full"
type ItemKind string      // wire values: see below
type ApprovalMode string  // wire values: "auto", "on-request", "never", "always"
type SandboxMode string   // wire values: "read-only", "workspace-write", "full-auto"

const (
    ThreadStatusNotLoaded   ThreadStatus = "notLoaded"
    ThreadStatusIdle        ThreadStatus = "idle"
    ThreadStatusSystemError ThreadStatus = "systemError"
    ThreadStatusActive      ThreadStatus = "active"

    TurnStatusCompleted   TurnStatus = "completed"
    TurnStatusInterrupted TurnStatus = "interrupted"
    TurnStatusFailed      TurnStatus = "failed"
    TurnStatusInProgress  TurnStatus = "inProgress"

    TurnItemsViewNotLoaded TurnItemsView = "notLoaded"
    TurnItemsViewSummary   TurnItemsView = "summary"
    TurnItemsViewFull      TurnItemsView = "full"

    ApprovalModeAuto      ApprovalMode = "auto"
    ApprovalModeOnRequest ApprovalMode = "on-request"
    ApprovalModeNever     ApprovalMode = "never"
    ApprovalModeAlways    ApprovalMode = "always"
)
```

Item kind constants (value = wire string):

```go
ItemKindUserMessage         // "userMessage"
ItemKindHookPrompt          // "hookPrompt"
ItemKindAgentMessage        // "agentMessage"
ItemKindPlan                // "plan"
ItemKindReasoning           // "reasoning"
ItemKindCommandExecution    // "commandExecution"
ItemKindFileChange          // "fileChange"
ItemKindMCPToolCall         // "mcpToolCall"
ItemKindDynamicToolCall     // "dynamicToolCall"
ItemKindCollabAgentToolCall // "collabAgentToolCall"
ItemKindSubAgentActivity    // "subAgentActivity"
ItemKindWebSearch           // "webSearch"
ItemKindImageView           // "imageView"
ItemKindSleep               // "sleep"
ItemKindImageGeneration     // "imageGeneration"
ItemKindEnteredReviewMode   // "enteredReviewMode"
ItemKindExitedReviewMode    // "exitedReviewMode"
ItemKindContextCompaction   // "contextCompaction"
```

## Approval / Server Request Handling

```go
type ApprovalHandler interface {
    HandleCommandExecutionApproval(context.Context, CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error)
    HandleFileChangeApproval(context.Context, FileChangeApprovalRequest) (FileChangeApprovalResult, error)
    HandlePermissionsApproval(context.Context, PermissionsApprovalRequest) (PermissionsApprovalResult, error)
    HandleUserInputRequest(context.Context, UserInputRequest) (UserInputResult, error)
}

type RequestHandler interface {
    HandleServerRequest(context.Context, ServerRequest) (ServerResponse, error)
}

type RequestHandlerFunc func(context.Context, ServerRequest) (ServerResponse, error)
```

Selected approval decision constants:

```go
ApprovalDecisionAccept
ApprovalDecisionAcceptForSession
ApprovalDecisionAcceptWithExecPolicyAmendment
ApprovalDecisionApplyNetworkPolicyAmendment
ApprovalDecisionDecline
ApprovalDecisionCancel

FileChangeApprovalDecisionAccept
FileChangeApprovalDecisionAcceptForSession
FileChangeApprovalDecisionDecline
FileChangeApprovalDecisionCancel
```
