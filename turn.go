package codexgo

import (
	"strings"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

// TurnResult holds the outcome of a completed turn.
type TurnResult struct {
	// Turn is the final Turn data as returned by the server.
	Turn Turn
	// Items accumulates all items observed during the turn via streaming events.
	Items []Item
	// Usage is the token-usage snapshot from the TurnCompleted event, if present.
	Usage *TokenUsage
	// Error is the structured error from the TurnCompleted event, if any.
	Error *TurnError
	// DeltaText holds agent-message text reconstructed from streaming delta events.
	// It is only set when Items carry no extractable agent text.
	DeltaText string
}

// FinalAgentText returns the text of the last agent-message item in the result.
// It searches Items, Turn.Items, and finally DeltaText (reconstructed from
// streaming delta events). Returns "" if no agent message is found.
func (r *TurnResult) FinalAgentText() string {
	if r == nil {
		return ""
	}
	lists := [][]Item{r.Items, r.Turn.Items}
	for _, items := range lists {
		for i := len(items) - 1; i >= 0; i-- {
			if items[i].Kind != protocol.ItemKindAgentMessage {
				continue
			}
			if text := extractTextCandidate(items[i].PayloadBytes()); text != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	if r.DeltaText != "" {
		return strings.TrimSpace(r.DeltaText)
	}
	return ""
}

// TurnOption is a functional option that configures a TurnStartRequest.
type TurnOption func(*TurnStartRequest)

// WithModel sets the model for the turn.
func WithModel(model string) TurnOption {
	return func(r *TurnStartRequest) {
		r.Model = model
	}
}

// WithApprovalPolicy sets the approval policy for the turn.
func WithApprovalPolicy(policy string) TurnOption {
	return func(r *TurnStartRequest) {
		r.ApprovalPolicy = policy
	}
}

// WithSandbox sets the sandbox policy for the turn.
func WithSandbox(sandbox string) TurnOption {
	return func(r *TurnStartRequest) {
		r.SandboxPolicy = sandbox
	}
}

// WithApprovalMode sets a typed approval policy for the turn.
func WithApprovalMode(mode ApprovalMode) TurnOption {
	return func(r *TurnStartRequest) {
		r.ApprovalPolicy = string(mode)
	}
}

// WithSandboxMode sets a typed sandbox policy for the turn.
func WithSandboxMode(mode SandboxMode) TurnOption {
	return func(r *TurnStartRequest) {
		r.SandboxPolicy = string(mode)
	}
}

// WithInputs sets typed multi-part inputs for the turn, replacing any plain
// string input. Use TextInput, ImageInput, LocalImageInput, SkillInput, and
// MentionInput to construct the items.
func WithInputs(inputs ...TurnInput) TurnOption {
	return func(r *TurnStartRequest) {
		r.Input = encodeInputs(inputs)
	}
}

// WithCWD sets the working directory for the turn.
func WithCWD(cwd string) TurnOption {
	return func(r *TurnStartRequest) {
		r.CWD = cwd
	}
}

// WithEffort sets the effort level for the turn.
func WithEffort(effort string) TurnOption {
	return func(r *TurnStartRequest) {
		r.Effort = effort
	}
}

// WithSkill sets a specific skill to invoke for this turn.
func WithSkill(skill string) TurnOption {
	return func(r *TurnStartRequest) { r.Skill = skill }
}

// applyTurnOptions applies all TurnOption values to req.
func applyTurnOptions(req *TurnStartRequest, opts []TurnOption) {
	for _, o := range opts {
		o(req)
	}
}

// configUpdateRequest is the payload for the config/update RPC call.
type configUpdateRequest struct {
	Model          string `json:"model,omitempty"`
	ApprovalPolicy string `json:"approvalPolicy,omitempty"`
	SandboxPolicy  string `json:"sandboxPolicy,omitempty"`
}

// threadCompactRequest is the payload for the thread/compact RPC call.
type threadCompactRequest struct {
	ThreadID string `json:"threadId"`
}

// Ensure protocol constants are used (avoids import cycle if they're ever split).
var _ = protocol.MethodConfigUpdate
var _ = protocol.MethodThreadCompact
