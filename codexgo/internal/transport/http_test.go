package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPTransportCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc":
			var req map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode rpc request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
		case "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			<-r.Context().Done()
		}
	}))
	defer srv.Close()

	tr := NewHTTP(srv.URL)
	defer tr.Close()

	var out struct {
		OK bool `json:"ok"`
	}
	if err := tr.Call(context.Background(), "test/method", map[string]any{"x": 1}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true, got %+v", out)
	}
}

func TestHTTPTransportNotification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("response writer is not a flusher")
			return
		}
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"method\":\"notify/event\",\"params\":{\"v\":42}}\n\n"))
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	tr := NewHTTP(srv.URL)
	defer tr.Close()

	select {
	case n := <-tr.Notifications():
		if n.Method != "notify/event" {
			t.Fatalf("unexpected method %q", n.Method)
		}
		var p struct {
			V int `json:"v"`
		}
		if err := n.DecodeParams(&p); err != nil {
			t.Fatalf("decode params: %v", err)
		}
		if p.V != 42 {
			t.Fatalf("expected v=42, got %d", p.V)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestHTTPTransportClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			w.Header().Set("Content-Type", "text/event-stream")
			<-r.Context().Done()
		}
	}))
	defer srv.Close()

	tr := NewHTTP(srv.URL)
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-tr.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done channel not closed after Close")
	}
}
