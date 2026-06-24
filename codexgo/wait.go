package codexgo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
)

const defaultWaitPollInterval = 750 * time.Millisecond

func (c *Client) Events() *EventSubscription {
	if c == nil || c.events == nil {
		return closedEventSubscription()
	}
	return c.events.Subscribe()
}

// maxTransientRetries is the maximum number of consecutive transient errors
// (e.g. rate-limit or connection failure) before WaitForTurn surfaces the error.
const maxTransientRetries = 5

// isTransientError reports whether err is a transient error that WaitForTurn
// should retry rather than surface immediately.
func isTransientError(err error) bool {
	if IsRateLimited(err) {
		return true
	}
	if IsHttpConnectionFailed(err) {
		return true
	}
	// The codex binary can transiently fail with "rollout is empty" while it is
	// writing the first entry for a new turn. This resolves quickly.
	if err != nil && strings.Contains(err.Error(), "is empty") {
		return true
	}
	return false
}

func (c *Client) WaitForTurn(ctx context.Context, threadID, turnID string) (Turn, error) {
	if strings.TrimSpace(threadID) == "" {
		return Turn{}, errors.New("threadID is required")
	}
	if strings.TrimSpace(turnID) == "" {
		return Turn{}, errors.New("turnID is required")
	}

	sub := c.Events()
	defer sub.Close()

	ticker := time.NewTicker(defaultWaitPollInterval)
	defer ticker.Stop()

	transientCount := 0

	check := func() (Turn, bool, error) {
		turn, err := c.readTurn(ctx, threadID, turnID)
		if err != nil {
			return Turn{}, false, err
		}
		if isTerminalTurnStatus(turn.Status) {
			return turn, true, nil
		}
		return turn, false, nil
	}

	for {
		if turn, done, err := check(); err == nil {
			transientCount = 0
			if done {
				return turn, nil
			}
		} else if errors.Is(err, errTurnNotFound) {
			transientCount = 0
		} else if isTransientError(err) {
			transientCount++
			if transientCount >= maxTransientRetries {
				return Turn{}, err
			}
			// Let the ticker drive the next retry.
		} else {
			return Turn{}, err
		}

		select {
		case <-ctx.Done():
			return Turn{}, ctx.Err()
		case event, ok := <-sub.C():
			if !ok {
				continue
			}
			if !eventMatchesTurn(event, threadID, turnID) {
				continue
			}
			if event.Method != protocol.MethodTurnCompleted && event.Method != protocol.MethodItemCompleted && event.Method != protocol.MethodItemStarted {
				continue
			}
			if turn, done, err := check(); err == nil {
				transientCount = 0
				if done {
					return turn, nil
				}
			} else if errors.Is(err, errTurnNotFound) {
				transientCount = 0
			} else if isTransientError(err) {
				transientCount++
				if transientCount >= maxTransientRetries {
					return Turn{}, err
				}
			} else {
				return Turn{}, err
			}
		case <-ticker.C:
		}
	}
}

func (c *Client) readTurn(ctx context.Context, threadID, turnID string) (Turn, error) {
	thread, err := c.ThreadRead(ctx, ThreadReadRequest{
		ThreadID:     threadID,
		IncludeTurns: true,
	})
	if err != nil {
		// Before the first user message is processed the binary rejects thread/read
		// with "not materialized yet". Treat this as "not found yet" so WaitForTurn
		// waits rather than surfacing the error immediately.
		if strings.Contains(err.Error(), "not materialized") {
			return Turn{}, errTurnNotFound
		}
		return Turn{}, fmt.Errorf("thread/read: %w", err)
	}
	for i := len(thread.Turns) - 1; i >= 0; i-- {
		if thread.Turns[i].ID == turnID {
			return thread.Turns[i], nil
		}
	}
	return Turn{}, errTurnNotFound
}

func eventMatchesTurn(event Event, threadID, turnID string) bool {
	switch v := event.Value.(type) {
	case TurnStartedEvent:
		return v.ThreadID == threadID && turnEventID(v.TurnID, v.Turn) == turnID
	case TurnCompletedEvent:
		return v.ThreadID == threadID && turnEventID(v.TurnID, v.Turn) == turnID
	case ItemStartedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemCompletedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemAgentMessageDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemPlanDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemReasoningSummaryTextDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemReasoningSummaryPartAddedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemReasoningTextDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemCommandExecutionOutputDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemFileChangePatchUpdatedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemFileChangeOutputDeltaEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemAutoApprovalReviewStartedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ItemAutoApprovalReviewCompletedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ThreadStartedEvent:
		return v.ThreadID == threadID
	case ThreadArchivedEvent:
		return v.ThreadID == threadID
	case ThreadUnarchivedEvent:
		return v.ThreadID == threadID
	case ThreadClosedEvent:
		return v.ThreadID == threadID
	case ThreadStatusChangedEvent:
		return v.ThreadID == threadID
	case ThreadGoalUpdatedEvent:
		return v.ThreadID == threadID
	case ThreadGoalClearedEvent:
		return v.ThreadID == threadID
	case ItemUpdatedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ServerRequestResolvedEvent:
		return v.ThreadID == threadID
	case ThreadTokenUsageUpdatedEvent:
		return v.ThreadID == threadID
	case TurnDiffUpdatedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case TurnPlanUpdatedEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case ErrorEvent:
		return v.ThreadID == threadID && v.TurnID == turnID
	case RawNotificationEvent:
		return (v.ThreadID == "" || v.ThreadID == threadID) && (v.TurnID == "" || v.TurnID == turnID)
	default:
		return false
	}
}

func turnEventID(turnID string, turn *Turn) string {
	if strings.TrimSpace(turnID) != "" {
		return turnID
	}
	if turn != nil {
		return turn.ID
	}
	return ""
}

func isTerminalTurnStatus(status protocol.TurnStatus) bool {
	switch status {
	case protocol.TurnStatusCompleted, protocol.TurnStatusFailed, protocol.TurnStatusInterrupted:
		return true
	default:
		return false
	}
}

var errTurnNotFound = errors.New("turn not found")

func closedEventSubscription() *EventSubscription {
	ch := make(chan Event)
	close(ch)
	return &EventSubscription{ch: ch}
}
