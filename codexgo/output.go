package codexgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/jsonx"
	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
)

func (c *Client) WaitForFinalAgentMessage(ctx context.Context, threadID, turnID string) (string, error) {
	turn, collector, err := c.waitForTurnOutput(ctx, threadID, turnID)
	if err != nil {
		return "", err
	}

	text := strings.TrimSpace(collector.finalAgentMessage(turn))
	if text == "" {
		return "", fmt.Errorf("no final agent message found for turn %s", turnID)
	}
	return text, nil
}

func (c *Client) WaitForStructuredOutput(ctx context.Context, threadID, turnID string, out any) (Turn, error) {
	if out == nil {
		return Turn{}, errors.New("out is required")
	}

	turn, collector, err := c.waitForTurnOutput(ctx, threadID, turnID)
	if err != nil {
		return Turn{}, err
	}

	payload := collector.structuredPayload(turn)
	if len(payload) == 0 {
		return turn, fmt.Errorf("no structured output found for turn %s", turnID)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return turn, fmt.Errorf("decode structured output: %w", err)
	}
	return turn, nil
}

func (c *Client) waitForTurnOutput(ctx context.Context, threadID, turnID string) (Turn, *outputCollector, error) {
	sub := c.Events()
	defer sub.Close()
	return c.waitForTurnOutputSub(ctx, threadID, turnID, sub)
}

// waitForTurnOutputSub is the core implementation. It accepts a pre-existing
// EventSubscription so callers can subscribe before initiating the turn (avoiding
// a race where events fire before the subscription is set up).
func (c *Client) waitForTurnOutputSub(ctx context.Context, threadID, turnID string, sub *EventSubscription) (Turn, *outputCollector, error) {
	collector := &outputCollector{}

	ticker := time.NewTicker(defaultWaitPollInterval)
	defer ticker.Stop()

	transientCount := 0

	for {
		turn, err := c.readTurn(ctx, threadID, turnID)
		if err == nil {
			transientCount = 0
			collector.captureTurn(turn)
			if isTerminalTurnStatus(turn.Status) {
				collector.capturePendingEvents(sub, threadID, turnID)
				// When a turn completes with an LLM call the server may write
				// items/usage asynchronously. If both the polled turn and the
				// event capture are empty, retry once after a short delay so
				// the data has time to settle. Skip the retry for interrupted
				// or failed turns — they genuinely have no items.
				if turn.Status == protocol.TurnStatusCompleted &&
					len(turn.Items) == 0 &&
					len(collector.items) == 0 &&
					len(collector.agentTextDeltas) == 0 &&
					collector.eventUsage == nil {
					turn = c.retryReadTurnForItems(ctx, threadID, turnID, turn, collector, sub)
				}
				return turn, collector, nil
			}
		} else if errors.Is(err, errTurnNotFound) {
			transientCount = 0
		} else if isTransientError(err) {
			transientCount++
			if transientCount >= maxTransientRetries {
				return Turn{}, nil, err
			}
		} else {
			return Turn{}, nil, err
		}

		select {
		case <-ctx.Done():
			return Turn{}, nil, ctx.Err()
		case event, ok := <-sub.C():
			if !ok {
				// Broker closed (client shutting down). Nil out the subscription
				// so this case is never selected again; ctx.Done() will exit.
				sub = nil
				continue
			}
			if !eventMatchesTurn(event, threadID, turnID) {
				continue
			}
			collector.captureEvent(event)
			if event.Method == protocol.MethodTurnCompleted {
				turn, err := c.readTurn(ctx, threadID, turnID)
				if err == nil {
					collector.captureTurn(turn)
					return turn, collector, nil
				}
				if tc, ok := event.Value.(TurnCompletedEvent); ok && tc.Turn != nil {
					collector.captureTurn(*tc.Turn)
					return *tc.Turn, collector, nil
				}
			}
		case <-ticker.C:
		}
	}
}

type outputCollector struct {
	turn            *Turn
	items           []Item
	itemPayloads    [][]byte
	agentTextDeltas []string
	eventUsage      *TokenUsage // captured from TurnCompletedEvent.Usage (top-level field)
}

func (c *outputCollector) captureTurn(turn Turn) {
	turnCopy := turn
	c.turn = &turnCopy
	c.items = mergeItems(c.items, turn.Items)
	for _, item := range turn.Items {
		payload := item.PayloadBytes()
		if len(payload) > 0 {
			c.itemPayloads = append(c.itemPayloads, payload)
		}
	}
	if len(turn.Raw) > 0 {
		c.itemPayloads = append(c.itemPayloads, cloneBytes(turn.Raw))
	}
}

func (c *outputCollector) captureEvent(event Event) {
	switch v := event.Value.(type) {
	case ItemCompletedEvent:
		if v.Item != nil {
			c.items = append(c.items, *v.Item)
			if payload := v.Item.PayloadBytes(); len(payload) > 0 {
				c.itemPayloads = append(c.itemPayloads, payload)
			}
		}
	case ItemAgentMessageDeltaEvent:
		if text := firstNonEmpty(v.Text, extractStringCandidate(event.Raw)); text != "" {
			c.agentTextDeltas = append(c.agentTextDeltas, text)
		}
	case TurnCompletedEvent:
		if v.Turn != nil {
			c.captureTurn(*v.Turn)
		}
		if v.Usage != nil {
			c.eventUsage = v.Usage
		}
	}
}

// retryReadTurnForItems retries readTurn up to 3 times (50 ms apart) when a
// completed turn has no items yet. It also drains any pending events between
// retries. The best turn seen is returned; collector is updated in-place.
func (c *Client) retryReadTurnForItems(ctx context.Context, threadID, turnID string, initial Turn, col *outputCollector, sub *EventSubscription) Turn {
	best := initial
	for range 3 {
		select {
		case <-ctx.Done():
			return best
		case <-time.After(50 * time.Millisecond):
		}
		col.capturePendingEvents(sub, threadID, turnID)
		if len(col.items) > 0 || len(col.agentTextDeltas) > 0 || col.eventUsage != nil {
			return best
		}
		refreshed, err := c.readTurn(ctx, threadID, turnID)
		if err != nil {
			continue
		}
		col.captureTurn(refreshed)
		best = refreshed
		if len(refreshed.Items) > 0 || refreshed.Usage != nil {
			return best
		}
	}
	return best
}

// deltaText returns the concatenated agent-message delta text, or "" if none.
func (c *outputCollector) deltaText() string {
	if len(c.agentTextDeltas) == 0 {
		return ""
	}
	return strings.Join(c.agentTextDeltas, "")
}

func (c *outputCollector) capturePendingEvents(sub *EventSubscription, threadID, turnID string) {
	if sub == nil {
		return
	}
	for {
		select {
		case event, ok := <-sub.C():
			if !ok {
				return
			}
			if !eventMatchesTurn(event, threadID, turnID) {
				continue
			}
			c.captureEvent(event)
		default:
			return
		}
	}
}

func (c *outputCollector) finalAgentMessage(turn Turn) string {
	for i := len(turn.Items) - 1; i >= 0; i-- {
		if turn.Items[i].Kind != protocol.ItemKindAgentMessage {
			continue
		}
		if text := extractTextCandidate(turn.Items[i].PayloadBytes()); text != "" {
			return text
		}
	}
	for i := len(c.items) - 1; i >= 0; i-- {
		if c.items[i].Kind != protocol.ItemKindAgentMessage {
			continue
		}
		if text := extractTextCandidate(c.items[i].PayloadBytes()); text != "" {
			return text
		}
	}
	if len(c.agentTextDeltas) > 0 {
		return strings.Join(c.agentTextDeltas, "")
	}
	for i := len(c.itemPayloads) - 1; i >= 0; i-- {
		if text := extractTextCandidate(c.itemPayloads[i]); text != "" {
			return text
		}
	}
	return ""
}

func (c *outputCollector) structuredPayload(turn Turn) []byte {
	for i := len(c.items) - 1; i >= 0; i-- {
		if payload := extractStructuredCandidate(c.items[i].PayloadBytes()); len(payload) > 0 {
			return payload
		}
	}
	for i := len(turn.Items) - 1; i >= 0; i-- {
		if payload := extractStructuredCandidate(turn.Items[i].PayloadBytes()); len(payload) > 0 {
			return payload
		}
	}
	for i := len(c.itemPayloads) - 1; i >= 0; i-- {
		if payload := extractStructuredCandidate(c.itemPayloads[i]); len(payload) > 0 {
			return payload
		}
	}
	if text := strings.TrimSpace(strings.Join(c.agentTextDeltas, "")); json.Valid([]byte(text)) {
		return []byte(text)
	}
	return nil
}

// JSON/text extraction helpers live in internal/jsonx (dependency-free, reusable).
// These package-private aliases keep call sites in this package unchanged.
var (
	extractStructuredCandidate = jsonx.ExtractStructuredCandidate
	extractTextCandidate       = jsonx.ExtractTextCandidate
	extractTextValue           = jsonx.ExtractTextValue
	extractStringCandidate     = jsonx.ExtractStringCandidate
	collectJSONCandidates      = jsonx.CollectJSONCandidates
	stripCodeFence             = jsonx.StripCodeFence
	firstNonEmpty              = jsonx.FirstNonEmpty
	cloneBytes                 = jsonx.CloneBytes
)

func mergeItems(existing []Item, incoming []Item) []Item {
	if len(existing) == 0 {
		return append([]Item(nil), incoming...)
	}
	merged := append([]Item(nil), existing...)
	indexByID := make(map[string]int, len(merged))
	for i, item := range merged {
		if item.ID != "" {
			indexByID[item.ID] = i
		}
	}
	for _, item := range incoming {
		if item.ID != "" {
			if idx, ok := indexByID[item.ID]; ok {
				merged[idx] = item
				continue
			}
			indexByID[item.ID] = len(merged)
		}
		merged = append(merged, item)
	}
	return merged
}
