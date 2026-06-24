package codexgo

import (
	"context"
	"encoding/json"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

// CommandExecTerminalSize is a PTY size in character cells.
type CommandExecTerminalSize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// CommandExecRequest starts an interactive command execution. Command is the
// argv slice. When ProcessID is empty the server assigns one (returned via the
// handle); pass an existing ProcessID to attach. SandboxPolicy is passed through
// as raw JSON.
type CommandExecRequest struct {
	Command            []string                 `json:"command"`
	CWD                string                   `json:"cwd,omitempty"`
	Env                map[string]string        `json:"env,omitempty"`
	TTY                bool                     `json:"tty,omitempty"`
	Size               *CommandExecTerminalSize `json:"size,omitempty"`
	StreamStdin        bool                     `json:"streamStdin,omitempty"`
	StreamStdoutStderr bool                     `json:"streamStdoutStderr,omitempty"`
	TimeoutMs          int                      `json:"timeoutMs,omitempty"`
	DisableTimeout     bool                     `json:"disableTimeout,omitempty"`
	OutputBytesCap     int                      `json:"outputBytesCap,omitempty"`
	DisableOutputCap   bool                     `json:"disableOutputCap,omitempty"`
	ProcessID          string                   `json:"processId,omitempty"`
	SandboxPolicy      json.RawMessage          `json:"sandboxPolicy,omitempty"`
}

// CommandExecResult is the terminal result of a (non-streaming) command exec.
type CommandExecResult struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CommandExecWriteRequest writes stdin bytes (base64) to a running process, or
// closes its stdin.
type CommandExecWriteRequest struct {
	ProcessID   string `json:"processId"`
	DeltaBase64 string `json:"deltaBase64,omitempty"`
	CloseStdin  bool   `json:"closeStdin,omitempty"`
}

// CommandExecResizeRequest resizes a running process's PTY.
type CommandExecResizeRequest struct {
	ProcessID string                  `json:"processId"`
	Size      CommandExecTerminalSize `json:"size"`
}

// CommandExecTerminateRequest terminates a running process.
type CommandExecTerminateRequest struct {
	ProcessID string `json:"processId"`
}

// CommandExec starts an interactive command. For streaming runs subscribe to
// CommandExecOutputDeltaEvent via Events() before calling, then drive the
// process with the returned handle.
func (c *Client) CommandExec(ctx context.Context, req CommandExecRequest) (CommandExecResult, error) {
	var resp CommandExecResult
	if err := c.transport.Call(ctx, protocol.MethodCommandExec, req, &resp); err != nil {
		return CommandExecResult{}, err
	}
	return resp, nil
}

// CommandExecWrite writes stdin to (or closes stdin of) a running process.
func (c *Client) CommandExecWrite(ctx context.Context, req CommandExecWriteRequest) error {
	return c.transport.Call(ctx, protocol.MethodCommandExecWrite, req, nil)
}

// CommandExecResize resizes a running process's PTY.
func (c *Client) CommandExecResize(ctx context.Context, req CommandExecResizeRequest) error {
	return c.transport.Call(ctx, protocol.MethodCommandExecResize, req, nil)
}

// CommandExecTerminate terminates a running process.
func (c *Client) CommandExecTerminate(ctx context.Context, req CommandExecTerminateRequest) error {
	return c.transport.Call(ctx, protocol.MethodCommandExecTerminate, req, nil)
}

// CommandExecHandle is a convenience wrapper binding a processId to the
// write/resize/terminate operations for an interactive command.
type CommandExecHandle struct {
	client    *Client
	processID string
}

// CommandExecHandle returns a handle for driving the process with the given ID.
func (c *Client) CommandExecHandle(processID string) *CommandExecHandle {
	return &CommandExecHandle{client: c, processID: processID}
}

// ProcessID returns the bound process ID.
func (h *CommandExecHandle) ProcessID() string { return h.processID }

// Write sends stdin bytes (base64-encoded) to the process.
func (h *CommandExecHandle) Write(ctx context.Context, deltaBase64 string) error {
	return h.client.CommandExecWrite(ctx, CommandExecWriteRequest{ProcessID: h.processID, DeltaBase64: deltaBase64})
}

// CloseStdin closes the process's stdin.
func (h *CommandExecHandle) CloseStdin(ctx context.Context) error {
	return h.client.CommandExecWrite(ctx, CommandExecWriteRequest{ProcessID: h.processID, CloseStdin: true})
}

// Resize resizes the process's PTY.
func (h *CommandExecHandle) Resize(ctx context.Context, cols, rows int) error {
	return h.client.CommandExecResize(ctx, CommandExecResizeRequest{
		ProcessID: h.processID,
		Size:      CommandExecTerminalSize{Cols: cols, Rows: rows},
	})
}

// Terminate terminates the process.
func (h *CommandExecHandle) Terminate(ctx context.Context) error {
	return h.client.CommandExecTerminate(ctx, CommandExecTerminateRequest{ProcessID: h.processID})
}
