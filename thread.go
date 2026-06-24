package codexgo

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
	schematypes "github.com/zealbase/codex-app-server-go/internal/protocol/schema"
)

// threadConfig holds the resolved configuration for a new or resumed thread.
type threadConfig struct {
	req ThreadStartRequest
	// initialInput, when non-empty, is run as the first turn immediately after
	// the thread starts (or resumes).
	initialInput string
}

// ThreadOption is a functional option for configuring a new or resumed thread.
type ThreadOption func(*threadConfig)

// WithThreadModel sets the model when starting a thread.
func WithThreadModel(model string) ThreadOption {
	return func(c *threadConfig) {
		c.req.Model = model
	}
}

// WithThreadApprovalPolicy sets the approval policy when starting a thread.
func WithThreadApprovalPolicy(policy string) ThreadOption {
	return func(c *threadConfig) {
		c.req.ApprovalPolicy = policy
	}
}

// WithThreadApprovalMode sets a typed approval policy when starting a thread.
func WithThreadApprovalMode(mode ApprovalMode) ThreadOption {
	return func(c *threadConfig) {
		c.req.ApprovalPolicy = string(mode)
	}
}

// WithThreadCWD sets the working directory when starting a thread.
func WithThreadCWD(cwd string) ThreadOption {
	return func(c *threadConfig) {
		c.req.CWD = cwd
	}
}

// WithThreadEphemeral marks the thread as ephemeral.
func WithThreadEphemeral(ephemeral bool) ThreadOption {
	return func(c *threadConfig) {
		c.req.Ephemeral = ephemeral
	}
}

// WithInitialInput sets a prompt to run immediately after the thread starts.
// If set, StartThread / ResumeThread return only after the first turn completes.
func WithInitialInput(input string) ThreadOption {
	return func(c *threadConfig) {
		c.initialInput = input
	}
}

// applyThreadOptions applies all ThreadOption values and returns the config.
func applyThreadOptions(opts []ThreadOption) threadConfig {
	var cfg threadConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// SessionThread represents a live, stateful session on the Codex app-server.
// Use Client.StartThread / Client.ResumeThread to obtain one.
type SessionThread struct {
	client    *Client
	threadID  string
	turnMu    sync.Mutex // prevents concurrent turns on the same thread
	stdinMu   sync.Mutex // serialises writes to the transport
	release   func()     // returns the semaphore slot; called once by Close
	closeOnce sync.Once
}

// Close releases the semaphore slot held by this SessionThread, allowing a
// new thread to be started if the client is at its maxThreads limit.
// Close is idempotent and safe to call from multiple goroutines.
func (t *SessionThread) Close() {
	t.closeOnce.Do(func() {
		if t.release != nil {
			t.release()
		}
	})
}

// ID returns the thread's persistent ID.
func (t *SessionThread) ID() string {
	return t.threadID
}

// Run executes a prompt synchronously and returns the completed TurnResult.
// It blocks until the turn reaches a terminal state (completed/failed/interrupted)
// or ctx is cancelled.
//
// The subscription to server events is established before TurnStart is called so
// that item/completed and turn/completed notifications are never missed, even on
// fast (mock) servers where events fire before TurnStart returns.
func (t *SessionThread) Run(ctx context.Context, input string, opts ...TurnOption) (*TurnResult, error) {
	t.turnMu.Lock()
	defer t.turnMu.Unlock()

	req := TurnStartRequest{ThreadID: t.threadID, Input: input}
	applyTurnOptions(&req, opts)

	// Subscribe before TurnStart to capture all events for this turn.
	sub := t.client.Events()
	defer sub.Close()

	turn, err := t.client.TurnStart(ctx, req)
	if err != nil {
		return nil, err
	}

	completed, collector, err := t.client.waitForTurnOutputSub(ctx, t.threadID, turn.ID, sub)
	if err != nil {
		return nil, err
	}

	items := completed.Items
	if len(items) == 0 {
		items = collector.items
	}

	usage := completed.Usage
	if usage == nil {
		usage = collector.eventUsage
	}

	result := &TurnResult{
		Turn:  completed,
		Items: items,
		Usage: usage,
	}
	if len(completed.Error) > 0 && string(completed.Error) != "null" {
		var turnErr TurnError
		if err := json.Unmarshal(completed.Error, &turnErr); err == nil {
			result.Error = &turnErr
		}
	}
	// If items carry no extractable agent text, fall back to streaming deltas.
	if result.FinalAgentText() == "" {
		result.DeltaText = collector.deltaText()
	}
	return result, nil
}

// RunStreamed executes a prompt and streams events over the returned channel.
// The channel is closed when the turn completes or errors.
// The caller must drain the channel to avoid blocking the event broker.
//
// Concurrency: RunStreamed acquires an internal turn mutex that prevents two
// turns from running concurrently on the same SessionThread. The mutex is held
// for the entire stream duration and is released only when the goroutine exits —
// either because a terminal event (turn/completed or error) was received, or
// because ctx was cancelled. Callers MUST cancel the context (or fully drain
// the channel until it is closed) to ensure the lock is released if they stop
// consuming events before the stream ends.
func (t *SessionThread) RunStreamed(ctx context.Context, input string, opts ...TurnOption) (<-chan ThreadEvent, error) {
	t.turnMu.Lock()

	req := TurnStartRequest{ThreadID: t.threadID, Input: input}
	applyTurnOptions(&req, opts)

	turn, err := t.client.TurnStart(ctx, req)
	if err != nil {
		t.turnMu.Unlock()
		return nil, err
	}

	out := make(chan ThreadEvent, 128)
	sub := t.client.Events()

	go func() {
		defer t.turnMu.Unlock()
		defer close(out)
		defer sub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sub.C():
				if !ok {
					return
				}
				if !eventMatchesTurn(event, t.threadID, turn.ID) {
					continue
				}
				te := ThreadEvent{Kind: event.Method, Raw: event.Value}
				select {
				case out <- te:
				case <-ctx.Done():
					return
				}
				// Stop streaming when the turn enters a terminal state.
				if event.Method == protocol.MethodTurnCompleted {
					return
				}
				if event.Method == protocol.MethodError {
					return
				}
			}
		}
	}()

	return out, nil
}

// Interrupt sends a turn/interrupt for the given turnID.
func (t *SessionThread) Interrupt(ctx context.Context, turnID string) error {
	return t.client.TurnInterrupt(ctx, TurnInterruptRequest{
		ThreadID: t.threadID,
		TurnID:   turnID,
	})
}

// Compact sends a thread/compact request to trigger context compaction.
func (t *SessionThread) Compact(ctx context.Context) error {
	t.stdinMu.Lock()
	defer t.stdinMu.Unlock()
	req := threadCompactRequest{ThreadID: t.threadID}
	return t.client.transport.Call(ctx, protocol.MethodThreadCompact, req, nil)
}

// Steer sends additional input to an in-progress turn (mid-turn steering).
// Only valid while a turn is active. Does not require the turn mutex because
// steering is sent to an already-running turn, not starting a new one.
func (t *SessionThread) Steer(ctx context.Context, turnID, input string) error {
	return t.client.TurnSteer(ctx, TurnSteerRequest{
		ThreadID: t.threadID,
		TurnID:   turnID,
		Input:    input,
	})
}

// SteerInputs sends typed multi-part input to an in-progress turn, mirroring
// Steer but accepting TurnInput items instead of a plain string.
func (t *SessionThread) SteerInputs(ctx context.Context, turnID string, inputs ...TurnInput) error {
	return t.client.TurnSteer(ctx, TurnSteerRequest{
		ThreadID: t.threadID,
		TurnID:   turnID,
		Input:    encodeInputs(inputs),
	})
}

// Fork creates a new thread branching from this thread at the given turnID.
// If turnID is empty the server forks from the latest turn.
func (t *SessionThread) Fork(ctx context.Context, turnID string, opts ...ThreadOption) (*SessionThread, error) {
	release, err := t.client.acquireThreadSlot(ctx)
	if err != nil {
		return nil, err
	}
	thread, err := t.client.ThreadFork(ctx, ThreadForkRequest{
		ThreadID: t.threadID,
		TurnID:   turnID,
	})
	if err != nil {
		release()
		return nil, err
	}
	_ = opts // reserved for future use
	return &SessionThread{client: t.client, threadID: thread.ID, release: release}, nil
}

// Archive marks this thread as archived.
func (t *SessionThread) Archive(ctx context.Context) error {
	return t.client.ThreadArchive(ctx, ThreadArchiveRequest{ThreadID: t.threadID})
}

// Unarchive restores this thread from archived state.
func (t *SessionThread) Unarchive(ctx context.Context) error {
	return t.client.ThreadUnarchive(ctx, ThreadUnarchiveRequest{ThreadID: t.threadID})
}

// SetName sets the display name for this thread.
func (t *SessionThread) SetName(ctx context.Context, name string) error {
	return t.client.ThreadSetName(ctx, ThreadSetNameRequest{ThreadID: t.threadID, Name: name})
}

// SetGoal creates or updates the persisted goal (objective) for this thread.
func (t *SessionThread) SetGoal(ctx context.Context, goal string) error {
	_, err := t.client.ThreadGoalSet(ctx, ThreadGoalSetRequest{ThreadID: t.threadID, Objective: goal})
	return err
}

// ClearGoal removes the persisted goal for this thread.
func (t *SessionThread) ClearGoal(ctx context.Context) error {
	return t.client.ThreadGoalClear(ctx, ThreadGoalClearRequest{ThreadID: t.threadID})
}

// GetGoal returns the persisted goal for this thread, or a zero ThreadGoal when
// none is set.
func (t *SessionThread) GetGoal(ctx context.Context) (ThreadGoal, error) {
	return t.client.ThreadGoalGet(ctx, ThreadGoalGetRequest{ThreadID: t.threadID})
}

// Rollback removes the specified turns from this thread's history.
func (t *SessionThread) Rollback(ctx context.Context, turnIDs []string) error {
	return t.client.ThreadRollback(ctx, ThreadRollbackRequest{ThreadID: t.threadID, TurnIDs: turnIDs})
}

// GitDiff requests the diff for the latest (or specified) turn on this thread.
// Returns the unified diff string.
func (t *SessionThread) GitDiff(ctx context.Context, turnID string) (string, error) {
	result, err := t.client.TurnDiff(ctx, TurnDiffRequest{
		ThreadID: t.threadID,
		TurnID:   turnID,
	})
	if err != nil {
		return "", err
	}
	return result.Diff, nil
}

// --- thread/metadata/update ---

// ThreadMetadataGitInfo patches stored Git metadata for a thread. A nil field is
// left unchanged; set a pointer to an empty string to clear it.
type ThreadMetadataGitInfo struct {
	Branch    *string `json:"branch,omitempty"`
	OriginURL *string `json:"originUrl,omitempty"`
	Sha       *string `json:"sha,omitempty"`
}

// ThreadMetadataUpdateRequest patches a thread's stored metadata.
type ThreadMetadataUpdateRequest struct {
	ThreadID string                 `json:"threadId"`
	GitInfo  *ThreadMetadataGitInfo `json:"gitInfo,omitempty"`
}

// ThreadMetadataUpdateResult holds the updated thread.
type ThreadMetadataUpdateResult struct {
	Thread Thread `json:"thread"`
}

// ThreadMetadataUpdate patches a thread's stored metadata (e.g. Git info).
func (c *Client) ThreadMetadataUpdate(ctx context.Context, req ThreadMetadataUpdateRequest) (ThreadMetadataUpdateResult, error) {
	var resp ThreadMetadataUpdateResult
	if err := c.transport.Call(ctx, protocol.MethodThreadMetadataUpdate, req, &resp); err != nil {
		return ThreadMetadataUpdateResult{}, err
	}
	return resp, nil
}

// --- thread/unsubscribe ---

// ThreadUnsubscribeStatus reports the outcome of an unsubscribe request.
type ThreadUnsubscribeStatus string

const (
	ThreadUnsubscribeStatusNotLoaded     ThreadUnsubscribeStatus = "notLoaded"
	ThreadUnsubscribeStatusNotSubscribed ThreadUnsubscribeStatus = "notSubscribed"
	ThreadUnsubscribeStatusUnsubscribed  ThreadUnsubscribeStatus = "unsubscribed"
)

// ThreadUnsubscribeRequest stops event delivery for a thread on this connection.
type ThreadUnsubscribeRequest struct {
	ThreadID string `json:"threadId"`
}

// ThreadUnsubscribeResult reports the unsubscribe status.
type ThreadUnsubscribeResult struct {
	Status ThreadUnsubscribeStatus `json:"status"`
}

// ThreadUnsubscribe stops event delivery for a thread on this connection.
func (c *Client) ThreadUnsubscribe(ctx context.Context, req ThreadUnsubscribeRequest) (ThreadUnsubscribeResult, error) {
	var resp ThreadUnsubscribeResult
	if err := c.transport.Call(ctx, protocol.MethodThreadUnsubscribe, req, &resp); err != nil {
		return ThreadUnsubscribeResult{}, err
	}
	return resp, nil
}

// --- thread/delete ---

// ThreadDeleteRequest deletes a thread and its rollout history.
type ThreadDeleteRequest struct {
	ThreadID string `json:"threadId"`
}

// ThreadDelete deletes a thread and its rollout history.
func (c *Client) ThreadDelete(ctx context.Context, req ThreadDeleteRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadDelete, req, nil)
}

// --- thread/inject_items ---

// ThreadInjectItemsRequest injects raw items into a thread's history. Each item
// is the raw JSON of a thread item as defined by the protocol.
type ThreadInjectItemsRequest struct {
	ThreadID string            `json:"threadId"`
	Items    []json.RawMessage `json:"items"`
}

// ThreadInjectItems injects items into a thread's history.
func (c *Client) ThreadInjectItems(ctx context.Context, req ThreadInjectItemsRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadInjectItems, req, nil)
}

// --- thread/shellCommand ---

// ThreadShellCommandRequest runs a shell command string in the thread's shell.
// Unlike CommandExec this preserves shell syntax (pipes, redirects, quoting) and
// runs unsandboxed with full access.
type ThreadShellCommandRequest struct {
	ThreadID string `json:"threadId"`
	Command  string `json:"command"`
}

// ThreadShellCommand runs a shell command string in the thread's shell.
func (c *Client) ThreadShellCommand(ctx context.Context, req ThreadShellCommandRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadShellCommand, req, nil)
}

// --- thread/approveGuardianDeniedAction ---

// ThreadApproveGuardianDeniedActionRequest approves an action the guardian
// reviewer denied. Event is the raw guardian event payload.
type ThreadApproveGuardianDeniedActionRequest struct {
	ThreadID string          `json:"threadId"`
	Event    json.RawMessage `json:"event"`
}

// ThreadApproveGuardianDeniedAction approves a guardian-denied action.
func (c *Client) ThreadApproveGuardianDeniedAction(ctx context.Context, req ThreadApproveGuardianDeniedActionRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadApproveGuardianDenied, req, nil)
}

type (
	ThreadGoal             = schematypes.ThreadGoal
	ThreadGoalStatus       = schematypes.ThreadGoalStatus
	ThreadGoalSetRequest   = schematypes.ThreadGoalSetParams
	ThreadGoalSetResult    = schematypes.ThreadGoalSetResponse
	ThreadGoalGetRequest   = schematypes.ThreadGoalGetParams
	ThreadGoalClearRequest = schematypes.ThreadGoalClearParams
)

const (
	ThreadGoalStatusActive        = schematypes.ThreadGoalStatusActive
	ThreadGoalStatusPaused        = schematypes.ThreadGoalStatusPaused
	ThreadGoalStatusBlocked       = schematypes.ThreadGoalStatusBlocked
	ThreadGoalStatusUsageLimited  = schematypes.ThreadGoalStatusUsageLimited
	ThreadGoalStatusBudgetLimited = schematypes.ThreadGoalStatusBudgetLimited
	ThreadGoalStatusComplete      = schematypes.ThreadGoalStatusComplete
)

// ThreadGoalSet creates or updates the persisted goal for a thread.
func (c *Client) ThreadGoalSet(ctx context.Context, req ThreadGoalSetRequest) (ThreadGoal, error) {
	var resp ThreadGoalSetResult
	if err := c.transport.Call(ctx, protocol.MethodThreadGoalSet, req, &resp); err != nil {
		return ThreadGoal{}, err
	}
	return resp.Goal, nil
}

// ThreadGoalClear removes the persisted goal for a thread.
func (c *Client) ThreadGoalClear(ctx context.Context, req ThreadGoalClearRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadGoalClear, req, nil)
}

// ThreadGoalGet returns the persisted goal for a thread, or a zero ThreadGoal
// when none is set.
func (c *Client) ThreadGoalGet(ctx context.Context, req ThreadGoalGetRequest) (ThreadGoal, error) {
	var resp schematypes.ThreadGoalGetResponse
	if err := c.transport.Call(ctx, protocol.MethodThreadGoalGet, req, &resp); err != nil {
		return ThreadGoal{}, err
	}
	if resp.Goal == nil {
		return ThreadGoal{}, nil
	}
	return *resp.Goal, nil
}
