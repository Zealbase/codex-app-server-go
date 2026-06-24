package protocol

import (
	"encoding/json"

	schematypes "github.com/zealbase/codex-app-server-go/codexgo/internal/protocol/schema"
)

const JSONRPCVersion = "2.0"

// Method name constants for all JSON-RPC methods.
const (
	MethodInitialize                          = "initialize"
	MethodInitialized                         = "initialized"
	MethodThreadStart                         = "thread/start"
	MethodThreadResume                        = "thread/resume"
	MethodThreadRead                          = "thread/read"
	MethodTurnStart                           = "turn/start"
	MethodTurnSteer                           = "turn/steer"
	MethodTurnInterrupt                       = "turn/interrupt"
	MethodReviewStart                         = "review/start"
	MethodModelList                           = "model/list"
	MethodTurnStarted                         = "turn/started"
	MethodTurnCompleted                       = "turn/completed"
	MethodTurnDiffUpdated                     = "turn/diff/updated"
	MethodTurnPlanUpdated                     = "turn/plan/updated"
	MethodThreadTokenUsageUpdated             = "thread/tokenUsage/updated"
	MethodError                               = "error"
	MethodItemStarted                         = "item/started"
	MethodItemCompleted                       = "item/completed"
	MethodItemAgentMessageDelta               = "item/agentMessage/delta"
	MethodItemPlanDelta                       = "item/plan/delta"
	MethodItemReasoningSummaryTextDelta       = "item/reasoning/summaryTextDelta"
	MethodItemReasoningSummaryPartAdded       = "item/reasoning/summaryPartAdded"
	MethodItemReasoningTextDelta              = "item/reasoning/textDelta"
	MethodItemCommandExecutionOutputDelta     = "item/commandExecution/outputDelta"
	MethodItemFileChangePatchUpdated          = "item/fileChange/patchUpdated"
	MethodItemFileChangeOutputDelta           = "item/fileChange/outputDelta"
	MethodItemAutoApprovalReviewStarted       = "item/autoApprovalReview/started"
	MethodItemAutoApprovalReviewCompleted     = "item/autoApprovalReview/completed"
	MethodItemCommandExecutionRequestApproval = "item/commandExecution/requestApproval"
	MethodItemFileChangeRequestApproval       = "item/fileChange/requestApproval"
	MethodItemPermissionsRequestApproval      = "item/permissions/requestApproval"
	MethodItemToolRequestUserInput            = "item/tool/requestUserInput"
	MethodItemToolCall                        = "item/tool/call"
	MethodItemMCPRequestApproval              = "item/mcp/requestApproval"
	MethodTurnDiff                            = "turn/diff"

	// Thread lifecycle
	MethodThreadCompact    = "thread/compact/start"
	MethodThreadFork       = "thread/fork"
	MethodThreadList       = "thread/list"
	MethodThreadLoadedList = "thread/loaded/list"
	MethodThreadArchive    = "thread/archive"
	MethodThreadUnarchive  = "thread/unarchive"
	MethodThreadSetName    = "thread/name/set"
	MethodThreadRollback   = "thread/rollback"

	// Thread goals
	MethodThreadGoalSet   = "thread/goal/set"
	MethodThreadGoalClear = "thread/goal/clear"
	MethodThreadGoalGet   = "thread/goal/get"

	// Config mutations (slash-command equivalents)
	MethodConfigUpdate = "config/update"

	// Config CRUD RPCs
	MethodConfigRead       = "config/read"
	MethodConfigValueWrite = "config/value/write"
	MethodConfigBatchWrite = "config/batchWrite"

	// Skills RPCs
	MethodSkillsList          = "skills/list"
	MethodSkillsConfigWrite   = "skills/config/write"
	MethodSkillsExtraRootsSet = "skills/extraRoots/set"

	// Experimental-feature RPCs
	MethodExperimentalFeatureList          = "experimentalFeature/list"
	MethodExperimentalFeatureEnablementSet = "experimentalFeature/enablement/set"

	// Hooks (server-side) RPC
	MethodHooksList = "hooks/list"

	// Interactive command-exec (PTY) RPCs
	MethodCommandExec          = "command/exec"
	MethodCommandExecWrite     = "command/exec/write"
	MethodCommandExecResize    = "command/exec/resize"
	MethodCommandExecTerminate = "command/exec/terminate"

	// Command-exec streaming notification (server-push)
	MethodCommandExecOutputDelta = "command/exec/outputDelta"

	// Thread extras (metadata / subscription / mutations)
	MethodThreadDelete                = "thread/delete"
	MethodThreadUnsubscribe           = "thread/unsubscribe"
	MethodThreadMetadataUpdate        = "thread/metadata/update"
	MethodThreadShellCommand          = "thread/shellCommand"
	MethodThreadApproveGuardianDenied = "thread/approveGuardianDeniedAction"
	MethodThreadInjectItems           = "thread/inject_items"

	// Model provider capabilities RPC
	MethodModelProviderCapabilitiesRead = "modelProvider/capabilities/read"

	// Additional server-push notifications (typed via events_extra.go)
	// NP1 — thread & account lifecycle
	MethodThreadDeleted            = "thread/deleted"
	MethodThreadNameUpdated        = "thread/name/updated"
	MethodThreadCompacted          = "thread/compacted"
	MethodThreadSettingsUpdated    = "thread/settings/updated"
	MethodAccountRateLimitsUpdated = "account/rateLimits/updated"
	// NP2 — warnings & model diagnostics
	MethodWarning                      = "warning"
	MethodConfigWarning                = "configWarning"
	MethodDeprecationNotice            = "deprecationNotice"
	MethodGuardianWarning              = "guardianWarning"
	MethodWindowsWorldWritableWarning  = "windows/worldWritableWarning"
	MethodWindowsSandboxSetupCompleted = "windowsSandbox/setupCompleted"
	MethodModelRerouted                = "model/rerouted"
	MethodModelVerification            = "model/verification"
	MethodTurnModerationMetadata       = "turn/moderationMetadata"
	// NP3 — process / exec / hooks
	MethodProcessExited            = "process/exited"
	MethodProcessOutputDelta       = "process/outputDelta"
	MethodItemTerminalInteraction  = "item/commandExecution/terminalInteraction"
	MethodItemMcpToolCallProgress  = "item/mcpToolCall/progress"
	MethodHookStarted              = "hook/started"
	MethodHookCompleted            = "hook/completed"
	MethodRawResponseItemCompleted = "rawResponseItem/completed"
	// NP4 — subsystem status
	MethodMcpServerStatusUpdated        = "mcpServer/startupStatus/updated"
	MethodMcpServerOauthLoginCompleted  = "mcpServer/oauthLogin/completed"
	MethodSkillsChanged                 = "skills/changed"
	MethodFsChanged                     = "fs/changed"
	MethodAppListUpdated                = "app/list/updated"
	MethodRemoteControlStatusChanged    = "remoteControl/status/changed"
	MethodExternalAgentConfigImportDone = "externalAgentConfig/import/completed"
	MethodExternalAgentConfigImportProg = "externalAgentConfig/import/progress"
	// NP5 — realtime (voice/audio)
	MethodThreadRealtimeStarted         = "thread/realtime/started"
	MethodThreadRealtimeClosed          = "thread/realtime/closed"
	MethodThreadRealtimeError           = "thread/realtime/error"
	MethodThreadRealtimeItemAdded       = "thread/realtime/itemAdded"
	MethodThreadRealtimeSdp             = "thread/realtime/sdp"
	MethodThreadRealtimeOutputAudio     = "thread/realtime/outputAudio/delta"
	MethodThreadRealtimeTranscriptDelta = "thread/realtime/transcript/delta"
	MethodThreadRealtimeTranscriptDone  = "thread/realtime/transcript/done"

	// Account / login RPCs
	MethodAccountLoginStart  = "account/login/start"
	MethodAccountLoginCancel = "account/login/cancel"
	MethodAccountRead        = "account/read"
	MethodAccountLogout      = "account/logout"

	// Account / login notifications (server-push)
	MethodAccountLoginCompleted = "account/login/completed"
	MethodAccountUpdated        = "account/updated"

	// Thread lifecycle notifications (server-push)
	MethodThreadStarted         = "thread/started"
	MethodThreadArchived        = "thread/archived"
	MethodThreadUnarchived      = "thread/unarchived"
	MethodThreadClosed          = "thread/closed"
	MethodThreadStatusChanged   = "thread/status/changed"
	MethodThreadGoalUpdated     = "thread/goal/updated"
	MethodThreadGoalCleared     = "thread/goal/cleared"
	MethodItemUpdated           = "item/updated"
	MethodServerRequestResolved = "serverRequest/resolved"
)

// RPC envelope type aliases -- canonical definitions live in the generated schema package.

type RPCRequest = schematypes.RPCRequest
type RPCResponse = schematypes.RPCResponse
type RPCNotification = schematypes.RPCNotification
type RPCError = schematypes.RPCError

// Backward-compat aliases.
type Request = RPCRequest
type Response = RPCResponse
type Notification = RPCNotification
type ErrorObject = RPCError

// Utility helpers for building JSON-RPC payloads.

func RawJSON(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func MustRawJSON(v any) json.RawMessage {
	b, err := RawJSON(v)
	if err != nil {
		panic(err)
	}
	return b
}

func StringID(id string) json.RawMessage {
	return MustRawJSON(id)
}

func NumberID(id int64) json.RawMessage {
	return MustRawJSON(id)
}
