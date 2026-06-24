package mock_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// CapturedRequest records a single inbound HTTP request to the mock server.
type CapturedRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// BodyJSON decodes the captured request body as a JSON object.
func (r CapturedRequest) BodyJSON() map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(r.Body, &out)
	return out
}

// MockResponsesServer is an in-process HTTP server that emulates the OpenAI
// Responses API surface the codex binary talks to: GET /v1/models and a queue
// of SSE bodies served from POST /v1/responses.
type MockResponsesServer struct {
	t      *testing.T
	server *httptest.Server

	mu        sync.Mutex
	queue     []string
	requests  []CapturedRequest
	modelGets int
	waiters   []chan struct{}
}

func newMockResponsesServer(t *testing.T) *MockResponsesServer {
	t.Helper()
	m := &MockResponsesServer{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", m.handleModels)
	mux.HandleFunc("/v1/responses", m.handleResponses)
	m.server = httptest.NewServer(mux)
	return m
}

// URL returns the base URL of the mock server (without a trailing /v1).
func (m *MockResponsesServer) URL() string {
	return m.server.URL
}

func (m *MockResponsesServer) handleModels(w http.ResponseWriter, r *http.Request) {
	m.record(r)
	m.mu.Lock()
	m.modelGets++
	m.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"mock-model","object":"model","created":0,"owned_by":"openai"}]}`)
}

func (m *MockResponsesServer) handleResponses(w http.ResponseWriter, r *http.Request) {
	m.record(r)

	m.mu.Lock()
	var body string
	if len(m.queue) > 0 {
		body = m.queue[0]
		m.queue = m.queue[1:]
	}
	m.mu.Unlock()

	if body == "" {
		// No queued response; emit a minimal completed stream so the turn
		// terminates instead of hanging.
		body = sseAssistantMessage("", "auto-empty")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (m *MockResponsesServer) record(r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	headers := make(map[string]string, len(r.Header))
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}

	m.mu.Lock()
	m.requests = append(m.requests, CapturedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Body:    bodyBytes,
	})
	waiters := m.waiters
	m.waiters = nil
	m.mu.Unlock()

	for _, ch := range waiters {
		close(ch)
	}
}

// EnqueueSSE queues a raw SSE body to be served by the next POST /v1/responses.
func (m *MockResponsesServer) EnqueueSSE(body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, body)
}

// EnqueueAssistantMessage queues an SSE stream that emits a single assistant
// message with the given text and response ID.
func (m *MockResponsesServer) EnqueueAssistantMessage(text, responseID string) {
	m.EnqueueSSE(sseAssistantMessage(text, responseID))
}

// Requests returns a snapshot of all captured requests.
func (m *MockResponsesServer) Requests() []CapturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CapturedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// responsesRequests returns only POST /v1/responses requests.
func (m *MockResponsesServer) responsesRequests() []CapturedRequest {
	out := []CapturedRequest{}
	for _, r := range m.Requests() {
		if r.Method == http.MethodPost && r.Path == "/v1/responses" {
			out = append(out, r)
		}
	}
	return out
}

// WaitForRequests blocks until at least n POST /v1/responses requests have been
// captured or the timeout elapses, then returns the captured Responses requests.
func (m *MockResponsesServer) WaitForRequests(n int, timeout time.Duration) []CapturedRequest {
	deadline := time.Now().Add(timeout)
	for {
		got := m.responsesRequests()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			return got
		}

		ch := make(chan struct{})
		m.mu.Lock()
		// Re-check under lock to avoid missing a request that landed between
		// the snapshot above and registering the waiter.
		if len(m.responsesRequestsLocked()) >= n {
			m.mu.Unlock()
			continue
		}
		m.waiters = append(m.waiters, ch)
		m.mu.Unlock()

		select {
		case <-ch:
		case <-time.After(time.Until(deadline)):
		}
	}
}

func (m *MockResponsesServer) responsesRequestsLocked() []CapturedRequest {
	out := []CapturedRequest{}
	for _, r := range m.requests {
		if r.Method == http.MethodPost && r.Path == "/v1/responses" {
			out = append(out, r)
		}
	}
	return out
}

// Close shuts down the mock HTTP server.
func (m *MockResponsesServer) Close() {
	m.server.Close()
}

// sseAssistantMessage builds an SSE response stream containing response.created,
// a single assistant output_text message, and response.completed with usage.
func sseAssistantMessage(text, responseID string) string {
	created := map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id": responseID,
		},
	}
	itemDone := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type": "message",
			"role": "assistant",
			"id":   "msg-" + responseID,
			"content": []any{
				map[string]any{"type": "output_text", "text": text},
			},
		},
	}
	completed := map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id": responseID,
			"usage": map[string]any{
				"input_tokens":          10,
				"input_tokens_details":  map[string]any{"cached_tokens": 0},
				"output_tokens":         5,
				"output_tokens_details": map[string]any{"reasoning_tokens": 0},
				"total_tokens":          15,
			},
		},
	}
	return sseEvent("response.created", created) +
		sseEvent("response.output_item.done", itemDone) +
		sseEvent("response.completed", completed)
}

func sseEvent(eventType string, payload map[string]any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("mock: marshal SSE event %q: %v", eventType, err))
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
}
