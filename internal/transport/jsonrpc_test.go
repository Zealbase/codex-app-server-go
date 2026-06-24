package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTransportRoutesCallNotificationAndInboundRequest(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	tr := New(client)
	defer tr.Close()

	var serverWG sync.WaitGroup
	serverWG.Add(1)
	go func() {
		defer serverWG.Done()
		dec := json.NewDecoder(server)
		enc := json.NewEncoder(server)
		enc.SetEscapeHTML(false)

		var req map[string]json.RawMessage
		if err := dec.Decode(&req); err != nil {
			t.Errorf("decode call: %v", err)
			return
		}
		if got := mustString(t, req["method"]); got != "ping" {
			t.Errorf("method = %q", got)
			return
		}
		if err := enc.Encode(map[string]any{
			"id":     mustID(t, req["id"]),
			"result": map[string]any{"pong": true},
		}); err != nil {
			t.Errorf("encode response: %v", err)
			return
		}

		var inboundReq map[string]json.RawMessage
		if err := enc.Encode(map[string]any{
			"method": "thread/updated",
			"params": map[string]any{"name": "demo"},
		}); err != nil {
			t.Errorf("encode notification: %v", err)
			return
		}
		if err := enc.Encode(map[string]any{
			"method": "item/requestApproval",
			"id":     2,
			"params": map[string]any{"reason": "approve"},
		}); err != nil {
			t.Errorf("encode inbound request: %v", err)
			return
		}
		if err := dec.Decode(&inboundReq); err != nil {
			t.Errorf("decode reply: %v", err)
			return
		}
		if got := mustID(t, inboundReq["id"]); got != 2 {
			t.Errorf("reply id = %d", got)
			return
		}
		if got := mustString(t, inboundReq["method"]); got != "" {
			t.Errorf("reply should not include method, got %q", got)
			return
		}
		if got := string(inboundReq["result"]); got != `{"decision":"accept"}` {
			t.Errorf("reply result = %s", got)
			return
		}
	}()

	var result struct {
		Pong bool `json:"pong"`
	}
	if err := tr.Call(context.Background(), "ping", map[string]any{"x": 1}, &result); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if !result.Pong {
		t.Fatalf("unexpected call result: %+v", result)
	}

	select {
	case n := <-tr.Notifications():
		var payload struct {
			Name string `json:"name"`
		}
		if err := n.DecodeParams(&payload); err != nil {
			t.Fatalf("notification decode failed: %v", err)
		}
		if payload.Name != "demo" {
			t.Fatalf("notification payload = %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification")
	}

	select {
	case req := <-tr.Requests():
		if req.Method() != "item/requestApproval" {
			t.Fatalf("request method = %q", req.Method())
		}
		var payload struct {
			Reason string `json:"reason"`
		}
		if err := req.DecodeParams(&payload); err != nil {
			t.Fatalf("request decode failed: %v", err)
		}
		if payload.Reason != "approve" {
			t.Fatalf("request payload = %+v", payload)
		}
		if err := req.Reply(context.Background(), map[string]string{"decision": "accept"}); err != nil {
			t.Fatalf("reply failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for inbound request")
	}

	serverWG.Wait()
}

func TestTransportCloseFailsPendingCall(t *testing.T) {
	client, server := net.Pipe()
	tr := New(client)
	seenReq := make(chan struct{})

	go func() {
		defer close(seenReq)
		dec := json.NewDecoder(server)
		var msg map[string]json.RawMessage
		_ = dec.Decode(&msg)
	}()

	callErr := make(chan error, 1)
	go func() {
		var result map[string]any
		callErr <- tr.Call(context.Background(), "wait", nil, &result)
	}()

	select {
	case <-seenReq:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for request to reach server")
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	_ = server.Close()

	select {
	case err := <-callErr:
		if !errors.Is(err, ErrClosed) && !errors.Is(err, ErrDisconnected) {
			t.Fatalf("unexpected call error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pending call to fail")
	}

	select {
	case <-tr.Done():
	case <-time.After(time.Second):
		t.Fatal("done channel did not close")
	}
}

func TestTransportReconnect(t *testing.T) {
	var (
		mu    sync.Mutex
		conns []net.Conn
	)

	opener := func(context.Context) (io.ReadWriteCloser, error) {
		client, server := net.Pipe()
		mu.Lock()
		conns = append(conns, server)
		mu.Unlock()
		return client, nil
	}

	tr, err := NewReconnecting(context.Background(), opener)
	if err != nil {
		t.Fatalf("new reconnecting transport: %v", err)
	}
	defer tr.Close()

	doRoundTrip := func(server net.Conn, wantMethod string, resp any) {
		t.Helper()
		dec := json.NewDecoder(server)
		enc := json.NewEncoder(server)
		enc.SetEscapeHTML(false)

		var msg map[string]json.RawMessage
		if err := dec.Decode(&msg); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := mustString(t, msg["method"]); got != wantMethod {
			t.Fatalf("method = %q", got)
		}
		if err := enc.Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}

	var first struct {
		OK bool `json:"ok"`
	}
	go func() {
		mu.Lock()
		server := conns[0]
		mu.Unlock()
		doRoundTrip(server, "first", map[string]any{
			"id":     1,
			"result": map[string]any{"ok": true},
		})
	}()
	if err := tr.Call(context.Background(), "first", nil, &first); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if !first.OK {
		t.Fatalf("unexpected first result: %+v", first)
	}

	mu.Lock()
	_ = conns[0].Close()
	mu.Unlock()
	time.Sleep(50 * time.Millisecond)

	if err := tr.Reconnect(context.Background()); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}

	var second struct {
		OK bool `json:"ok"`
	}
	go func() {
		mu.Lock()
		server := conns[1]
		mu.Unlock()
		doRoundTrip(server, "second", map[string]any{
			"id":     1,
			"result": map[string]any{"ok": true},
		})
	}()
	if err := tr.Call(context.Background(), "second", nil, &second); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if !second.OK {
		t.Fatalf("unexpected second result: %+v", second)
	}
}

func mustString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func mustID(t *testing.T, raw json.RawMessage) uint64 {
	t.Helper()
	id, err := parseID(raw)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}
	return id
}
