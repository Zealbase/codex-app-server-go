package codexgo_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go"
	"github.com/zealbase/codex-app-server-go/internal/testutil"
)

func TestWithInputsWireFormat(t *testing.T) {
	client, mock := newClientFromMock(t)

	type inputItem struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		URL       string `json:"url"`
		MediaType string `json:"mediaType"`
		Name      string `json:"name"`
		ID        string `json:"id"`
	}
	captured := make(chan []inputItem, 1)
	mock.Handle("turn/start", func(params json.RawMessage) (any, error) {
		var wire struct {
			Input []inputItem `json:"input"`
		}
		testutil.MustReadParams(params, &wire)
		select {
		case captured <- wire.Input:
		default:
		}
		return map[string]any{"turn": map[string]any{"id": "t1", "status": "inProgress"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.TurnStart(ctx, func() codexgo.TurnStartRequest {
		req := codexgo.TurnStartRequest{ThreadID: "thread-1"}
		codexgo.WithInputs(
			codexgo.TextInput("hello"),
			codexgo.ImageInput("https://example.com/a.png", "image/png"),
			codexgo.SkillInput("review"),
			codexgo.MentionInput("m1", "@file"),
		)(&req)
		return req
	}())
	if err != nil {
		t.Fatalf("TurnStart: %v", err)
	}

	select {
	case items := <-captured:
		if len(items) != 4 {
			t.Fatalf("expected 4 input items, got %d: %+v", len(items), items)
		}
		if items[0].Type != "text" || items[0].Text != "hello" {
			t.Fatalf("bad text item: %+v", items[0])
		}
		if items[1].Type != "image" || items[1].URL != "https://example.com/a.png" || items[1].MediaType != "image/png" {
			t.Fatalf("bad image item: %+v", items[1])
		}
		if items[2].Type != "skill" || items[2].Name != "review" {
			t.Fatalf("bad skill item: %+v", items[2])
		}
		if items[3].Type != "mention" || items[3].ID != "m1" || items[3].Text != "@file" {
			t.Fatalf("bad mention item: %+v", items[3])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for turn/start")
	}
}

func TestLocalImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pixel.png")
	// 1x1 transparent PNG.
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatalf("write temp png: %v", err)
	}

	item, err := codexgo.LocalImageInput(path)
	if err != nil {
		t.Fatalf("LocalImageInput: %v", err)
	}
	url, _ := item["url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("expected png data URI, got %q", url)
	}
	if item["type"] != "image" {
		t.Fatalf("expected type image, got %v", item["type"])
	}
}

func TestLocalImageInputMissingFile(t *testing.T) {
	if _, err := codexgo.LocalImageInput(filepath.Join(t.TempDir(), "nope.png")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestModelListAndModels(t *testing.T) {
	client, mock := newClientFromMock(t)

	mock.Handle("model/list", func(params json.RawMessage) (any, error) {
		var req struct {
			IncludeHidden bool `json:"includeHidden"`
		}
		testutil.MustReadParams(params, &req)
		data := []map[string]any{
			{"id": "gpt-5", "model": "gpt-5", "hidden": false, "isDefault": true},
		}
		if req.IncludeHidden {
			data = append(data, map[string]any{"id": "gpt-secret", "model": "gpt-secret", "hidden": true})
		}
		return map[string]any{"data": data}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	all, err := client.ModelList(ctx, true)
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 models with hidden, got %d", len(all))
	}

	visible, err := client.ModelList(ctx, false)
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if len(visible) != 1 || visible[0].ID != "gpt-5" || !visible[0].IsDefault {
		t.Fatalf("unexpected visible models: %+v", visible)
	}

	ids, err := client.Models(ctx)
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(ids) != 1 || ids[0] != "gpt-5" {
		t.Fatalf("unexpected model ids: %+v", ids)
	}
}

func TestTypedEnumOptions(t *testing.T) {
	var req codexgo.TurnStartRequest
	codexgo.WithApprovalMode(codexgo.ApprovalModeNever)(&req)
	codexgo.WithSandboxMode(codexgo.SandboxWorkspaceWrite)(&req)
	if req.ApprovalPolicy != "never" {
		t.Fatalf("approval policy: %q", req.ApprovalPolicy)
	}
	if req.SandboxPolicy != "workspace-write" {
		t.Fatalf("sandbox policy: %q", req.SandboxPolicy)
	}
}
