package transport

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestReconnectingHTTP_ReconnectsAfterDrop(t *testing.T) {
	oldInit := reconnectInitialBackoffNanos.Load()
	oldMax := reconnectMaxBackoffNanos.Load()
	reconnectInitialBackoffNanos.Store(int64(10 * time.Millisecond))
	reconnectMaxBackoffNanos.Store(int64(50 * time.Millisecond))
	defer func() {
		reconnectInitialBackoffNanos.Store(oldInit)
		reconnectMaxBackoffNanos.Store(oldMax)
	}()

	var conns int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			http.NotFound(w, r)
			return
		}
		n := atomic.AddInt32(&conns, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("response writer is not a flusher")
			return
		}
		// Each connection emits one event tagged with its connection number,
		// then the first connection drops while later ones stay open.
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"method\":\"notify/event\",\"params\":{\"conn\":%d}}\n\n", n)
		flusher.Flush()
		if n == 1 {
			// Drop the first SSE stream to force a reconnect.
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	tr := NewReconnectingHTTP(srv.URL)
	defer tr.Close()

	want := map[int]bool{1: false, 2: false}
	deadline := time.After(5 * time.Second)
	for len(want) > 0 {
		select {
		case n := <-tr.Notifications():
			var p struct {
				Conn int `json:"conn"`
			}
			if err := n.DecodeParams(&p); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			delete(want, p.Conn)
		case <-deadline:
			t.Fatalf("timed out; still waiting for events from connections %v", want)
		}
	}
}

func TestReconnectingHTTP_CloseStopsReconnect(t *testing.T) {
	oldInit := reconnectInitialBackoffNanos.Load()
	oldMax := reconnectMaxBackoffNanos.Load()
	reconnectInitialBackoffNanos.Store(int64(10 * time.Millisecond))
	reconnectMaxBackoffNanos.Store(int64(50 * time.Millisecond))
	defer func() {
		reconnectInitialBackoffNanos.Store(oldInit)
		reconnectMaxBackoffNanos.Store(oldMax)
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			w.Header().Set("Content-Type", "text/event-stream")
			<-r.Context().Done()
		}
	}))
	defer srv.Close()

	tr := NewReconnectingHTTP(srv.URL)
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-tr.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done channel not closed after Close")
	}
}
