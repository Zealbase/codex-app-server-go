package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestItemRoundTrip(t *testing.T) {
	original, err := NewItem(ItemKindCommandExecution, struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
	}{Command: "ls", Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	original.ID = "item_123"

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Item
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("ID mismatch: got %q want %q", decoded.ID, original.ID)
	}
	if decoded.Kind != original.Kind {
		t.Fatalf("Kind mismatch: got %q want %q", decoded.Kind, original.Kind)
	}

	var payload struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
	}
	if err := decoded.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Command != "ls" || payload.Cwd != "/tmp" {
		t.Fatalf("payload mismatch: %+v", payload)
	}
}

func TestRPCNotificationDecodeParams(t *testing.T) {
	evt := TurnStartedEvent{
		ThreadID: "thread_1",
		TurnID:   "turn_1",
		Turn: &Turn{
			ID:     "turn_1",
			Status: TurnStatusInProgress,
		},
	}
	params, err := RawJSON(evt)
	if err != nil {
		t.Fatalf("RawJSON: %v", err)
	}

	n := RPCNotification{
		Version: JSONRPCVersion,
		Method:  MethodTurnStarted,
		Params:  params,
	}

	var decoded TurnStartedEvent
	if err := n.DecodeParams(&decoded); err != nil {
		t.Fatalf("DecodeParams: %v", err)
	}
	if decoded.ThreadID != evt.ThreadID || decoded.TurnID != evt.TurnID || decoded.Turn == nil {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
}

func TestThreadMarshalRoundTrip(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	original := Thread{
		ID:        "thread_1",
		Preview:   "first pass",
		Status:    ThreadStatusIdle,
		CreatedAt: &now,
		Turns: []Turn{
			{
				ID:     "turn_1",
				Status: TurnStatusCompleted,
				Usage:  &TokenUsage{InputTokens: 12, OutputTokens: 7, TotalTokens: 19},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Thread
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID || decoded.Preview != original.Preview || decoded.Status != original.Status {
		t.Fatalf("decoded mismatch: %+v", decoded)
	}
	if len(decoded.Turns) != 1 || decoded.Turns[0].ID != "turn_1" || decoded.Turns[0].Usage == nil || decoded.Turns[0].Usage.TotalTokens != 19 {
		t.Fatalf("turn mismatch: %+v", decoded.Turns)
	}
}
