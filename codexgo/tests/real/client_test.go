package real_test

import (
	"context"
	"os"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
)

// skipIfNoEndpoint skips the calling test unless CODEX_REAL_ENDPOINT is set.
func skipIfNoEndpoint(t *testing.T) {
	t.Helper()
	if os.Getenv("CODEX_REAL_ENDPOINT") == "" {
		t.Skip("CODEX_REAL_ENDPOINT not set; skipping real integration test")
	}
}

// realModel returns the model to use for turns, overridable via CODEX_TEST_MODEL.
// Default is gpt-5.4-mini which is the smallest model available on the
// localdev server (codex-mini-latest does not exist there).
func realModel() string {
	if m := os.Getenv("CODEX_TEST_MODEL"); m != "" {
		return m
	}
	return "gpt-5.4-mini"
}

// newRealClient builds a Go SDK client pointed at CODEX_REAL_ENDPOINT.
// It skips the test if the endpoint is unset. Auth is read from CODEX_API_KEY.
// Transport defaults to HTTP; set CODEX_TRANSPORT=ws to use WebSocket.
func newRealClient(t *testing.T) *codexgo.Client {
	t.Helper()
	skipIfNoEndpoint(t)

	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")
	apiKey := os.Getenv("CODEX_API_KEY")

	var opts []codexgo.Option
	switch os.Getenv("CODEX_TRANSPORT") {
	case "ws":
		if apiKey != "" {
			opts = append(opts, codexgo.WithWSBearerToken(apiKey))
		}
		dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		opts = append(opts, codexgo.WithWSTransport(dialCtx, endpoint))
	default:
		if apiKey != "" {
			opts = append(opts, codexgo.WithHTTPBearerToken(apiKey))
		}
		opts = append(opts, codexgo.WithHTTPTransport(endpoint))
	}

	client, err := codexgo.New(opts...)
	if err != nil {
		t.Fatalf("newRealClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
