// Code generated from sdk/codex-go/internal/protocol/schema/v2.schema.json. DO NOT EDIT.
// Custom extensions (MarshalJSON, DecodeParams, etc.) live in separate *_ext.go files.
package schema

import (
	"encoding/json"
	"time"
)

// --- Enum types ---

type ThreadStatus string

const (
	ThreadStatusNotLoaded   ThreadStatus = "notLoaded"
	ThreadStatusIdle        ThreadStatus = "idle"
	ThreadStatusSystemError ThreadStatus = "systemError"
	ThreadStatusActive      ThreadStatus = "active"
)

type ThreadActiveFlag string

const (
	ThreadActiveFlagWaitingOnApproval  ThreadActiveFlag = "waitingOnApproval"
	ThreadActiveFlagWaitingOnUserInput ThreadActiveFlag = "waitingOnUserInput"
)

type TurnStatus string

const (
	TurnStatusCompleted   TurnStatus = "completed"
	TurnStatusInterrupted TurnStatus = "interrupted"
	TurnStatusFailed      TurnStatus = "failed"
	TurnStatusInProgress  TurnStatus = "inProgress"
)

type TurnItemsView string

const (
	TurnItemsViewNotLoaded TurnItemsView = "notLoaded"
	TurnItemsViewSummary   TurnItemsView = "summary"
	TurnItemsViewFull      TurnItemsView = "full"
)

type ItemKind string

const (
	ItemKindUserMessage         ItemKind = "userMessage"
	ItemKindHookPrompt          ItemKind = "hookPrompt"
	ItemKindAgentMessage        ItemKind = "agentMessage"
	ItemKindPlan                ItemKind = "plan"
	ItemKindReasoning           ItemKind = "reasoning"
	ItemKindCommandExecution    ItemKind = "commandExecution"
	ItemKindFileChange          ItemKind = "fileChange"
	ItemKindMCPToolCall         ItemKind = "mcpToolCall"
	ItemKindDynamicToolCall     ItemKind = "dynamicToolCall"
	ItemKindCollabAgentToolCall ItemKind = "collabAgentToolCall"
	ItemKindSubAgentActivity    ItemKind = "subAgentActivity"
	ItemKindWebSearch           ItemKind = "webSearch"
	ItemKindImageView           ItemKind = "imageView"
	ItemKindSleep               ItemKind = "sleep"
	ItemKindImageGeneration     ItemKind = "imageGeneration"
	ItemKindEnteredReviewMode   ItemKind = "enteredReviewMode"
	ItemKindExitedReviewMode    ItemKind = "exitedReviewMode"
	ItemKindContextCompaction   ItemKind = "contextCompaction"
)

type ApprovalDecision string

const (
	ApprovalDecisionAccept                        ApprovalDecision = "accept"
	ApprovalDecisionAcceptForSession              ApprovalDecision = "acceptForSession"
	ApprovalDecisionAcceptWithExecpolicyAmendment ApprovalDecision = "acceptWithExecpolicyAmendment"
	ApprovalDecisionApplyNetworkPolicyAmendment   ApprovalDecision = "applyNetworkPolicyAmendment"
	ApprovalDecisionDecline                       ApprovalDecision = "decline"
	ApprovalDecisionCancel                        ApprovalDecision = "cancel"
)

type FileChangeApprovalDecision string

const (
	FileChangeApprovalDecisionAccept           FileChangeApprovalDecision = "accept"
	FileChangeApprovalDecisionAcceptForSession FileChangeApprovalDecision = "acceptForSession"
	FileChangeApprovalDecisionDecline          FileChangeApprovalDecision = "decline"
	FileChangeApprovalDecisionCancel           FileChangeApprovalDecision = "cancel"
)

type PermissionsScope string

const (
	PermissionsScopeSession PermissionsScope = "session"
	PermissionsScopeTurn    PermissionsScope = "turn"
)

// --- Shared data types ---

type TokenUsage struct {
	InputTokens     int64 `json:"inputTokens,omitempty"`
	OutputTokens    int64 `json:"outputTokens,omitempty"`
	TotalTokens     int64 `json:"totalTokens,omitempty"`
	ReasoningTokens int64 `json:"reasoningTokens,omitempty"`
}

type TurnError struct {
	Message string          `json:"message,omitempty"`
	Code    string          `json:"code,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type GitInfo struct {
	Root     string `json:"root,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Remote   string `json:"remote,omitempty"`
	Dirty    bool   `json:"dirty,omitempty"`
	Detached bool   `json:"detached,omitempty"`
}

// --- Core data models (schema-faithful; runtime types with custom decoders live in protocol) ---

// SchemaItem mirrors the Item definition from the schema.
// For the full runtime type with payload encoding, see protocol.Item.
type SchemaItem struct {
	ID   string   `json:"id,omitempty"`
	Type ItemKind `json:"type"`
}

// SchemaTurn mirrors the Turn definition from the schema.
// For the full runtime type with flexible time parsing, see protocol.Turn.
type SchemaTurn struct {
	ID          string        `json:"id,omitempty"`
	Items       []SchemaItem  `json:"items,omitempty"`
	ItemsView   TurnItemsView `json:"itemsView,omitempty"`
	Status      TurnStatus    `json:"status,omitempty"`
	Error       *TurnError    `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"startedAt,omitempty"`
	CompletedAt *time.Time    `json:"completedAt,omitempty"`
	DurationMS  int64         `json:"durationMs,omitempty"`
	Usage       *TokenUsage   `json:"usage,omitempty"`
}

// SchemaThread mirrors the Thread definition from the schema.
// For the full runtime type with flexible status decoding, see protocol.Thread.
type SchemaThread struct {
	ID             string       `json:"id,omitempty"`
	SessionID      string       `json:"sessionId,omitempty"`
	ForkedFromID   string       `json:"forkedFromId,omitempty"`
	ParentThreadID string       `json:"parentThreadId,omitempty"`
	Preview        string       `json:"preview,omitempty"`
	Ephemeral      bool         `json:"ephemeral,omitempty"`
	ModelProvider  string       `json:"modelProvider,omitempty"`
	CreatedAt      *time.Time   `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time   `json:"updatedAt,omitempty"`
	Status         ThreadStatus `json:"status,omitempty"`
	Path           string       `json:"path,omitempty"`
	CWD            string       `json:"cwd,omitempty"`
	CliVersion     string       `json:"cliVersion,omitempty"`
	Source         string       `json:"source,omitempty"`
	ThreadSource   string       `json:"threadSource,omitempty"`
	AgentNickname  string       `json:"agentNickname,omitempty"`
	AgentRole      string       `json:"agentRole,omitempty"`
	GitInfo        *GitInfo     `json:"gitInfo,omitempty"`
	Name           string       `json:"name,omitempty"`
	Turns          []SchemaTurn `json:"turns,omitempty"`
}

// --- RPC envelope types ---

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type RPCRequest struct {
	Version string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCNotification struct {
	Version string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// InitializedNotification is the empty notification sent after Initialize.
type InitializedNotification struct{}

// --- Initialize ---

type ClientInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type Capabilities struct {
	ExperimentalAPI                bool `json:"experimentalApi,omitempty"`
	OptOutNotificationMethods      bool `json:"optOutNotificationMethods,omitempty"`
	MCPServerOpenAIFormElicitation bool `json:"mcpServerOpenaiFormElicitation,omitempty"`
}

type InitializeRequest struct {
	ClientInfo   ClientInfo   `json:"clientInfo"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

type InitializeResult struct {
	UserAgent      string `json:"userAgent,omitempty"`
	CodexHome      string `json:"codexHome,omitempty"`
	PlatformFamily string `json:"platformFamily,omitempty"`
	PlatformOS     string `json:"platformOs,omitempty"`
}

// --- Thread/Turn RPC requests ---

type ThreadStartRequest struct {
	Model                 string          `json:"model,omitempty"`
	CWD                   string          `json:"cwd,omitempty"`
	ApprovalPolicy        string          `json:"approvalPolicy,omitempty"`
	RuntimeWorkspaceRoots []string        `json:"runtimeWorkspaceRoots,omitempty"`
	Environments          []string        `json:"environments,omitempty"`
	Personality           string          `json:"personality,omitempty"`
	DynamicTools          []string        `json:"dynamicTools,omitempty"`
	Ephemeral             bool            `json:"ephemeral,omitempty"`
	Metadata              json.RawMessage `json:"metadata,omitempty"`
}

type ThreadResumeRequest struct {
	ThreadID         string   `json:"threadId"`
	History          []string `json:"history,omitempty"`
	Path             string   `json:"path,omitempty"`
	ExcludeTurns     []string `json:"excludeTurns,omitempty"`
	InitialTurnsPage int      `json:"initialTurnsPage,omitempty"`
}

type ThreadReadRequest struct {
	ThreadID     string `json:"threadId"`
	IncludeTurns bool   `json:"includeTurns,omitempty"`
}

type TurnStartRequest struct {
	ThreadID            string          `json:"threadId"`
	Input               string          `json:"input,omitempty"`
	ClientUserMessageID string          `json:"clientUserMessageId,omitempty"`
	CWD                 string          `json:"cwd,omitempty"`
	ApprovalPolicy      string          `json:"approvalPolicy,omitempty"`
	SandboxPolicy       string          `json:"sandboxPolicy,omitempty"`
	Permissions         []string        `json:"permissions,omitempty"`
	Model               string          `json:"model,omitempty"`
	ServiceTier         string          `json:"serviceTier,omitempty"`
	Effort              string          `json:"effort,omitempty"`
	Summary             string          `json:"summary,omitempty"`
	OutputSchema        json.RawMessage `json:"outputSchema,omitempty"`
	CollaborationMode   string          `json:"collaborationMode,omitempty"`
	MultiAgentMode      string          `json:"multiAgentMode,omitempty"`
	Environments        []string        `json:"environments,omitempty"`
	// Skill triggers a specific named skill for this turn (using $ prefix equivalent).
	Skill               string          `json:"skill,omitempty"`
}

type TurnInterruptRequest struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// --- Thread fork / list / archive / setName / rollback ---

type ThreadForkRequest struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId,omitempty"` // fork at specific turn
}

type ThreadForkResult struct {
	Thread SchemaThread `json:"thread"`
}

type ThreadListRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type ThreadListResult struct {
	Threads []SchemaThread `json:"threads"`
	Cursor  string         `json:"cursor,omitempty"`
}

type ThreadArchiveRequest struct {
	ThreadID string `json:"threadId"`
}

type ThreadUnarchiveRequest struct {
	ThreadID string `json:"threadId"`
}

type ThreadSetNameRequest struct {
	ThreadID string `json:"threadId"`
	Name     string `json:"name"`
}

type ThreadRollbackRequest struct {
	ThreadID string   `json:"threadId"`
	TurnIDs  []string `json:"turnIds"`
}

// --- Turn steer ---

type TurnSteerRequest struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Input    string `json:"input"`
}

// --- Review ---

type ReviewStartRequest struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId,omitempty"`
}

// --- Turn diff ---

type TurnDiffRequest struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId,omitempty"`
}

type TurnDiffResult struct {
	Diff string `json:"diff,omitempty"`
}

// --- Skills ---

// SkillScope enumerates where a skill is sourced from.
type SkillScope string

const (
	SkillScopeUser   SkillScope = "user"
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeSystem SkillScope = "system"
	SkillScopeAdmin  SkillScope = "admin"
)

// SkillErrorInfo describes a parse/load error for a skill file.
type SkillErrorInfo struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

// SkillInterface holds UI-facing metadata for a skill (display name, icon, etc.).
type SkillInterface struct {
	BrandColor       string `json:"brandColor,omitempty"`
	DefaultPrompt    string `json:"defaultPrompt,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	IconLarge        string `json:"iconLarge,omitempty"`
	IconSmall        string `json:"iconSmall,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
}

// SkillToolDependency describes a tool dependency declared by a skill.
type SkillToolDependency struct {
	Type        string `json:"type"`
	Value       string `json:"value"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	Transport   string `json:"transport,omitempty"`
	URL         string `json:"url,omitempty"`
}

// SkillDependencies lists tool dependencies for a skill.
type SkillDependencies struct {
	Tools []SkillToolDependency `json:"tools"`
}

// SkillMetadata is the full metadata for a skill (name, path, scope, etc.).
type SkillMetadata struct {
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Enabled          bool               `json:"enabled"`
	Path             string             `json:"path"`
	Scope            SkillScope         `json:"scope"`
	ShortDescription string             `json:"shortDescription,omitempty"`
	Interface        *SkillInterface    `json:"interface,omitempty"`
	Dependencies     *SkillDependencies `json:"dependencies,omitempty"`
}

// SkillSummary is a lighter view of a skill (no path or scope required).
type SkillSummary struct {
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Enabled          bool            `json:"enabled"`
	Path             string          `json:"path,omitempty"`
	ShortDescription string          `json:"shortDescription,omitempty"`
	Interface        *SkillInterface `json:"interface,omitempty"`
}

// SkillsListEntry groups skills and errors for a single cwd.
type SkillsListEntry struct {
	CWD    string          `json:"cwd"`
	Skills []SkillMetadata `json:"skills"`
	Errors []SkillErrorInfo `json:"errors"`
}

// SkillsListParams is the request type for the skills/list RPC.
type SkillsListParams struct {
	CWDs        []string `json:"cwds,omitempty"`
	ForceReload bool     `json:"forceReload,omitempty"`
}

// SkillsListResponse is the response type for the skills/list RPC.
type SkillsListResponse struct {
	Data []SkillsListEntry `json:"data"`
}

// SkillsChangedNotification is sent when watched skill files change on disk.
type SkillsChangedNotification struct{}

// SkillsConfigWriteParams enables or disables a skill by name or path.
type SkillsConfigWriteParams struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
}

// SkillsConfigWriteResponse reports the effective enabled state after a config write.
type SkillsConfigWriteResponse struct {
	EffectiveEnabled bool `json:"effectiveEnabled"`
}

// SkillsExtraRootsSetParams sets extra filesystem roots to scan for skills.
type SkillsExtraRootsSetParams struct {
	ExtraRoots []string `json:"extraRoots"`
}

// SkillsExtraRootsSetResponse is the empty response for skills/extraRoots/set.
type SkillsExtraRootsSetResponse struct{}

// PluginSkillReadParams reads the raw content of a remote plugin skill.
type PluginSkillReadParams struct {
	RemoteMarketplaceName string `json:"remoteMarketplaceName"`
	RemotePluginID        string `json:"remotePluginId"`
	SkillName             string `json:"skillName"`
}

// PluginSkillReadResponse returns the raw content of a remote plugin skill.
type PluginSkillReadResponse struct {
	Contents string `json:"contents,omitempty"`
}

// --- ThreadGoal ---

// ThreadGoalStatus enumerates the lifecycle states of a thread goal.
type ThreadGoalStatus string

const (
	ThreadGoalStatusActive       ThreadGoalStatus = "active"
	ThreadGoalStatusPaused       ThreadGoalStatus = "paused"
	ThreadGoalStatusBlocked      ThreadGoalStatus = "blocked"
	ThreadGoalStatusUsageLimited ThreadGoalStatus = "usageLimited"
	ThreadGoalStatusBudgetLimited ThreadGoalStatus = "budgetLimited"
	ThreadGoalStatusComplete     ThreadGoalStatus = "complete"
)

// ThreadGoal tracks a goal/objective set for a thread, including usage budgets.
type ThreadGoal struct {
	ThreadID        string           `json:"threadId"`
	Objective       string           `json:"objective"`
	Status          ThreadGoalStatus `json:"status"`
	CreatedAt       int64            `json:"createdAt"`
	UpdatedAt       int64            `json:"updatedAt"`
	TimeUsedSeconds int64            `json:"timeUsedSeconds"`
	TokensUsed      int64            `json:"tokensUsed"`
	TokenBudget     *int64           `json:"tokenBudget,omitempty"`
}

// ThreadGoalSetParams is the request type for threadGoal/set.
type ThreadGoalSetParams struct {
	ThreadID    string            `json:"threadId"`
	Objective   string            `json:"objective,omitempty"`
	Status      *ThreadGoalStatus `json:"status,omitempty"`
	TokenBudget *int64            `json:"tokenBudget,omitempty"`
}

// ThreadGoalSetResponse is the response type for threadGoal/set.
type ThreadGoalSetResponse struct {
	Goal ThreadGoal `json:"goal"`
}

// ThreadGoalGetParams is the request type for threadGoal/get.
type ThreadGoalGetParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadGoalGetResponse is the response type for threadGoal/get.
type ThreadGoalGetResponse struct {
	Goal *ThreadGoal `json:"goal,omitempty"`
}

// ThreadGoalClearParams is the request type for threadGoal/clear.
type ThreadGoalClearParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadGoalClearResponse is the response type for threadGoal/clear.
type ThreadGoalClearResponse struct {
	Cleared bool `json:"cleared"`
}

// ThreadGoalUpdatedNotification is emitted when a thread's goal is created or updated.
type ThreadGoalUpdatedNotification struct {
	ThreadID string     `json:"threadId"`
	TurnID   string     `json:"turnId,omitempty"`
	Goal     ThreadGoal `json:"goal"`
}

// ThreadGoalClearedNotification is emitted when a thread's goal is cleared.
type ThreadGoalClearedNotification struct {
	ThreadID string `json:"threadId"`
}
