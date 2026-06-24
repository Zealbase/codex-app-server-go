package codexgo_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go"
	"github.com/zealbase/codex-app-server-go/internal/testutil"
)

func newAccountTestClient(t *testing.T, mock *testutil.MockServer, clientConn net.Conn) *codexgo.Client {
	t.Helper()
	mock.Handle("initialize", func(_ json.RawMessage) (any, error) {
		return map[string]any{"userAgent": "mock/1.0"}, nil
	})
	mock.Handle("initialized", func(_ json.RawMessage) (any, error) { return nil, nil })

	rc := &connReadCloser{clientConn}
	wc := &connWriteCloser{clientConn}
	client, err := codexgo.New(codexgo.WithStdioTransport(rc, wc))
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return client
}

func TestAccountRead(t *testing.T) {
	mock, clientConn := testutil.NewMockServer()
	defer mock.Close()

	mock.Handle("account/read", func(_ json.RawMessage) (any, error) {
		return map[string]any{
			"account": map[string]any{
				"type":     "chatgpt",
				"email":    "user@example.com",
				"planType": "pro",
			},
			"requiresOpenaiAuth": false,
		}, nil
	})

	client := newAccountTestClient(t, mock, clientConn)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := client.AccountRead(ctx)
	if err != nil {
		t.Fatalf("AccountRead(): %v", err)
	}
	if res.Account == nil {
		t.Fatal("expected non-nil account")
	}
	if res.Account.Email != "user@example.com" {
		t.Fatalf("unexpected email: %q", res.Account.Email)
	}
	if res.Account.PlanType != codexgo.PlanTypePro {
		t.Fatalf("unexpected planType: %q", res.Account.PlanType)
	}
}

func TestLoginAPIKey(t *testing.T) {
	mock, clientConn := testutil.NewMockServer()
	defer mock.Close()

	gotKey := make(chan string, 1)
	mock.Handle("account/login/start", func(params json.RawMessage) (any, error) {
		var req struct {
			Type   string `json:"type"`
			APIKey string `json:"apiKey"`
		}
		testutil.MustReadParams(params, &req)
		if req.Type != "apiKey" {
			t.Errorf("unexpected login type: %q", req.Type)
		}
		gotKey <- req.APIKey
		return map[string]any{"type": "apiKey"}, nil
	})

	client := newAccountTestClient(t, mock, clientConn)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.LoginAPIKey(ctx, "sk-test-123"); err != nil {
		t.Fatalf("LoginAPIKey(): %v", err)
	}
	select {
	case k := <-gotKey:
		if k != "sk-test-123" {
			t.Fatalf("unexpected apiKey: %q", k)
		}
	case <-ctx.Done():
		t.Fatal("server never received login params")
	}
}

func TestLoginChatGPTAndWait(t *testing.T) {
	mock, clientConn := testutil.NewMockServer()
	defer mock.Close()

	mock.Handle("account/login/start", func(_ json.RawMessage) (any, error) {
		return map[string]any{
			"type":    "chatgpt",
			"loginId": "login-abc",
			"authUrl": "https://auth.example/oauth",
		}, nil
	})

	client := newAccountTestClient(t, mock, clientConn)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	handle, err := client.LoginChatGPT(ctx)
	if err != nil {
		t.Fatalf("LoginChatGPT(): %v", err)
	}
	if handle.LoginID != "login-abc" || handle.AuthURL != "https://auth.example/oauth" {
		t.Fatalf("unexpected handle: %+v", handle)
	}

	done := make(chan codexgo.LoginCompleted, 1)
	errc := make(chan error, 1)
	go func() {
		res, err := handle.Wait(ctx)
		if err != nil {
			errc <- err
			return
		}
		done <- res
	}()

	// Give the Wait subscription a moment to register before notifying.
	time.Sleep(50 * time.Millisecond)
	if err := mock.Notify("account/login/completed", map[string]any{
		"success": true,
		"loginId": "login-abc",
	}); err != nil {
		t.Fatalf("Notify(): %v", err)
	}

	select {
	case res := <-done:
		if !res.Success || res.LoginID != "login-abc" {
			t.Fatalf("unexpected completion: %+v", res)
		}
	case err := <-errc:
		t.Fatalf("Wait(): %v", err)
	case <-ctx.Done():
		t.Fatal("login wait timed out")
	}
}

func TestLogout(t *testing.T) {
	mock, clientConn := testutil.NewMockServer()
	defer mock.Close()

	called := make(chan struct{}, 1)
	mock.Handle("account/logout", func(_ json.RawMessage) (any, error) {
		called <- struct{}{}
		return map[string]any{}, nil
	})

	client := newAccountTestClient(t, mock, clientConn)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Logout(ctx); err != nil {
		t.Fatalf("Logout(): %v", err)
	}
	select {
	case <-called:
	case <-ctx.Done():
		t.Fatal("server never received logout")
	}
}
