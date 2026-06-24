// Package codexgo provides a compact client for the Codex app-server.
package codexgo

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
	schematypes "github.com/zealbase/codex-app-server-go/codexgo/internal/protocol/schema"
	"github.com/zealbase/codex-app-server-go/codexgo/internal/transport"
)

type (
	ClientInfo             = schematypes.ClientInfo
	Capabilities           = schematypes.Capabilities
	InitializeRequest      = schematypes.InitializeRequest
	InitializeResult       = schematypes.InitializeResult
	ThreadStartRequest     = schematypes.ThreadStartRequest
	ThreadResumeRequest    = schematypes.ThreadResumeRequest
	ThreadReadRequest      = schematypes.ThreadReadRequest
	TurnStartRequest       = schematypes.TurnStartRequest
	TurnInterruptRequest   = schematypes.TurnInterruptRequest
	ThreadForkRequest      = schematypes.ThreadForkRequest
	ThreadListRequest      = schematypes.ThreadListRequest
	ThreadArchiveRequest   = schematypes.ThreadArchiveRequest
	ThreadUnarchiveRequest = schematypes.ThreadUnarchiveRequest
	ThreadSetNameRequest   = schematypes.ThreadSetNameRequest
	ThreadRollbackRequest  = schematypes.ThreadRollbackRequest
	TurnSteerRequest       = schematypes.TurnSteerRequest
	ReviewStartRequest     = schematypes.ReviewStartRequest
	TurnDiffRequest        = schematypes.TurnDiffRequest
	TurnDiffResult         = schematypes.TurnDiffResult
	Thread                 = protocol.Thread
	Turn                   = protocol.Turn
	Item                   = protocol.Item
	ThreadStatus           = protocol.ThreadStatus
	ThreadActiveFlag       = protocol.ThreadActiveFlag
	TurnStatus             = protocol.TurnStatus
	TurnItemsView          = protocol.TurnItemsView
	ItemKind               = protocol.ItemKind
	TokenUsage             = protocol.TokenUsage
	TurnError              = protocol.TurnError
)

const (
	ThreadStatusNotLoaded   = protocol.ThreadStatusNotLoaded
	ThreadStatusIdle        = protocol.ThreadStatusIdle
	ThreadStatusSystemError = protocol.ThreadStatusSystemError
	ThreadStatusActive      = protocol.ThreadStatusActive
)

const (
	ThreadActiveFlagWaitingOnApproval  = protocol.ThreadActiveFlagWaitingOnApproval
	ThreadActiveFlagWaitingOnUserInput = protocol.ThreadActiveFlagWaitingOnUserInput
)

const (
	TurnStatusCompleted   = protocol.TurnStatusCompleted
	TurnStatusInterrupted = protocol.TurnStatusInterrupted
	TurnStatusFailed      = protocol.TurnStatusFailed
	TurnStatusInProgress  = protocol.TurnStatusInProgress
)

const (
	TurnItemsViewNotLoaded = protocol.TurnItemsViewNotLoaded
	TurnItemsViewSummary   = protocol.TurnItemsViewSummary
	TurnItemsViewFull      = protocol.TurnItemsViewFull
)

const (
	ItemKindUserMessage         = protocol.ItemKindUserMessage
	ItemKindHookPrompt          = protocol.ItemKindHookPrompt
	ItemKindAgentMessage        = protocol.ItemKindAgentMessage
	ItemKindPlan                = protocol.ItemKindPlan
	ItemKindReasoning           = protocol.ItemKindReasoning
	ItemKindCommandExecution    = protocol.ItemKindCommandExecution
	ItemKindFileChange          = protocol.ItemKindFileChange
	ItemKindMCPToolCall         = protocol.ItemKindMCPToolCall
	ItemKindDynamicToolCall     = protocol.ItemKindDynamicToolCall
	ItemKindCollabAgentToolCall = protocol.ItemKindCollabAgentToolCall
	ItemKindSubAgentActivity    = protocol.ItemKindSubAgentActivity
	ItemKindWebSearch           = protocol.ItemKindWebSearch
	ItemKindImageView           = protocol.ItemKindImageView
	ItemKindSleep               = protocol.ItemKindSleep
	ItemKindImageGeneration     = protocol.ItemKindImageGeneration
	ItemKindEnteredReviewMode   = protocol.ItemKindEnteredReviewMode
	ItemKindExitedReviewMode    = protocol.ItemKindExitedReviewMode
	ItemKindContextCompaction   = protocol.ItemKindContextCompaction
)

// ApprovalMode is a typed approval policy for turns and threads. The string
// values match the wire values the app-server expects for approvalPolicy.
type ApprovalMode string

const (
	ApprovalModeDenyAll    ApprovalMode = "deny_all"
	ApprovalModeAutoReview ApprovalMode = "auto_review"
	ApprovalModeOnRequest  ApprovalMode = "on-request"
	ApprovalModeNever      ApprovalMode = "never"
)

// SandboxMode is a typed sandbox policy for turns. The string values match the
// wire values the app-server expects for sandboxPolicy.
type SandboxMode string

const (
	SandboxReadOnly       SandboxMode = "read-only"
	SandboxWorkspaceWrite SandboxMode = "workspace-write"
	SandboxFullAccess     SandboxMode = "danger-full-access"
)

// CodexError wraps an RPCError and exposes the structured codexErrorInfo payload.
type CodexError struct {
	RPCCode        int
	RPCMessage     string
	ErrorType      string // codexErrorInfo.type  e.g. "HttpConnectionFailed"
	HTTPStatusCode int    // codexErrorInfo.httpStatusCode e.g. 429
	AdditionalInfo string // additionalDetails
}

func (e *CodexError) Error() string {
	if e.HTTPStatusCode != 0 {
		return fmt.Sprintf("codex error %d (HTTP %d): %s", e.RPCCode, e.HTTPStatusCode, e.RPCMessage)
	}
	return fmt.Sprintf("codex error %d: %s", e.RPCCode, e.RPCMessage)
}

// AsCodexError unwraps err into a *CodexError if the underlying cause is a
// transport.RPCError that carries a codexErrorInfo payload.
func AsCodexError(err error) (*CodexError, bool) {
	var rpcErr *transport.RPCError
	if !errors.As(err, &rpcErr) {
		return nil, false
	}
	ce := &CodexError{RPCCode: rpcErr.Code, RPCMessage: rpcErr.Message}
	if len(rpcErr.Data) > 0 {
		var envelope struct {
			CodexErrorInfo struct {
				Type           string `json:"type"`
				HTTPStatusCode int    `json:"httpStatusCode"`
			} `json:"codexErrorInfo"`
			AdditionalDetails string `json:"additionalDetails"`
		}
		if json.Unmarshal(rpcErr.Data, &envelope) == nil {
			ce.ErrorType = envelope.CodexErrorInfo.Type
			ce.HTTPStatusCode = envelope.CodexErrorInfo.HTTPStatusCode
			ce.AdditionalInfo = envelope.AdditionalDetails
		}
	}
	return ce, true
}

// IsRateLimited reports whether err is an HTTP 429 rate-limit error from Codex.
func IsRateLimited(err error) bool {
	ce, ok := AsCodexError(err)
	return ok && ce.HTTPStatusCode == 429
}

// IsUnauthorized reports whether err is an HTTP 401 authentication error.
func IsUnauthorized(err error) bool {
	ce, ok := AsCodexError(err)
	return ok && ce.HTTPStatusCode == 401
}

// IsInternalServerError reports whether err is an HTTP 500 from Codex.
func IsInternalServerError(err error) bool {
	ce, ok := AsCodexError(err)
	return ok && ce.HTTPStatusCode == 500
}

// IsHttpConnectionFailed reports whether err is a connection failure.
func IsHttpConnectionFailed(err error) bool {
	ce, ok := AsCodexError(err)
	return ok && ce.ErrorType == "HttpConnectionFailed"
}

// Package codexgo provides a compact client for the Codex app-server.
