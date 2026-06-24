package real_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go/codexgo"
	"github.com/zealbase/codex-app-server-go/codexgo/internal/transport"
)

// HTTP transport is a work-in-progress feature. The Codex app-server only speaks
// WebSocket; HTTP+SSE support requires the ws-http-bridge sidecar
// (see plugins/codex-server/ws-http-bridge). Until that bridge is deployed in
// front of the target server, the HTTP transport tests below are disabled by
// default. Set CODEX_ENABLE_HTTP_TESTS=1 to run them against a bridge-fronted
// endpoint.
func skipHTTPTransportWIP(t *testing.T) {
	t.Helper()
	if os.Getenv("CODEX_ENABLE_HTTP_TESTS") != "1" {
		t.Skip("HTTP transport is WIP (requires ws-http-bridge); set CODEX_ENABLE_HTTP_TESTS=1 to run")
	}
}

// skipIfHTTPNotSupported skips the test when the server returns 405, which
// means the server only accepts WebSocket connections (bridge not deployed).
func skipIfHTTPNotSupported(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "405") {
		t.Skipf("HTTP transport not supported by this server (ws-http-bridge not deployed): %v", err)
	}
}

// runPingRoundTrip starts a thread on the given client and runs a single turn,
// asserting the final agent message is non-empty.
func runPingRoundTrip(t *testing.T, client *codexgo.Client, prompt string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	thread, err := client.StartThread(ctx, codexgo.WithThreadModel(realModel()))
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	defer thread.Close()

	result, err := thread.Run(ctx, prompt)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	resp, err := client.WaitForFinalAgentMessage(ctx, thread.ID(), result.Turn.ID)
	if err != nil {
		t.Fatalf("WaitForFinalAgentMessage: %v", err)
	}
	if resp == "" {
		t.Fatal("final agent message is empty")
	}
}

func TestReal_HTTPTransport(t *testing.T) {
	skipHTTPTransportWIP(t)
	skipIfNoEndpoint(t)
	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")

	var opts []codexgo.Option
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		opts = append(opts, codexgo.WithHTTPBearerToken(key))
	}
	opts = append(opts, codexgo.WithHTTPTransport(endpoint))

	client, err := codexgo.New(opts...)
	skipIfHTTPNotSupported(t, err)
	if err != nil {
		t.Fatalf("New (http): %v", err)
	}
	defer client.Close()

	runPingRoundTrip(t, client, "say PING")
}

func TestReal_WSTransport(t *testing.T) {
	skipIfNoEndpoint(t)
	if os.Getenv("CODEX_TRANSPORT_WS_DISABLED") == "1" {
		t.Skip("CODEX_TRANSPORT_WS_DISABLED=1; skipping WebSocket transport test")
	}
	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")

	var opts []codexgo.Option
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		opts = append(opts, codexgo.WithWSBearerToken(key))
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opts = append(opts, codexgo.WithWSTransport(dialCtx, endpoint))

	client, err := codexgo.New(opts...)
	if err != nil {
		t.Fatalf("New (ws): %v", err)
	}
	defer client.Close()

	runPingRoundTrip(t, client, "say PING")
}

func TestReal_ReconnectingHTTP(t *testing.T) {
	skipHTTPTransportWIP(t)
	skipIfNoEndpoint(t)
	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")

	var opts []codexgo.Option
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		opts = append(opts, codexgo.WithHTTPBearerToken(key))
	}
	opts = append(opts, codexgo.WithReconnectingHTTPTransport(endpoint))

	client, err := codexgo.New(opts...)
	skipIfHTTPNotSupported(t, err)
	if err != nil {
		t.Fatalf("New (reconnecting http): %v", err)
	}
	defer client.Close()

	runPingRoundTrip(t, client, "say PING")
}

func TestReal_RetryTransport(t *testing.T) {
	skipHTTPTransportWIP(t)
	skipIfNoEndpoint(t)
	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")

	var opts []codexgo.Option
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		opts = append(opts, codexgo.WithHTTPBearerToken(key))
	}
	opts = append(opts,
		codexgo.WithHTTPTransport(endpoint),
		codexgo.WithRetry(transport.DefaultRetryConfig()),
	)

	client, err := codexgo.New(opts...)
	skipIfHTTPNotSupported(t, err)
	if err != nil {
		t.Fatalf("New (retry): %v", err)
	}
	defer client.Close()

	runPingRoundTrip(t, client, "say PING")
}
