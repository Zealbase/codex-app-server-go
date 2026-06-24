// marshal_ext.go holds custom MarshalJSON implementations that extend the
// generated schema types. This file is NOT regenerated; it survives schema
// updates intact.
package schema

import (
	"encoding/json"
	"strings"
)

// MarshalJSON omits the Ephemeral field when false so the wire format stays
// compact. The alias breaks the recursion that would otherwise occur when
// calling json.Marshal on the same type.
func (r ThreadStartRequest) MarshalJSON() ([]byte, error) {
	type alias struct {
		Model                 string          `json:"model,omitempty"`
		CWD                   string          `json:"cwd,omitempty"`
		ApprovalPolicy        string          `json:"approvalPolicy,omitempty"`
		RuntimeWorkspaceRoots []string        `json:"runtimeWorkspaceRoots,omitempty"`
		Environments          []string        `json:"environments,omitempty"`
		Personality           string          `json:"personality,omitempty"`
		DynamicTools          []string        `json:"dynamicTools,omitempty"`
		Metadata              json.RawMessage `json:"metadata,omitempty"`
	}
	return json.Marshal(alias{
		Model:                 r.Model,
		CWD:                   r.CWD,
		ApprovalPolicy:        r.ApprovalPolicy,
		RuntimeWorkspaceRoots: r.RuntimeWorkspaceRoots,
		Environments:          r.Environments,
		Personality:           r.Personality,
		DynamicTools:          r.DynamicTools,
		Metadata:              r.Metadata,
	})
}

// wireInput converts a string Input field into the multi-part array shape the
// server expects. When the string is a pre-encoded JSON array (typed inputs),
// it is passed through verbatim.
func wireInput(s string) any {
	switch {
	case strings.HasPrefix(strings.TrimSpace(s), "["):
		return json.RawMessage(s)
	case s != "":
		return []map[string]string{{"type": "text", "text": s}}
	default:
		return []map[string]string{}
	}
}

// MarshalJSON converts the string Input into the multi-part array format the
// server expects and passes all other fields through unchanged.
func (r TurnSteerRequest) MarshalJSON() ([]byte, error) {
	type alias struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Input    any    `json:"input"`
	}
	return json.Marshal(alias{
		ThreadID: r.ThreadID,
		TurnID:   r.TurnID,
		Input:    wireInput(r.Input),
	})
}

// MarshalJSON converts the string Input into the multi-part array format the
// server expects and passes all other fields through unchanged.
func (r TurnStartRequest) MarshalJSON() ([]byte, error) {
	type alias struct {
		ThreadID            string   `json:"threadId"`
		Input               any      `json:"input"`
		ClientUserMessageID string   `json:"clientUserMessageId,omitempty"`
		CWD                 string   `json:"cwd,omitempty"`
		ApprovalPolicy      string   `json:"approvalPolicy,omitempty"`
		SandboxPolicy       string   `json:"sandboxPolicy,omitempty"`
		Permissions         []string `json:"permissions,omitempty"`
		Model               string   `json:"model,omitempty"`
		ServiceTier         string   `json:"serviceTier,omitempty"`
		Effort              string   `json:"effort,omitempty"`
		Summary             string   `json:"summary,omitempty"`
		OutputSchema        any      `json:"outputSchema,omitempty"`
		CollaborationMode   string   `json:"collaborationMode,omitempty"`
		MultiAgentMode      string   `json:"multiAgentMode,omitempty"`
		Environments        []string `json:"environments,omitempty"`
	}

	return json.Marshal(alias{
		ThreadID:            r.ThreadID,
		Input:               wireInput(r.Input),
		ClientUserMessageID: r.ClientUserMessageID,
		CWD:                 r.CWD,
		ApprovalPolicy:      r.ApprovalPolicy,
		SandboxPolicy:       r.SandboxPolicy,
		Permissions:         r.Permissions,
		Model:               r.Model,
		ServiceTier:         r.ServiceTier,
		Effort:              r.Effort,
		Summary:             r.Summary,
		OutputSchema:        r.OutputSchema,
		CollaborationMode:   r.CollaborationMode,
		MultiAgentMode:      r.MultiAgentMode,
		Environments:        r.Environments,
	})
}
