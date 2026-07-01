package codexgo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

type (
	ApprovalDecision                = protocol.ApprovalDecision
	CommandExecutionApprovalRequest = protocol.CommandExecutionApprovalRequest
	CommandExecutionApprovalResult  = protocol.CommandExecutionApprovalResponse
	FileChangeApprovalRequest       = protocol.FileChangeApprovalRequest
	FileChangeApprovalResult        = protocol.FileChangeApprovalResponse
	PermissionsApprovalRequest      = protocol.PermissionsApprovalRequest
	PermissionsApprovalResult       = protocol.PermissionsApprovalResponse
	UserInputQuestion               = protocol.UserInputQuestion
	UserInputOption                 = protocol.UserInputOption
	UserInputRequest                = protocol.UserInputRequest
	UserInputResult                 = protocol.UserInputResult
	ServerRequest                   = protocol.ServerRequest
	ServerResponse                  = protocol.ServerResponse
	MCPToolCallApprovalRequest      = protocol.MCPToolCallApprovalRequest
	MCPToolCallApprovalResponse     = protocol.MCPToolCallApprovalResponse
)

const (
	ApprovalDecisionAccept                        = "accept"
	ApprovalDecisionAcceptForSession              = "acceptForSession"
	ApprovalDecisionAcceptWithExecPolicyAmendment = "acceptWithExecpolicyAmendment"
	ApprovalDecisionApplyNetworkPolicyAmendment   = "applyNetworkPolicyAmendment"
	ApprovalDecisionDecline                       = "decline"
	ApprovalDecisionCancel                        = "cancel"

	FileChangeApprovalDecisionAccept           = "accept"
	FileChangeApprovalDecisionAcceptForSession = "acceptForSession"
	FileChangeApprovalDecisionDecline          = "decline"
	FileChangeApprovalDecisionCancel           = "cancel"
)

type ApprovalHandler interface {
	HandleCommandExecutionApproval(context.Context, CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error)
	HandleFileChangeApproval(context.Context, FileChangeApprovalRequest) (FileChangeApprovalResult, error)
	HandlePermissionsApproval(context.Context, PermissionsApprovalRequest) (PermissionsApprovalResult, error)
	HandleUserInputRequest(context.Context, UserInputRequest) (UserInputResult, error)
}

type RequestHandler interface {
	HandleServerRequest(context.Context, ServerRequest) (ServerResponse, error)
}

type RequestHandlerFunc func(context.Context, ServerRequest) (ServerResponse, error)

func (f RequestHandlerFunc) HandleServerRequest(ctx context.Context, req ServerRequest) (ServerResponse, error) {
	return f(ctx, req)
}

type approvalHandlerAdapter struct {
	handler ApprovalHandler
}

func (a *approvalHandlerAdapter) HandleServerRequest(ctx context.Context, req ServerRequest) (ServerResponse, error) {
	decoded, err := protocol.DecodeServerRequest(req.Method, req.Params)
	if err != nil {
		return ServerResponse{}, err
	}

	switch v := decoded.(type) {
	case CommandExecutionApprovalRequest:
		result, err := a.handler.HandleCommandExecutionApproval(ctx, v)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)
	case FileChangeApprovalRequest:
		result, err := a.handler.HandleFileChangeApproval(ctx, v)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)
	case PermissionsApprovalRequest:
		result, err := a.handler.HandlePermissionsApproval(ctx, v)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)
	case UserInputRequest:
		result, err := a.handler.HandleUserInputRequest(ctx, v)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)
	default:
		return ServerResponse{}, protocol.ErrUnsupportedServerRequest
	}
}

func serverResponseFrom(v any) (ServerResponse, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return ServerResponse{}, err
	}
	return ServerResponse{Result: data}, nil
}

// MCPApprovalHandler handles MCP tool call approval requests.
type MCPApprovalHandler interface {
	HandleMCPToolCallApproval(ctx context.Context, req MCPToolCallApprovalRequest) (MCPToolCallApprovalResponse, error)
}

// ExecApprovalHandler handles command-execution approval requests.
type ExecApprovalHandler interface {
	HandleCommandExecutionApproval(context.Context, CommandExecutionApprovalRequest) (CommandExecutionApprovalResult, error)
}

// FileApprovalHandler handles file-change approval requests.
type FileApprovalHandler interface {
	HandleFileChangeApproval(context.Context, FileChangeApprovalRequest) (FileChangeApprovalResult, error)
}

// DynamicToolHandler handles dynamic tool call requests from the server.
type DynamicToolHandler interface {
	HandleDynamicToolCall(ctx context.Context, req DynamicToolCallRequest) (DynamicToolCallResult, error)
}

// DynamicToolCallRequest is the payload for an "item/tool/call" server-initiated request.
type DynamicToolCallRequest struct {
	ToolName  string          `json:"tool"`
	ToolInput json.RawMessage `json:"arguments"`
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	// ItemID is populated from the wire's "callId" — unlike other item/*
	// message types in this package, item/tool/call identifies the call via
	// callId, not itemId. Field kept named ItemID for source compatibility
	// with existing callers.
	ItemID string `json:"callId"`
}

// DynamicToolCallOutputContentItem is a single content item returned in a
// DynamicToolCallResult. It is a discriminated union matching the
// DynamicToolCallOutputContentItem schema: exactly one of the "inputText" or
// "inputImage" variants. Construct values with TextContent or ImageContent
// rather than populating the fields directly.
type DynamicToolCallOutputContentItem struct {
	// Type discriminates the variant: "inputText" or "inputImage".
	Type string `json:"type"`

	// Text holds the payload for the "inputText" variant.
	Text string `json:"text,omitempty"`

	// ImageURL holds the payload for the "inputImage" variant.
	ImageURL string `json:"imageUrl,omitempty"`
}

// TextContent builds an "inputText" DynamicToolCallOutputContentItem.
func TextContent(text string) DynamicToolCallOutputContentItem {
	return DynamicToolCallOutputContentItem{Type: "inputText", Text: text}
}

// ImageContent builds an "inputImage" DynamicToolCallOutputContentItem.
func ImageContent(url string) DynamicToolCallOutputContentItem {
	return DynamicToolCallOutputContentItem{Type: "inputImage", ImageURL: url}
}

// DynamicToolCallResult is the response to a DynamicToolCallRequest.
type DynamicToolCallResult struct {
	ContentItems []DynamicToolCallOutputContentItem `json:"contentItems"`
	Success      bool                               `json:"success"`
}

// Dispatcher routes incoming server requests to specialised handlers by method.
// Any method not handled by a registered handler is denied with a default response.
//
// Dispatcher implements RequestHandler so it can be passed to WithRequestHandler.
type Dispatcher struct {
	// Exec handles "item/commandExecution/requestApproval" requests.
	// If nil, such requests are denied.
	Exec ExecApprovalHandler

	// File handles "item/fileChange/requestApproval" requests.
	// If nil, such requests are denied.
	File FileApprovalHandler

	// DynamicTool handles "item/tool/call" server-initiated requests.
	// If nil, dynamic tool calls are answered with an empty content array.
	DynamicTool DynamicToolHandler

	// MCP handles "item/mcp/requestApproval" requests.
	// If nil, such requests are declined.
	MCP MCPApprovalHandler

	// Fallback is consulted for any request method not matched by the above.
	// If nil, unmatched requests return ErrUnsupportedServerRequest.
	Fallback RequestHandler
}

// HandleServerRequest implements RequestHandler.
func (d *Dispatcher) HandleServerRequest(ctx context.Context, req ServerRequest) (ServerResponse, error) {
	switch req.Method {
	case protocol.MethodItemCommandExecutionRequestApproval:
		if d.Exec == nil {
			return serverResponseFrom(CommandExecutionApprovalResult{Decision: ApprovalDecisionDecline})
		}
		var r CommandExecutionApprovalRequest
		if err := json.Unmarshal(req.Params, &r); err != nil {
			return ServerResponse{}, err
		}
		result, err := d.Exec.HandleCommandExecutionApproval(ctx, r)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)

	case protocol.MethodItemFileChangeRequestApproval:
		if d.File == nil {
			return serverResponseFrom(FileChangeApprovalResult{Decision: FileChangeApprovalDecisionDecline})
		}
		var r FileChangeApprovalRequest
		if err := json.Unmarshal(req.Params, &r); err != nil {
			return ServerResponse{}, err
		}
		result, err := d.File.HandleFileChangeApproval(ctx, r)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)

	case protocol.MethodItemToolCall:
		if d.DynamicTool == nil {
			return serverResponseFrom(DynamicToolCallResult{ContentItems: []DynamicToolCallOutputContentItem{}, Success: false})
		}
		var r DynamicToolCallRequest
		if err := json.Unmarshal(req.Params, &r); err != nil {
			return ServerResponse{}, err
		}
		result, err := d.DynamicTool.HandleDynamicToolCall(ctx, r)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)

	case protocol.MethodItemMCPRequestApproval:
		if d.MCP == nil {
			return serverResponseFrom(MCPToolCallApprovalResponse{Decision: "decline"})
		}
		var r MCPToolCallApprovalRequest
		if err := json.Unmarshal(req.Params, &r); err != nil {
			return ServerResponse{}, err
		}
		result, err := d.MCP.HandleMCPToolCallApproval(ctx, r)
		if err != nil {
			return ServerResponse{}, err
		}
		return serverResponseFrom(result)

	default:
		if d.Fallback != nil {
			return d.Fallback.HandleServerRequest(ctx, req)
		}
		return ServerResponse{}, protocol.ErrUnsupportedServerRequest
	}
}

// TurnInput is a single typed input item for a turn or steer request. Construct
// values with TextInput, ImageInput, LocalImageInput, SkillInput, or
// MentionInput. Each serializes to the multi-part wire shape the app-server
// expects.
type TurnInput map[string]any

// TextInput wraps a plain text fragment: {"type":"text","text":...}.
func TextInput(text string) TurnInput {
	return TurnInput{"type": "text", "text": text}
}

// ImageInput references a remote image by URL: {"type":"image","url":...}.
// mediaType is optional; when non-empty it is sent as "mediaType".
func ImageInput(url, mediaType string) TurnInput {
	item := TurnInput{"type": "image", "url": url}
	if mediaType != "" {
		item["mediaType"] = mediaType
	}
	return item
}

// LocalImageInput reads the file at path synchronously, base64-encodes it, and
// produces an image input carrying a data: URI. The media type is inferred from
// the file extension, defaulting to application/octet-stream.
func LocalImageInput(path string) (TurnInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("local image input: %w", err)
	}
	mediaType := mime.TypeByExtension(filepath.Ext(path))
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	encoded := base64DataURI(mediaType, data)
	return TurnInput{"type": "image", "url": encoded, "mediaType": mediaType}, nil
}

// SkillInput references a named skill: {"type":"skill","name":...}.
func SkillInput(skillName string) TurnInput {
	return TurnInput{"type": "skill", "name": skillName}
}

// MentionInput references an @mention resource:
// {"type":"mention","id":...,"text":...}.
func MentionInput(mentionID, text string) TurnInput {
	return TurnInput{"type": "mention", "id": mentionID, "text": text}
}

// encodeInputs serializes typed inputs into the JSON-array string carried by the
// Input field; TurnStartRequest/TurnSteerRequest marshaling pass it through as
// the wire input array. Returns "" for an empty slice.
func encodeInputs(inputs []TurnInput) string {
	if len(inputs) == 0 {
		return ""
	}
	b, err := json.Marshal(inputs)
	if err != nil {
		return ""
	}
	return string(b)
}

func base64DataURI(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// FileChangePayload is the decoded payload for an ItemKindFileChange item.
type FileChangePayload struct {
	FilePath   string `json:"filePath"`
	ChangeType string `json:"changeType"` // "create", "modify", "delete"
	Diff       string `json:"diff,omitempty"`
	NewContent string `json:"newContent,omitempty"`
	OldContent string `json:"oldContent,omitempty"`
}

// CommandExecutionPayload is the decoded payload for an ItemKindCommandExecution item.
type CommandExecutionPayload struct {
	Command  string `json:"command"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

// ItemAsFileChange attempts to decode the item payload as a FileChangePayload.
// Returns nil if the item is not a fileChange or decoding fails.
func ItemAsFileChange(item Item) *FileChangePayload {
	if item.Kind != ItemKindFileChange {
		return nil
	}
	payload := item.PayloadBytes()
	if len(payload) == 0 {
		return nil
	}
	var p FileChangePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	return &p
}

// ItemAsCommandExecution attempts to decode the item payload as a CommandExecutionPayload.
// Returns nil if the item is not a commandExecution or decoding fails.
func ItemAsCommandExecution(item Item) *CommandExecutionPayload {
	if item.Kind != ItemKindCommandExecution {
		return nil
	}
	payload := item.PayloadBytes()
	if len(payload) == 0 {
		return nil
	}
	var p CommandExecutionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	return &p
}
