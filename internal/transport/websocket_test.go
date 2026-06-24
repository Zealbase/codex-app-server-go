package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func wsURL(s string) string {
	return "ws" + strings.TrimPrefix(s, "http")
}

func TestWebSocketTransportCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(rw, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var req map[string]json.RawMessage
		if err := json.Unmarshal(data, &req); err != nil {
			return
		}
		id := req["id"]
		resp := []byte(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"pong":true}}`)
		_ = c.Write(ctx, websocket.MessageText, resp)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tr, err := NewWebSocket(ctx, wsURL(srv.URL))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tr.Close()

	var out struct {
		Pong bool `json:"pong"`
	}
	if err := tr.Call(ctx, "ping", nil, &out); err != nil {
		t.Fatalf("call: %v", err)
	}
	if !out.Pong {
		t.Fatalf("expected pong=true, got %+v", out)
	}
}

func TestWebSocketTransportNotification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(rw, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		note := []byte(`{"jsonrpc":"2.0","method":"event/log","params":{"msg":"hi"}}`)
		_ = c.Write(ctx, websocket.MessageText, note)
		// Keep the connection alive until the client closes.
		_, _, _ = c.Read(ctx)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tr, err := NewWebSocket(ctx, wsURL(srv.URL))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tr.Close()

	select {
	case n := <-tr.Notifications():
		if n.Method != "event/log" {
			t.Fatalf("expected method event/log, got %q", n.Method)
		}
		var p struct {
			Msg string `json:"msg"`
		}
		if err := n.DecodeParams(&p); err != nil {
			t.Fatalf("decode params: %v", err)
		}
		if p.Msg != "hi" {
			t.Fatalf("expected msg=hi, got %q", p.Msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestWebSocketTransportClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(rw, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		_, _, _ = c.Read(r.Context())
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tr, err := NewWebSocket(ctx, wsURL(srv.URL))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-tr.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done() did not close after Close()")
	}
}
