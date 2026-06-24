package protocol

import (
	"encoding/json"

	schematypes "github.com/zealbase/codex-app-server-go/codexgo/internal/protocol/schema"
)

// Approval decision enum aliases -- canonical definitions live in the generated schema package.

type ApprovalDecision = schematypes.ApprovalDecision
type FileChangeApprovalDecision = schematypes.FileChangeApprovalDecision
type PermissionsScope = schematypes.PermissionsScope

const (
	ApprovalDecisionAccept                        = schematypes.ApprovalDecisionAccept
	ApprovalDecisionAcceptForSession              = schematypes.ApprovalDecisionAcceptForSession
	ApprovalDecisionAcceptWithExecpolicyAmendment = schematypes.ApprovalDecisionAcceptWithExecpolicyAmendment
	ApprovalDecisionApplyNetworkPolicyAmendment   = schematypes.ApprovalDecisionApplyNetworkPolicyAmendment
	ApprovalDecisionDecline                       = schematypes.ApprovalDecisionDecline
	ApprovalDecisionCancel                        = schematypes.ApprovalDecisionCancel
)

const (
	FileChangeApprovalDecisionAccept           = schematypes.FileChangeApprovalDecisionAccept
	FileChangeApprovalDecisionAcceptForSession = schematypes.FileChangeApprovalDecisionAcceptForSession
	FileChangeApprovalDecisionDecline          = schematypes.FileChangeApprovalDecisionDecline
	FileChangeApprovalDecisionCancel           = schematypes.FileChangeApprovalDecisionCancel
)

const (
	PermissionsScopeSession = schematypes.PermissionsScopeSession
	PermissionsScopeTurn    = schematypes.PermissionsScopeTurn
)

// Approval request/response types (not in the protocol schema definition).

type CommandExecutionApprovalRequest struct {
	ItemID                 string          `json:"itemId,omitempty"`
	ThreadID               string          `json:"threadId,omitempty"`
	TurnID                 string          `json:"turnId,omitempty"`
	EnvironmentID          string          `json:"environmentId,omitempty"`
	ApprovalID             string          `json:"approvalId,omitempty"`
	Reason                 string          `json:"reason,omitempty"`
	Command                string          `json:"command,omitempty"`
	Cwd                    string          `json:"cwd,omitempty"`
	CommandActions         []string        `json:"commandActions,omitempty"`
	NetworkApprovalContext json.RawMessage `json:"networkApprovalContext,omitempty"`
}

type CommandExecutionApprovalResponse struct {
	Decision ApprovalDecision `json:"decision"`
}

type FileChangeApprovalRequest struct {
	ItemID    string   `json:"itemId,omitempty"`
	ThreadID  string   `json:"threadId,omitempty"`
	TurnID    string   `json:"turnId,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	GrantRoot string   `json:"grantRoot,omitempty"`
	FilePaths []string `json:"filePaths,omitempty"`
	Diff      string   `json:"diff,omitempty"`
}

type FileChangeApprovalResponse struct {
	Decision FileChangeApprovalDecision `json:"decision"`
}

type PermissionsApprovalRequest struct {
	ItemID      string           `json:"itemId,omitempty"`
	ThreadID    string           `json:"threadId,omitempty"`
	TurnID      string           `json:"turnId,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	Permissions []string         `json:"permissions,omitempty"`
	Scope       PermissionsScope `json:"scope,omitempty"`
}

type PermissionsApprovalResponse struct {
	Permissions []string         `json:"permissions,omitempty"`
	Scope       PermissionsScope `json:"scope,omitempty"`
}

type UserInputOption struct {
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type UserInputQuestion struct {
	Header   string            `json:"header,omitempty"`
	ID       string            `json:"id,omitempty"`
	Question string            `json:"question,omitempty"`
	Options  []UserInputOption `json:"options,omitempty"`
}

type UserInputRequest struct {
	ItemID    string              `json:"itemId,omitempty"`
	ThreadID  string              `json:"threadId,omitempty"`
	TurnID    string              `json:"turnId,omitempty"`
	Questions []UserInputQuestion `json:"questions,omitempty"`
}

type UserInputResponse struct {
	Answers map[string]string `json:"answers,omitempty"`
}

type UserInputResult = UserInputResponse

type MCPServerElicitationRequest struct {
	ItemID   string          `json:"itemId,omitempty"`
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	Kind     string          `json:"kind,omitempty"`
	Form     json.RawMessage `json:"form,omitempty"`
	URL      string          `json:"url,omitempty"`
}

type MCPServerElicitationResponse struct {
	Decision ApprovalDecision `json:"decision"`
}

type MCPToolCallApprovalRequest struct {
	ToolName   string          `json:"toolName"`
	ServerName string          `json:"serverName,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	ThreadID   string          `json:"threadId,omitempty"`
	TurnID     string          `json:"turnId,omitempty"`
}

type MCPToolCallApprovalResponse struct {
	Decision string `json:"decision"` // "accept", "acceptForSession", "decline", "cancel"
}

type AttestationGenerateResponse struct {
	Token string `json:"token"`
}

type CurrentTimeReadResponse struct {
	CurrentTimeAt int64 `json:"currentTimeAt"`
}
