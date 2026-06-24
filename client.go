package codexgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zealbase/codex-app-server-go/internal/process"
	"github.com/zealbase/codex-app-server-go/internal/protocol"
	"github.com/zealbase/codex-app-server-go/internal/transport"
)

// modelCatalogTTL is how long a successfully-fetched model catalog is cached.
// After expiry the next call to loadModelCatalog re-fetches from the server.
const modelCatalogTTL = 5 * time.Minute

type Client struct {
	transport Transport
	events    *eventBroker

	modelMu         sync.Mutex
	modelCatalog    map[string]struct{}
	modelCatalogErr error
	modelCatalogAt  time.Time

	threadSem chan struct{} // nil means unlimited

	proc *process.ManagedProcess
}

func New(opts ...Option) (*Client, error) {
	cfg := clientConfig{}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	// Build the HTTP/WS transport now that all options are processed, so the
	// retry policy applies regardless of where WithRetry sits in the list.
	if cfg.innerTransport != nil {
		inner := cfg.innerTransport
		if cfg.retry != nil {
			inner = transport.NewRetryTransport(inner, *cfg.retry)
		}
		cfg.transport = newChannelTransport(inner)
	}

	var proc *process.ManagedProcess
	if cfg.processPath != "" {
		p, stdin, stdout, err := process.Start(context.Background(), cfg.processPath, cfg.processArgs, cfg.processEnv, cfg.processDir)
		if err != nil {
			return nil, err
		}
		proc = p
		cfg.transport = NewStdioTransport(stdout, stdin)
	}

	if cfg.transport == nil {
		cfg.transport = NewStdioTransport(os.Stdin, os.Stdout)
	}

	if cfg.requestHandler != nil {
		cfg.transport.SetRequestHandler(cfg.requestHandler)
	}

	client := &Client{transport: cfg.transport, proc: proc}
	if source, ok := cfg.transport.(notificationSource); ok {
		client.events = newEventBroker()
		go client.notificationLoop(source.Notifications())
	}

	maxThreads := cfg.maxThreads
	if maxThreads == 0 {
		maxThreads = 64 // default
	}
	// maxThreads == -1 means unlimited; threadSem stays nil.
	if maxThreads > 0 {
		client.threadSem = make(chan struct{}, maxThreads)
	}

	if cfg.autoInit {
		initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer initCancel()
		initReq := InitializeRequest{
			ClientInfo: ClientInfo{Name: "codex-go-sdk", Version: "0.1.0"},
			Capabilities: Capabilities{
				ExperimentalAPI: true,
			},
		}
		if _, err := client.Initialize(initCtx, initReq); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("codex initialize: %w", err)
		}
		if cfg.autoLoginAPIKey != "" {
			if err := client.LoginAPIKey(initCtx, cfg.autoLoginAPIKey); err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("codex auto-login: %w", err)
			}
		}
	}

	return client, nil
}

// Ping verifies the transport is responsive by listing threads with a limit of 1.
// Returns nil if the server responds within ctx's deadline, or an error otherwise.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.ThreadList(ctx, ThreadListRequest{Limit: 1})
	return err
}

// ModelInfo describes a single model entry from the server's model/list RPC.
type ModelInfo struct {
	ID          string `json:"id"`
	Model       string `json:"model"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Hidden      bool   `json:"hidden"`
	IsDefault   bool   `json:"isDefault"`
}

// modelListRequest is the payload for the model/list RPC.
type modelListRequest struct {
	IncludeHidden bool `json:"includeHidden"`
}

// modelListResponse is the server's model/list reply.
type modelListResponse struct {
	Data []ModelInfo `json:"data"`
}

// ModelList returns the full model catalog from the server. When includeHidden
// is true, hidden models are included. Results are not cached.
func (c *Client) ModelList(ctx context.Context, includeHidden bool) ([]ModelInfo, error) {
	var resp modelListResponse
	req := modelListRequest{IncludeHidden: includeHidden}
	if err := c.transport.Call(ctx, protocol.MethodModelList, req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Models returns the sorted list of model IDs supported by the connected server.
// It is a convenience wrapper over ModelList that excludes hidden models and
// returns IDs only.
func (c *Client) Models(ctx context.Context) ([]string, error) {
	infos, err := c.ModelList(ctx, false)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(infos))
	for _, m := range infos {
		id := m.ID
		if id == "" {
			id = m.Model
		}
		if id != "" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	return models, nil
}

// defaultDaemonSocketPath returns the standard codex app-server daemon control socket path.
func defaultDaemonSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "app-server-control", "app-server-control.sock")
}

// FindBinary returns the path to the codex binary, searching in order:
//  1. CODEX_BIN env var (nharness convention)
//  2. CODEX_CLI_PATH env var (official Python SDK convention)
//  3. exec.LookPath("codex") in the system PATH
func FindBinary() (string, error) {
	for _, envVar := range []string{"CODEX_BIN", "CODEX_CLI_PATH"} {
		if p := os.Getenv(envVar); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return exec.LookPath("codex")
}

func (c *Client) Close() error {
	if c == nil || c.transport == nil {
		return nil
	}
	if c.events != nil {
		c.events.close()
	}
	err := c.transport.Close()
	if c.proc != nil {
		_ = c.proc.Shutdown(context.Background())
	}
	return err
}

func (c *Client) Initialize(ctx context.Context, req InitializeRequest) (InitializeResult, error) {
	var result InitializeResult
	if err := c.transport.Call(ctx, protocol.MethodInitialize, req, &result); err != nil {
		return InitializeResult{}, err
	}
	if err := c.transport.Notify(ctx, protocol.MethodInitialized, nil); err != nil {
		return InitializeResult{}, err
	}
	return result, nil
}

func (c *Client) ThreadStart(ctx context.Context, req ThreadStartRequest) (Thread, error) {
	var resp struct {
		Thread Thread `json:"thread"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadStart, req, &resp); err != nil {
		return Thread{}, err
	}
	return resp.Thread, nil
}

func (c *Client) ThreadResume(ctx context.Context, req ThreadResumeRequest) (Thread, error) {
	var resp struct {
		Thread Thread `json:"thread"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadResume, req, &resp); err != nil {
		return Thread{}, err
	}
	return resp.Thread, nil
}

func (c *Client) ThreadRead(ctx context.Context, req ThreadReadRequest) (Thread, error) {
	var resp struct {
		Thread Thread `json:"thread"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadRead, req, &resp); err != nil {
		return Thread{}, err
	}
	return resp.Thread, nil
}

func (c *Client) ThreadFork(ctx context.Context, req ThreadForkRequest) (Thread, error) {
	var resp struct {
		Thread Thread `json:"thread"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadFork, req, &resp); err != nil {
		return Thread{}, err
	}
	return resp.Thread, nil
}

func (c *Client) ThreadList(ctx context.Context, req ThreadListRequest) ([]Thread, error) {
	// The binary returns {"data":[...]} for persistent (on-disk) threads.
	var resp struct {
		Data []Thread `json:"data"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadList, req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ThreadLoadedList returns the IDs of threads currently loaded in the server's
// memory. Newly started threads appear here before they are persisted to disk.
func (c *Client) ThreadLoadedList(ctx context.Context) ([]string, error) {
	var resp struct {
		Data []string `json:"data"`
	}
	if err := c.transport.Call(ctx, protocol.MethodThreadLoadedList, map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) ThreadArchive(ctx context.Context, req ThreadArchiveRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadArchive, req, nil)
}

func (c *Client) ThreadUnarchive(ctx context.Context, req ThreadUnarchiveRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadUnarchive, req, nil)
}

func (c *Client) ThreadSetName(ctx context.Context, req ThreadSetNameRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadSetName, req, nil)
}

func (c *Client) ThreadRollback(ctx context.Context, req ThreadRollbackRequest) error {
	return c.transport.Call(ctx, protocol.MethodThreadRollback, req, nil)
}

func (c *Client) TurnSteer(ctx context.Context, req TurnSteerRequest) error {
	return c.transport.Call(ctx, protocol.MethodTurnSteer, req, nil)
}

func (c *Client) ReviewStart(ctx context.Context, req ReviewStartRequest) error {
	return c.transport.Call(ctx, protocol.MethodReviewStart, req, nil)
}

func (c *Client) TurnStart(ctx context.Context, req TurnStartRequest) (Turn, error) {
	var resp struct {
		Turn Turn `json:"turn"`
	}
	if err := c.transport.Call(ctx, protocol.MethodTurnStart, req, &resp); err != nil {
		return Turn{}, err
	}
	return resp.Turn, nil
}

func (c *Client) TurnInterrupt(ctx context.Context, req TurnInterruptRequest) error {
	return c.transport.Call(ctx, protocol.MethodTurnInterrupt, req, nil)
}

// TurnRead returns the current state of a turn by ID. For a completed turn it
// includes items and usage; use WaitForFinalAgentMessage to wait until
// completion when the turn may still be in progress.
func (c *Client) TurnRead(ctx context.Context, threadID, turnID string) (Turn, error) {
	return c.readTurn(ctx, threadID, turnID)
}

func (c *Client) TurnDiff(ctx context.Context, req TurnDiffRequest) (TurnDiffResult, error) {
	var result TurnDiffResult
	if err := c.transport.Call(ctx, protocol.MethodTurnDiff, req, &result); err != nil {
		return TurnDiffResult{}, err
	}
	return result, nil
}

// StartThread creates a new session thread on the server and returns a
// stateful SessionThread that can be used to run turns.
func (c *Client) StartThread(ctx context.Context, opts ...ThreadOption) (*SessionThread, error) {
	release, err := c.acquireThreadSlot(ctx)
	if err != nil {
		return nil, err
	}
	cfg := applyThreadOptions(opts)
	thread, err := c.ThreadStart(ctx, cfg.req)
	if err != nil {
		release()
		return nil, err
	}
	st := &SessionThread{client: c, threadID: thread.ID, release: release}
	if cfg.initialInput != "" {
		if _, err := st.Run(ctx, cfg.initialInput); err != nil {
			st.Close()
			return nil, err
		}
	}
	return st, nil
}

// ResumeThread reconnects to an existing thread by ID and returns a stateful
// SessionThread.
func (c *Client) ResumeThread(ctx context.Context, threadID string, opts ...ThreadOption) (*SessionThread, error) {
	release, err := c.acquireThreadSlot(ctx)
	if err != nil {
		return nil, err
	}
	cfg := applyThreadOptions(opts)
	_, err = c.ThreadResume(ctx, ThreadResumeRequest{ThreadID: threadID})
	if err != nil {
		release()
		return nil, err
	}
	st := &SessionThread{client: c, threadID: threadID, release: release}
	if cfg.initialInput != "" {
		if _, err := st.Run(ctx, cfg.initialInput); err != nil {
			st.Close()
			return nil, err
		}
	}
	return st, nil
}

// acquireThreadSlot blocks until a semaphore slot is available (or ctx is done).
// Returns a release function that must be called when the slot is no longer needed.
func (c *Client) acquireThreadSlot(ctx context.Context) (func(), error) {
	if c.threadSem == nil {
		return func() {}, nil
	}
	select {
	case c.threadSem <- struct{}{}:
		return func() {
			select {
			case <-c.threadSem:
			default:
			}
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SetModel sends a config/update request that changes the active model.
func (c *Client) SetModel(ctx context.Context, model string) error {
	return c.transport.Call(ctx, protocol.MethodConfigUpdate, configUpdateRequest{Model: model}, nil)
}

// SetApprovalPolicy sends a config/update request that changes the approval policy.
func (c *Client) SetApprovalPolicy(ctx context.Context, policy string) error {
	return c.transport.Call(ctx, protocol.MethodConfigUpdate, configUpdateRequest{ApprovalPolicy: policy}, nil)
}

// SetSandbox sends a config/update request that changes the sandbox policy.
func (c *Client) SetSandbox(ctx context.Context, sandbox string) error {
	return c.transport.Call(ctx, protocol.MethodConfigUpdate, configUpdateRequest{SandboxPolicy: sandbox}, nil)
}

func NewStdioTransport(stdin io.ReadCloser, stdout io.WriteCloser) Transport {
	return newChannelTransport(transport.NewStdio(transport.Combine(stdin, stdout)))
}

// Call makes a raw JSON-RPC call and decodes the result into result.
// Exposed for integration tests that need to probe the binary's raw response format.
func (c *Client) Call(ctx context.Context, method string, params, result any) error {
	return c.transport.Call(ctx, method, params, result)
}

func (c *Client) validateModel(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	if c == nil || c.transport == nil {
		return nil
	}
	models, err := c.loadModelCatalog(ctx)
	if err != nil {
		// Catalog fetch failed; skip client-side validation and let server decide.
		return nil
	}
	if len(models) == 0 {
		// Empty catalog (network unavailable); pass through to server.
		return nil
	}
	if _, ok := models[model]; ok {
		return nil
	}
	return fmt.Errorf("unsupported model %q: not present in server model catalog", model)
}

// loadModelCatalog fetches (and caches with TTL) the server's model list.
// Unlike sync.Once, a failed fetch is retried on the next call so that transient
// network errors do not permanently disable client-side model validation.
func (c *Client) loadModelCatalog(ctx context.Context) (map[string]struct{}, error) {
	c.modelMu.Lock()
	defer c.modelMu.Unlock()

	now := time.Now()
	// Return cached catalog if it was fetched successfully and is still fresh.
	if c.modelCatalog != nil && now.Sub(c.modelCatalogAt) < modelCatalogTTL {
		return c.modelCatalog, nil
	}

	var raw json.RawMessage
	if err := c.transport.Call(ctx, protocol.MethodModelList, struct{}{}, &raw); err != nil {
		c.modelCatalogErr = err
		return nil, err
	}
	models := make(map[string]struct{})
	if err := collectStringsFromJSON(raw, models); err != nil {
		c.modelCatalogErr = err
		return nil, err
	}
	c.modelCatalog = models
	c.modelCatalogErr = nil
	c.modelCatalogAt = now
	return c.modelCatalog, nil
}

func collectStringsFromJSON(data []byte, out map[string]struct{}) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	collectStringValues(v, out)
	return nil
}

func collectStringValues(v any, out map[string]struct{}) {
	switch x := v.(type) {
	case string:
		out[x] = struct{}{}
	case []any:
		for _, elem := range x {
			collectStringValues(elem, out)
		}
	case map[string]any:
		for _, elem := range x {
			collectStringValues(elem, out)
		}
	}
}

type stdioTransport struct {
	inner     transport.Transport
	handlerMu sync.RWMutex
	handler   RequestHandler
	closed    chan struct{}
	readyCh   chan struct{}
}

// newChannelTransport adapts any channel-based internal transport.Transport to
// the public, handler-based Transport interface, running a request loop that
// routes server-initiated requests to the configured RequestHandler.
func newChannelTransport(inner transport.Transport) *stdioTransport {
	t := &stdioTransport{
		inner:   inner,
		closed:  make(chan struct{}),
		readyCh: make(chan struct{}),
	}
	close(t.readyCh)
	go t.requestLoop()
	return t
}

func (s *stdioTransport) Call(ctx context.Context, method string, params any, result any) error {
	return s.inner.Call(ctx, method, params, result)
}

func (s *stdioTransport) Notify(ctx context.Context, method string, params any) error {
	return s.inner.Notify(ctx, method, params)
}

func (s *stdioTransport) Notifications() <-chan transport.Notification {
	return s.inner.Notifications()
}

func (s *stdioTransport) SetRequestHandler(handler RequestHandler) {
	s.handlerMu.Lock()
	s.handler = handler
	s.handlerMu.Unlock()
}

func (s *stdioTransport) Close() error {
	select {
	case <-s.closed:
		return nil
	default:
		close(s.closed)
		return s.inner.Close()
	}
}

func (s *stdioTransport) requestLoop() {
	<-s.readyCh
	for {
		select {
		case req, ok := <-s.inner.Requests():
			if !ok {
				return
			}
			s.handlerMu.RLock()
			handler := s.handler
			s.handlerMu.RUnlock()
			if handler == nil {
				_ = req.ReplyError(req.Context(), -32601, "request handler not configured", nil)
				continue
			}
			resp, err := handler.HandleServerRequest(req.Context(), ServerRequest{
				Method: req.Method(),
				Params: req.Params(),
			})
			if err != nil {
				_ = req.ReplyError(req.Context(), -32603, err.Error(), nil)
				continue
			}
			_ = req.Reply(req.Context(), json.RawMessage(resp.Result))
		case <-s.closed:
			return
		}
	}
}

type notificationSource interface {
	Notifications() <-chan transport.Notification
}

func (c *Client) notificationLoop(ch <-chan transport.Notification) {
	if c == nil || c.events == nil {
		return
	}
	for note := range ch {
		c.events.publish(decodeEvent(note.Method, note.Params))
	}
	c.events.close()
}
