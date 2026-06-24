package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	schematypes "github.com/zealbase/codex-app-server-go/internal/protocol/schema"
)

var ErrUnsupportedServerRequest = errors.New("protocol: unsupported server request")

// Request/response type aliases pointing at schema-generated definitions.

type ClientInfo = schematypes.ClientInfo
type Capabilities = schematypes.Capabilities
type InitializeRequest = schematypes.InitializeRequest
type InitializeResult = schematypes.InitializeResult
type ThreadStartRequest = schematypes.ThreadStartRequest
type ThreadResumeRequest = schematypes.ThreadResumeRequest
type ThreadReadRequest = schematypes.ThreadReadRequest
type TurnStartRequest = schematypes.TurnStartRequest
type TurnInterruptRequest = schematypes.TurnInterruptRequest

// ServerRequest / ServerResponse are protocol-layer helpers (not in schema).

type ServerRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type ServerResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
}

// Thread is the runtime representation of a thread with flexible time/status parsing.

type Thread struct {
	ID             string          `json:"id,omitempty"`
	SessionID      string          `json:"sessionId,omitempty"`
	ParentThreadID string          `json:"parentThreadId,omitempty"`
	ForkedFromID   string          `json:"forkedFromId,omitempty"`
	Preview        string          `json:"preview,omitempty"`
	Ephemeral      bool            `json:"ephemeral,omitempty"`
	ModelProvider  string          `json:"modelProvider,omitempty"`
	CreatedAt      *time.Time      `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time      `json:"updatedAt,omitempty"`
	Path           string          `json:"path,omitempty"`
	CWD            string          `json:"cwd,omitempty"`
	Status         ThreadStatus    `json:"status,omitempty"`
	Name           string          `json:"name,omitempty"`
	Turns          []Turn          `json:"turns,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

func (t *Thread) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID             string               `json:"id,omitempty"`
		SessionID      string               `json:"sessionId,omitempty"`
		ParentThreadID string               `json:"parentThreadId,omitempty"`
		ForkedFromID   string               `json:"forkedFromId,omitempty"`
		Preview        string               `json:"preview,omitempty"`
		Ephemeral      bool                 `json:"ephemeral,omitempty"`
		ModelProvider  string               `json:"modelProvider,omitempty"`
		CreatedAt      *flexibleTime        `json:"createdAt,omitempty"`
		UpdatedAt      *flexibleTime        `json:"updatedAt,omitempty"`
		Path           string               `json:"path,omitempty"`
		CWD            string               `json:"cwd,omitempty"`
		Status         flexibleThreadStatus `json:"status,omitempty"`
		Name           string               `json:"name,omitempty"`
		Turns          []Turn               `json:"turns,omitempty"`
		Rollout        []Turn               `json:"rollout,omitempty"`
	}
	var v alias
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Thread{
		ID:             v.ID,
		SessionID:      v.SessionID,
		ParentThreadID: v.ParentThreadID,
		ForkedFromID:   v.ForkedFromID,
		Preview:        v.Preview,
		Ephemeral:      v.Ephemeral,
		ModelProvider:  v.ModelProvider,
		Path:           v.Path,
		CWD:            v.CWD,
		Status:         v.Status.ThreadStatus,
		Name:           v.Name,
		Turns:          v.Turns,
	}
	if len(t.Turns) == 0 && len(v.Rollout) > 0 {
		t.Turns = v.Rollout
	}
	if v.CreatedAt != nil {
		t.CreatedAt = &v.CreatedAt.Time
	}
	if v.UpdatedAt != nil {
		t.UpdatedAt = &v.UpdatedAt.Time
	}
	t.Raw = make([]byte, len(data))
	copy(t.Raw, data)
	return nil
}

// Turn is the runtime representation of a turn with flexible time parsing.

type Turn struct {
	ID          string          `json:"id,omitempty"`
	Items       []Item          `json:"items,omitempty"`
	ItemsView   TurnItemsView   `json:"itemsView,omitempty"`
	Status      TurnStatus      `json:"status,omitempty"`
	Error       json.RawMessage `json:"error,omitempty"`
	StartedAt   *time.Time      `json:"startedAt,omitempty"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	DurationMS  int64           `json:"durationMs,omitempty"`
	Usage       *TokenUsage     `json:"usage,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID          string          `json:"id,omitempty"`
		Items       []Item          `json:"items,omitempty"`
		ItemsView   TurnItemsView   `json:"itemsView,omitempty"`
		Status      TurnStatus      `json:"status,omitempty"`
		Error       json.RawMessage `json:"error,omitempty"`
		StartedAt   *flexibleTime   `json:"startedAt,omitempty"`
		CompletedAt *flexibleTime   `json:"completedAt,omitempty"`
		DurationMS  int64           `json:"durationMs,omitempty"`
		Usage       *TokenUsage     `json:"usage,omitempty"`
	}
	var v alias
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = Turn{
		ID:         v.ID,
		Items:      v.Items,
		ItemsView:  v.ItemsView,
		Status:     v.Status,
		Error:      v.Error,
		DurationMS: v.DurationMS,
		Usage:      v.Usage,
	}
	if v.StartedAt != nil {
		t.StartedAt = &v.StartedAt.Time
	}
	if v.CompletedAt != nil {
		t.CompletedAt = &v.CompletedAt.Time
	}
	t.Raw = make([]byte, len(data))
	copy(t.Raw, data)
	return nil
}

// flexibleTime handles Unix-timestamp and RFC-3339 time values from the server.
type flexibleTime struct {
	time.Time
}

// flexibleThreadStatus handles both string and object-shaped status payloads.
type flexibleThreadStatus struct {
	ThreadStatus
}

func (s *flexibleThreadStatus) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("empty thread status")
	}
	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		s.ThreadStatus = ThreadStatus(raw)
		return nil
	}

	// Newer servers encode "active" as an object that carries active flags.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err == nil {
		for _, key := range []string{"status", "state", "kind", "value"} {
			if raw, ok := payload[key]; ok {
				var parsed string
				if err := json.Unmarshal(raw, &parsed); err == nil && parsed != "" {
					s.ThreadStatus = ThreadStatus(parsed)
					return nil
				}
			}
		}
		if _, ok := payload["activeFlags"]; ok {
			s.ThreadStatus = ThreadStatusActive
			return nil
		}
	}

	// Fall back to the active state for any object-shaped status payload.
	s.ThreadStatus = ThreadStatusActive
	return nil
}

func (t *flexibleTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("empty time value")
	}
	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				t.Time = parsed
				return nil
			}
		}
		return fmt.Errorf("invalid time string %q", raw)
	}

	n, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(string(data), 64)
		if ferr != nil {
			return fmt.Errorf("invalid time value %q", string(data))
		}
		n = int64(f)
	}
	t.Time = timeFromUnixScalar(n)
	return nil
}

func timeFromUnixScalar(n int64) time.Time {
	if n > 1e12 || n < -1e12 {
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}
