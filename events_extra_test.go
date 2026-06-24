package codexgo

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zealbase/codex-app-server-go/internal/protocol"
)

// TestDecodeExtraNotifications verifies every notification added in the
// notification-coverage phase decodes into its typed event (not the
// RawNotificationEvent fallback). This is a pure unit test — no binary needed.
func TestDecodeExtraNotifications(t *testing.T) {
	cases := []struct {
		method string
		raw    string
		want   any
	}{
		// NP1
		{protocol.MethodThreadDeleted, `{"threadId":"t1"}`, ThreadDeletedEvent{}},
		{protocol.MethodThreadNameUpdated, `{"threadId":"t1","threadName":"n"}`, ThreadNameUpdatedEvent{}},
		{protocol.MethodThreadCompacted, `{"threadId":"t1","turnId":"u1"}`, ThreadCompactedEvent{}},
		{protocol.MethodThreadSettingsUpdated, `{"threadId":"t1","threadSettings":{}}`, ThreadSettingsUpdatedEvent{}},
		{protocol.MethodAccountUpdated, `{"authMode":"apiKey"}`, AccountUpdatedEvent{}},
		{protocol.MethodAccountRateLimitsUpdated, `{"rateLimits":{}}`, AccountRateLimitsUpdatedEvent{}},
		// NP2
		{protocol.MethodWarning, `{"message":"w"}`, WarningEvent{}},
		{protocol.MethodConfigWarning, `{"summary":"s"}`, ConfigWarningEvent{}},
		{protocol.MethodDeprecationNotice, `{"summary":"s"}`, DeprecationNoticeEvent{}},
		{protocol.MethodGuardianWarning, `{"threadId":"t1","message":"m"}`, GuardianWarningEvent{}},
		{protocol.MethodWindowsWorldWritableWarning, `{"samplePaths":["a"],"extraCount":1}`, WindowsWorldWritableWarningEvent{}},
		{protocol.MethodWindowsSandboxSetupCompleted, `{"mode":"m","success":true}`, WindowsSandboxSetupCompletedEvent{}},
		{protocol.MethodModelRerouted, `{"threadId":"t1","turnId":"u1","fromModel":"a","toModel":"b"}`, ModelReroutedEvent{}},
		{protocol.MethodModelVerification, `{"threadId":"t1","turnId":"u1","verifications":[]}`, ModelVerificationEvent{}},
		{protocol.MethodTurnModerationMetadata, `{"threadId":"t1","turnId":"u1","metadata":{}}`, TurnModerationMetadataEvent{}},
		// NP3
		{protocol.MethodProcessExited, `{"processHandle":"p","exitCode":0}`, ProcessExitedEvent{}},
		{protocol.MethodProcessOutputDelta, `{"processHandle":"p","stream":"stdout","deltaBase64":"aGk="}`, ProcessOutputDeltaEvent{}},
		{protocol.MethodItemTerminalInteraction, `{"threadId":"t1","turnId":"u1","itemId":"i","processId":"p","stdin":"x"}`, TerminalInteractionEvent{}},
		{protocol.MethodItemMcpToolCallProgress, `{"threadId":"t1","turnId":"u1","itemId":"i","message":"m"}`, McpToolCallProgressEvent{}},
		{protocol.MethodHookStarted, `{"threadId":"t1","run":{}}`, HookStartedEvent{}},
		{protocol.MethodHookCompleted, `{"threadId":"t1","run":{}}`, HookCompletedEvent{}},
		{protocol.MethodRawResponseItemCompleted, `{"threadId":"t1","turnId":"u1","item":{}}`, RawResponseItemCompletedEvent{}},
		// NP4
		{protocol.MethodMcpServerStatusUpdated, `{"name":"s","status":"ready"}`, McpServerStatusUpdatedEvent{}},
		{protocol.MethodMcpServerOauthLoginCompleted, `{"name":"s","success":true}`, McpServerOauthLoginCompletedEvent{}},
		{protocol.MethodSkillsChanged, `{"forceRefetch":true}`, SkillsChangedEvent{}},
		{protocol.MethodFsChanged, `{"watchId":"w","changedPaths":["/a"]}`, FsChangedEvent{}},
		{protocol.MethodAppListUpdated, `{"data":[]}`, AppListUpdatedEvent{}},
		{protocol.MethodRemoteControlStatusChanged, `{"status":"connected","serverName":"s","installationId":"i"}`, RemoteControlStatusChangedEvent{}},
		{protocol.MethodExternalAgentConfigImportDone, `{"importId":"i","itemTypeResults":[]}`, ExternalAgentConfigImportCompletedEvent{}},
		{protocol.MethodExternalAgentConfigImportProg, `{"importId":"i","itemTypeResults":[]}`, ExternalAgentConfigImportProgressEvent{}},
		// NP5
		{protocol.MethodThreadRealtimeStarted, `{"threadId":"t1","version":"v1"}`, ThreadRealtimeStartedEvent{}},
		{protocol.MethodThreadRealtimeClosed, `{"threadId":"t1","reason":"done"}`, ThreadRealtimeClosedEvent{}},
		{protocol.MethodThreadRealtimeError, `{"threadId":"t1","message":"e"}`, ThreadRealtimeErrorEvent{}},
		{protocol.MethodThreadRealtimeItemAdded, `{"threadId":"t1","item":{}}`, ThreadRealtimeItemAddedEvent{}},
		{protocol.MethodThreadRealtimeSdp, `{"threadId":"t1","sdp":"v=0"}`, ThreadRealtimeSdpEvent{}},
		{protocol.MethodThreadRealtimeOutputAudio, `{"threadId":"t1","audio":{}}`, ThreadRealtimeOutputAudioDeltaEvent{}},
		{protocol.MethodThreadRealtimeTranscriptDelta, `{"threadId":"t1","role":"user","delta":"hi"}`, ThreadRealtimeTranscriptDeltaEvent{}},
		{protocol.MethodThreadRealtimeTranscriptDone, `{"threadId":"t1","role":"user","text":"hi"}`, ThreadRealtimeTranscriptDoneEvent{}},
	}

	if len(cases) != 38 {
		t.Fatalf("expected 38 notification cases, got %d", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			ev := decodeEvent(tc.method, json.RawMessage(tc.raw))
			if ev.Method != tc.method {
				t.Fatalf("method = %q, want %q", ev.Method, tc.method)
			}
			gotType := typeName(ev.Value)
			wantType := typeName(tc.want)
			if gotType != wantType {
				t.Fatalf("decoded type = %s, want %s (fell through to Raw?)", gotType, wantType)
			}
			if _, isRaw := ev.Value.(RawNotificationEvent); isRaw {
				t.Fatalf("method %q fell through to RawNotificationEvent", tc.method)
			}
		})
	}
}

func typeName(v any) string {
	return fmt.Sprintf("%T", v)
}
