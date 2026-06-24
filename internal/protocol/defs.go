package protocol

import (
	"encoding/json"

	schematypes "github.com/zealbase/codex-app-server-go/internal/protocol/schema"
)

// Enum / data type aliases -- canonical definitions live in the generated schema package.

type ThreadStatus = schematypes.ThreadStatus
type ThreadActiveFlag = schematypes.ThreadActiveFlag
type TurnStatus = schematypes.TurnStatus
type TurnItemsView = schematypes.TurnItemsView
type ItemKind = schematypes.ItemKind
type TokenUsage = schematypes.TokenUsage
type TurnError = schematypes.TurnError

// Re-export constants so existing call-sites inside the protocol package compile.

const (
	ThreadStatusNotLoaded   = schematypes.ThreadStatusNotLoaded
	ThreadStatusIdle        = schematypes.ThreadStatusIdle
	ThreadStatusSystemError = schematypes.ThreadStatusSystemError
	ThreadStatusActive      = schematypes.ThreadStatusActive
)

const (
	ThreadActiveFlagWaitingOnApproval  = schematypes.ThreadActiveFlagWaitingOnApproval
	ThreadActiveFlagWaitingOnUserInput = schematypes.ThreadActiveFlagWaitingOnUserInput
)

const (
	TurnStatusCompleted   = schematypes.TurnStatusCompleted
	TurnStatusInterrupted = schematypes.TurnStatusInterrupted
	TurnStatusFailed      = schematypes.TurnStatusFailed
	TurnStatusInProgress  = schematypes.TurnStatusInProgress
)

const (
	TurnItemsViewNotLoaded = schematypes.TurnItemsViewNotLoaded
	TurnItemsViewSummary   = schematypes.TurnItemsViewSummary
	TurnItemsViewFull      = schematypes.TurnItemsViewFull
)

const (
	ItemKindUserMessage         = schematypes.ItemKindUserMessage
	ItemKindHookPrompt          = schematypes.ItemKindHookPrompt
	ItemKindAgentMessage        = schematypes.ItemKindAgentMessage
	ItemKindPlan                = schematypes.ItemKindPlan
	ItemKindReasoning           = schematypes.ItemKindReasoning
	ItemKindCommandExecution    = schematypes.ItemKindCommandExecution
	ItemKindFileChange          = schematypes.ItemKindFileChange
	ItemKindMCPToolCall         = schematypes.ItemKindMCPToolCall
	ItemKindDynamicToolCall     = schematypes.ItemKindDynamicToolCall
	ItemKindCollabAgentToolCall = schematypes.ItemKindCollabAgentToolCall
	ItemKindSubAgentActivity    = schematypes.ItemKindSubAgentActivity
	ItemKindWebSearch           = schematypes.ItemKindWebSearch
	ItemKindImageView           = schematypes.ItemKindImageView
	ItemKindSleep               = schematypes.ItemKindSleep
	ItemKindImageGeneration     = schematypes.ItemKindImageGeneration
	ItemKindEnteredReviewMode   = schematypes.ItemKindEnteredReviewMode
	ItemKindExitedReviewMode    = schematypes.ItemKindExitedReviewMode
	ItemKindContextCompaction   = schematypes.ItemKindContextCompaction
)

// Event types (server-push notification payloads; not in the protocol schema).

type TurnStartedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	Turn     *Turn  `json:"turn,omitempty"`
}

type TurnCompletedEvent struct {
	ThreadID string      `json:"threadId,omitempty"`
	TurnID   string      `json:"turnId,omitempty"`
	Status   TurnStatus  `json:"status,omitempty"`
	Turn     *Turn       `json:"turn,omitempty"`
	Usage    *TokenUsage `json:"usage,omitempty"`
}

type ItemStartedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	Item     *Item  `json:"item,omitempty"`
}

type ItemCompletedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	Item     *Item  `json:"item,omitempty"`
}

type ThreadTokenUsageUpdatedEvent struct {
	ThreadID string      `json:"threadId,omitempty"`
	Usage    *TokenUsage `json:"usage,omitempty"`
}

type TurnDiffUpdatedEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	Diff     json.RawMessage `json:"diff,omitempty"`
}

type TurnPlanUpdatedEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	Plan     json.RawMessage `json:"plan,omitempty"`
}

type ErrorEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	Message  string          `json:"message,omitempty"`
	Code     string          `json:"code,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type ThreadStartedEvent struct {
	ThreadID string  `json:"threadId,omitempty"`
	Thread   *Thread `json:"thread,omitempty"`
}

type ThreadArchivedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
}

type ThreadUnarchivedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
}

type ThreadClosedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
}

type ThreadStatusChangedEvent struct {
	ThreadID string       `json:"threadId,omitempty"`
	Status   ThreadStatus `json:"status,omitempty"`
}

type ThreadGoalUpdatedEvent struct {
	ThreadID string                   `json:"threadId,omitempty"`
	Goal     *schematypes.ThreadGoal  `json:"goal,omitempty"`
}

type ThreadGoalClearedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
}

type ItemUpdatedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	Item     *Item  `json:"item,omitempty"`
}

type ServerRequestResolvedEvent struct {
	ThreadID  string          `json:"threadId,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}
