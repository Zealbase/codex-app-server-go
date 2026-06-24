package codexgo

import (
	"context"
	"io"

	"github.com/zealbase/codex-app-server-go/internal/transport"
)

type Option func(*clientConfig) error

type clientConfig struct {
	transport      Transport
	requestHandler RequestHandler
	// maxThreads sets the maximum number of concurrent live SessionThreads.
	// 0 means use the default (64). -1 means unlimited.
	maxThreads int

	// processPath, when non-empty, names a binary to spawn as a managed
	// subprocess whose stdio is wired to a StdioTransport.
	processPath string
	processArgs []string
	processEnv  []string
	processDir  string

	// autoLoginAPIKey, when non-empty, triggers a LoginAPIKey call inside New()
	// after the initialize handshake completes.
	autoLoginAPIKey string

	// autoInit requests the app-server initialize handshake be run inside New().
	// It is set by options that wire up a real client transport, and stays false
	// for the default os.Stdin/os.Stdout fallback (used when the SDK itself acts
	// as an app-server handler).
	autoInit bool

	// httpAuthHeaders / wsAuthHeaders carry auth headers applied when an
	// HTTP/WS transport is constructed via WithHTTPTransport / WithWSTransport.
	httpAuthHeaders map[string]string
	wsAuthHeaders   map[string]string

	// retry, when non-nil, wraps the constructed transport in retry logic.
	retry *transport.RetryConfig

	// innerTransport holds the raw internal transport built by
	// WithHTTPTransport / WithWSTransport, before retry wrapping and channel
	// adaptation. New() applies the retry policy to it (if any) regardless of
	// option ordering, then adapts it into cfg.transport.
	innerTransport transport.Transport
}

func WithTransport(t Transport) Option {
	return func(cfg *clientConfig) error {
		cfg.transport = t
		cfg.autoInit = true
		return nil
	}
}

func WithApprovalHandler(handler ApprovalHandler) Option {
	return func(cfg *clientConfig) error {
		if handler == nil {
			cfg.requestHandler = nil
			return nil
		}
		cfg.requestHandler = &approvalHandlerAdapter{handler: handler}
		return nil
	}
}

func WithRequestHandler(handler RequestHandler) Option {
	return func(cfg *clientConfig) error {
		cfg.requestHandler = handler
		return nil
	}
}

func WithStdioTransport(stdin io.ReadCloser, stdout io.WriteCloser) Option {
	return func(cfg *clientConfig) error {
		cfg.transport = NewStdioTransport(stdin, stdout)
		return nil
	}
}

// WithStdioProcess spawns binaryPath with args as a managed subprocess and
// wires its stdin/stdout to a StdioTransport. The process is shut down (EOF →
// SIGTERM → SIGKILL) when the Client is closed.
func WithStdioProcess(binaryPath string, args ...string) Option {
	return func(cfg *clientConfig) error {
		cfg.processPath = binaryPath
		cfg.processArgs = args
		cfg.autoInit = true
		return nil
	}
}

// WithProcessEnv appends key=value to the subprocess environment.
// Only applies when used with WithStdioProcess.
// May be called multiple times; each call appends one variable.
func WithProcessEnv(key, value string) Option {
	return func(cfg *clientConfig) error {
		cfg.processEnv = append(cfg.processEnv, key+"="+value)
		return nil
	}
}

// WithProcessDir sets the working directory for the subprocess spawned by
// WithStdioProcess. If empty, the current process's working directory is used.
func WithProcessDir(dir string) Option {
	return func(cfg *clientConfig) error {
		cfg.processDir = dir
		return nil
	}
}

func setAuthHeader(m *map[string]string, key, value string) {
	if *m == nil {
		*m = make(map[string]string)
	}
	(*m)[key] = value
}

// WithHTTPBearerToken sets "Authorization: Bearer <token>" on HTTP transports
// constructed via WithHTTPTransport. It has no effect on a transport supplied
// through WithTransport.
func WithHTTPBearerToken(token string) Option {
	return func(cfg *clientConfig) error {
		setAuthHeader(&cfg.httpAuthHeaders, "Authorization", "Bearer "+token)
		return nil
	}
}

// WithHTTPAPIKey sets "X-API-Key: <key>" on HTTP transports constructed via
// WithHTTPTransport. It has no effect on a transport supplied through
// WithTransport.
func WithHTTPAPIKey(key string) Option {
	return func(cfg *clientConfig) error {
		setAuthHeader(&cfg.httpAuthHeaders, "X-API-Key", key)
		return nil
	}
}

// WithWSBearerToken sets "Authorization: Bearer <token>" on WebSocket transports
// constructed via WithWSTransport. It has no effect on a transport supplied
// through WithTransport.
func WithWSBearerToken(token string) Option {
	return func(cfg *clientConfig) error {
		setAuthHeader(&cfg.wsAuthHeaders, "Authorization", "Bearer "+token)
		return nil
	}
}

// WithWSAPIKey sets "X-API-Key: <key>" on WebSocket transports constructed via
// WithWSTransport. It has no effect on a transport supplied through
// WithTransport.
func WithWSAPIKey(key string) Option {
	return func(cfg *clientConfig) error {
		setAuthHeader(&cfg.wsAuthHeaders, "X-API-Key", key)
		return nil
	}
}

// WithHTTPTransport constructs an HTTP transport targeting baseURL, applying any
// auth configured via WithHTTPBearerToken / WithHTTPAPIKey. Any retry policy
// configured via WithRetry is applied in New() regardless of option ordering.
//
// WIP: The Codex app-server speaks WebSocket only. This HTTP+SSE transport
// requires the ws-http-bridge sidecar (plugins/codex-server/ws-http-bridge) in
// front of the server; without it, requests fail with HTTP 405. Prefer
// WithWSTransport for direct connections until the bridge is generally deployed.
func WithHTTPTransport(baseURL string) Option {
	return func(cfg *clientConfig) error {
		var opts []transport.HTTPOption
		for k, v := range cfg.httpAuthHeaders {
			opts = append(opts, transport.WithHTTPHeader(k, v))
		}
		cfg.innerTransport = transport.NewHTTP(baseURL, opts...)
		cfg.autoInit = true
		return nil
	}
}

// WithWSTransport dials a WebSocket transport at url, applying any auth
// configured via WithWSBearerToken / WithWSAPIKey. Any retry policy configured
// via WithRetry is applied in New() regardless of option ordering.
func WithWSTransport(ctx context.Context, url string) Option {
	return func(cfg *clientConfig) error {
		var opts []transport.WSOption
		for k, v := range cfg.wsAuthHeaders {
			opts = append(opts, transport.WithWSHeader(k, v))
		}
		ws, err := transport.NewWebSocket(ctx, url, opts...)
		if err != nil {
			return err
		}
		cfg.innerTransport = ws
		cfg.autoInit = true
		return nil
	}
}

// WithReconnectingHTTPTransport is like WithHTTPTransport but the SSE
// connection is automatically re-established if it drops. Any retry policy
// configured via WithRetry is applied in New() regardless of option ordering.
//
// WIP: shares the HTTP+SSE transport that requires the ws-http-bridge sidecar.
// See WithHTTPTransport for details.
func WithReconnectingHTTPTransport(baseURL string) Option {
	return func(cfg *clientConfig) error {
		var opts []transport.HTTPOption
		for k, v := range cfg.httpAuthHeaders {
			opts = append(opts, transport.WithHTTPHeader(k, v))
		}
		cfg.innerTransport = transport.NewReconnectingHTTP(baseURL, opts...)
		cfg.autoInit = true
		return nil
	}
}

// WithReconnectingWSTransport is like WithWSTransport but automatically re-dials
// if the WebSocket connection drops. Any retry policy configured via WithRetry
// is applied in New() regardless of option ordering.
func WithReconnectingWSTransport(ctx context.Context, url string) Option {
	return func(cfg *clientConfig) error {
		var opts []transport.WSOption
		for k, v := range cfg.wsAuthHeaders {
			opts = append(opts, transport.WithWSHeader(k, v))
		}
		ws, err := transport.NewReconnectingWS(ctx, url, opts...)
		if err != nil {
			return err
		}
		cfg.innerTransport = ws
		cfg.autoInit = true
		return nil
	}
}

// WithRetry records a retry policy applied when an HTTP/WS transport is
// constructed via WithHTTPTransport / WithWSTransport. A zero-value cfg uses
// DefaultRetryConfig(). Ordering relative to WithHTTPTransport / WithWSTransport
// does not matter; the policy is applied in New().
func WithRetry(cfg transport.RetryConfig) Option {
	return func(c *clientConfig) error {
		rc := cfg
		c.retry = &rc
		return nil
	}
}

// WithAutoLogin authenticates the client with an OpenAI API key inside New(),
// immediately after the initialize handshake. It only takes effect when the
// client also performs auto-initialization (i.e. a real transport is wired up
// via WithTransport / WithStdioProcess / WithHTTPTransport / WithWSTransport).
func WithAutoLogin(apiKey string) Option {
	return func(cfg *clientConfig) error {
		cfg.autoLoginAPIKey = apiKey
		return nil
	}
}

// WithMaxThreads sets the maximum number of concurrent live SessionThreads.
// Default is 64. Pass 0 to use the default; pass -1 for unlimited.
func WithMaxThreads(n int) Option {
	return func(cfg *clientConfig) error {
		cfg.maxThreads = n
		return nil
	}
}

type Transport interface {
	Call(context.Context, string, any, any) error
	Notify(context.Context, string, any) error
	SetRequestHandler(RequestHandler)
	Close() error
}
