package codexgo

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
)

type (
	TurnStartedEvent             = protocol.TurnStartedEvent
	TurnCompletedEvent           = protocol.TurnCompletedEvent
	ItemStartedEvent             = protocol.ItemStartedEvent
	ItemCompletedEvent           = protocol.ItemCompletedEvent
	ThreadTokenUsageUpdatedEvent = protocol.ThreadTokenUsageUpdatedEvent
	TurnDiffUpdatedEvent         = protocol.TurnDiffUpdatedEvent
	TurnPlanUpdatedEvent         = protocol.TurnPlanUpdatedEvent
	ErrorEvent                   = protocol.ErrorEvent
	ThreadStartedEvent           = protocol.ThreadStartedEvent
	ThreadArchivedEvent          = protocol.ThreadArchivedEvent
	ThreadUnarchivedEvent        = protocol.ThreadUnarchivedEvent
	ThreadClosedEvent            = protocol.ThreadClosedEvent
	ThreadStatusChangedEvent     = protocol.ThreadStatusChangedEvent
	ThreadGoalUpdatedEvent       = protocol.ThreadGoalUpdatedEvent
	ThreadGoalClearedEvent       = protocol.ThreadGoalClearedEvent
	ItemUpdatedEvent             = protocol.ItemUpdatedEvent
	ServerRequestResolvedEvent   = protocol.ServerRequestResolvedEvent
)

type ItemAgentMessageDeltaEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	ItemID   string          `json:"itemId,omitempty"`
	Delta    json.RawMessage `json:"delta,omitempty"`
	Text     string          `json:"text,omitempty"`
}

type ItemPlanDeltaEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	ItemID   string          `json:"itemId,omitempty"`
	Delta    json.RawMessage `json:"delta,omitempty"`
	Text     string          `json:"text,omitempty"`
}

type ItemReasoningSummaryTextDeltaEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
	Text     string `json:"text,omitempty"`
}

type ItemReasoningSummaryPartAddedEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	ItemID   string          `json:"itemId,omitempty"`
	Part     json.RawMessage `json:"part,omitempty"`
}

type ItemReasoningTextDeltaEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
	Text     string `json:"text,omitempty"`
}

type ItemCommandExecutionOutputDeltaEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
	Stream   string `json:"stream,omitempty"`
	Output   string `json:"output,omitempty"`
	Delta    string `json:"delta,omitempty"`
}

// CommandExecOutputDeltaEvent is a streaming output chunk from an interactive
// command/exec process (the command/exec/outputDelta notification). DeltaBase64
// is the base64-encoded output for the given stream ("stdout" or "stderr").
type CommandExecOutputDeltaEvent struct {
	ProcessID   string `json:"processId,omitempty"`
	Stream      string `json:"stream,omitempty"`
	DeltaBase64 string `json:"deltaBase64,omitempty"`
	CapReached  bool   `json:"capReached,omitempty"`
}

type ItemFileChangePatchUpdatedEvent struct {
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	ItemID   string          `json:"itemId,omitempty"`
	Patch    json.RawMessage `json:"patch,omitempty"`
}

type ItemFileChangeOutputDeltaEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
	Output   string `json:"output,omitempty"`
	Delta    string `json:"delta,omitempty"`
}

type ItemAutoApprovalReviewStartedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
}

type ItemAutoApprovalReviewCompletedEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
}

type RawNotificationEvent struct {
	ThreadID string `json:"threadId,omitempty"`
	TurnID   string `json:"turnId,omitempty"`
	ItemID   string `json:"itemId,omitempty"`
}

// Event is one server-push notification emitted by the app-server.
// Value is a concrete typed event struct when the method is recognized.
type Event struct {
	Method string
	Raw    json.RawMessage
	Value  any
}

// Decode unmarshals the raw notification payload into v.
func (e Event) Decode(v any) error {
	if len(e.Raw) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(e.Raw, v)
}

// ThreadEvent wraps any event that can appear on the streaming channel returned
// by SessionThread.RunStreamed.
//
// Kind is the JSON-RPC notification method name (e.g. "turn/started").
// Raw holds the concrete typed event value (one of the *Event structs in this
// package); callers can type-switch on Raw to access strongly-typed fields.
type ThreadEvent struct {
	Kind string
	Raw  any
}

func decodeEvent(method string, raw json.RawMessage) Event {
	event := Event{
		Method: method,
		Raw:    cloneRawMessage(raw),
	}

	var target any
	switch method {
	case protocol.MethodTurnStarted:
		target = &TurnStartedEvent{}
	case protocol.MethodTurnCompleted:
		target = &TurnCompletedEvent{}
	case protocol.MethodTurnDiffUpdated:
		target = &TurnDiffUpdatedEvent{}
	case protocol.MethodTurnPlanUpdated:
		target = &TurnPlanUpdatedEvent{}
	case protocol.MethodThreadTokenUsageUpdated:
		target = &ThreadTokenUsageUpdatedEvent{}
	case protocol.MethodItemStarted:
		target = &ItemStartedEvent{}
	case protocol.MethodItemCompleted:
		target = &ItemCompletedEvent{}
	case protocol.MethodItemAgentMessageDelta:
		target = &ItemAgentMessageDeltaEvent{}
	case protocol.MethodItemPlanDelta:
		target = &ItemPlanDeltaEvent{}
	case protocol.MethodItemReasoningSummaryTextDelta:
		target = &ItemReasoningSummaryTextDeltaEvent{}
	case protocol.MethodItemReasoningSummaryPartAdded:
		target = &ItemReasoningSummaryPartAddedEvent{}
	case protocol.MethodItemReasoningTextDelta:
		target = &ItemReasoningTextDeltaEvent{}
	case protocol.MethodItemCommandExecutionOutputDelta:
		target = &ItemCommandExecutionOutputDeltaEvent{}
	case protocol.MethodCommandExecOutputDelta:
		target = &CommandExecOutputDeltaEvent{}
	case protocol.MethodItemFileChangePatchUpdated:
		target = &ItemFileChangePatchUpdatedEvent{}
	case protocol.MethodItemFileChangeOutputDelta:
		target = &ItemFileChangeOutputDeltaEvent{}
	case protocol.MethodItemAutoApprovalReviewStarted:
		target = &ItemAutoApprovalReviewStartedEvent{}
	case protocol.MethodItemAutoApprovalReviewCompleted:
		target = &ItemAutoApprovalReviewCompletedEvent{}
	case protocol.MethodThreadStarted:
		target = &ThreadStartedEvent{}
	case protocol.MethodThreadArchived:
		target = &ThreadArchivedEvent{}
	case protocol.MethodThreadUnarchived:
		target = &ThreadUnarchivedEvent{}
	case protocol.MethodThreadClosed:
		target = &ThreadClosedEvent{}
	case protocol.MethodThreadStatusChanged:
		target = &ThreadStatusChangedEvent{}
	case protocol.MethodThreadGoalUpdated:
		target = &ThreadGoalUpdatedEvent{}
	case protocol.MethodThreadGoalCleared:
		target = &ThreadGoalClearedEvent{}
	case protocol.MethodItemUpdated:
		target = &ItemUpdatedEvent{}
	case protocol.MethodServerRequestResolved:
		target = &ServerRequestResolvedEvent{}
	case protocol.MethodAccountLoginCompleted:
		target = &LoginCompleted{}
	case protocol.MethodError:
		target = &ErrorEvent{}
	default:
		if t := extraEventTarget(method); t != nil {
			target = t
		} else {
			target = &RawNotificationEvent{}
		}
	}

	if len(raw) == 0 {
		event.Value = derefEventValue(target)
		return event
	}
	if err := json.Unmarshal(raw, target); err != nil {
		fallback := &RawNotificationEvent{}
		if json.Unmarshal(raw, fallback) == nil {
			event.Value = *fallback
			return event
		}
		event.Value = RawNotificationEvent{}
		return event
	}
	event.Value = derefEventValue(target)
	return event
}

func derefEventValue(v any) any {
	switch x := v.(type) {
	case *TurnStartedEvent:
		return *x
	case *TurnCompletedEvent:
		return *x
	case *TurnDiffUpdatedEvent:
		return *x
	case *TurnPlanUpdatedEvent:
		return *x
	case *ThreadTokenUsageUpdatedEvent:
		return *x
	case *ItemStartedEvent:
		return *x
	case *ItemCompletedEvent:
		return *x
	case *ItemAgentMessageDeltaEvent:
		return *x
	case *ItemPlanDeltaEvent:
		return *x
	case *ItemReasoningSummaryTextDeltaEvent:
		return *x
	case *ItemReasoningSummaryPartAddedEvent:
		return *x
	case *ItemReasoningTextDeltaEvent:
		return *x
	case *ItemCommandExecutionOutputDeltaEvent:
		return *x
	case *CommandExecOutputDeltaEvent:
		return *x
	case *ItemFileChangePatchUpdatedEvent:
		return *x
	case *ItemFileChangeOutputDeltaEvent:
		return *x
	case *ItemAutoApprovalReviewStartedEvent:
		return *x
	case *ItemAutoApprovalReviewCompletedEvent:
		return *x
	case *ThreadStartedEvent:
		return *x
	case *ThreadArchivedEvent:
		return *x
	case *ThreadUnarchivedEvent:
		return *x
	case *ThreadClosedEvent:
		return *x
	case *ThreadStatusChangedEvent:
		return *x
	case *ThreadGoalUpdatedEvent:
		return *x
	case *ThreadGoalClearedEvent:
		return *x
	case *ItemUpdatedEvent:
		return *x
	case *ServerRequestResolvedEvent:
		return *x
	case *LoginCompleted:
		return *x
	case *ErrorEvent:
		return *x
	case *RawNotificationEvent:
		return *x
	default:
		if dv, ok := derefExtraEvent(v); ok {
			return dv
		}
		return nil
	}
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

type EventSubscription struct {
	ch     <-chan Event
	cancel func()
}

func (s *EventSubscription) C() <-chan Event {
	if s == nil {
		return nil
	}
	return s.ch
}

func (s *EventSubscription) Close() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
}

type eventBroker struct {
	mu     sync.Mutex
	nextID uint64
	subs   map[uint64]chan Event
	closed bool
}

func newEventBroker() *eventBroker {
	return &eventBroker{subs: make(map[uint64]chan Event)}
}

func (b *eventBroker) Subscribe() *EventSubscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 128)
	if b.closed {
		close(ch)
		return &EventSubscription{ch: ch}
	}
	b.nextID++
	id := b.nextID
	b.subs[id] = ch

	return &EventSubscription{
		ch: ch,
		cancel: func() {
			b.unsubscribe(id)
		},
	}
}

func (b *eventBroker) publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	for id, ch := range b.subs {
		select {
		case ch <- event:
		default:
			close(ch)
			delete(b.subs, id)
		}
	}
}

func (b *eventBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subs {
		close(ch)
		delete(b.subs, id)
	}
}

func (b *eventBroker) unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, ok := b.subs[id]
	if !ok {
		return
	}
	close(ch)
	delete(b.subs, id)
}

// StreamText reads ThreadEvents from ch and writes agent message text deltas
// to w. It returns when ch is closed or ctx is done.
// Useful for printing streaming responses to a terminal or HTTP response writer.
func StreamText(ctx context.Context, ch <-chan ThreadEvent, w io.Writer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if delta, ok := ev.Raw.(ItemAgentMessageDeltaEvent); ok {
				if delta.Text != "" {
					if _, err := io.WriteString(w, delta.Text); err != nil {
						return err
					}
				}
			}
		}
	}
}
