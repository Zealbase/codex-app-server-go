package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
)

// StdioTransport implements Transport over a ReadWriteCloser using jrpc2 for
// framing and request/response management. The framing discipline is
// newline-delimited JSON (channel.Line), which matches the Codex app-server
// protocol.
type StdioTransport struct {
	client *jrpc2.Client

	requests chan *Request
	notes    chan Notification
	done     chan struct{}
	doneOnce sync.Once

	closeOnce sync.Once
}

// NewStdio creates a StdioTransport from an existing ReadWriteCloser.
// The caller retains ownership of rwc; Close will close it via the jrpc2 client.
func NewStdio(rwc io.ReadWriteCloser) *StdioTransport {
	t := &StdioTransport{
		requests: make(chan *Request, 32),
		notes:    make(chan Notification, 32),
		done:     make(chan struct{}),
	}

	opts := &jrpc2.ClientOptions{
		// OnNotify is called for every server notification (no id field).
		OnNotify: func(req *jrpc2.Request) {
			var params json.RawMessage
			// jrpc2 Request.UnmarshalParams into *json.RawMessage never errors.
			_ = req.UnmarshalParams(&params)
			n := Notification{
				Method: req.Method(),
				Params: params,
			}
			select {
			case t.notes <- n:
			case <-t.done:
			}
		},
		// OnCallback is called for server-initiated requests (has an id field).
		// The handler blocks until the caller sends a reply via *Request.Reply or
		// *Request.ReplyError, then returns the result to jrpc2 for encoding.
		OnCallback: func(ctx context.Context, req *jrpc2.Request) (any, error) {
			var params json.RawMessage
			_ = req.UnmarshalParams(&params)

			replyCh := make(chan replyResult, 1)
			inbound := &Request{
				replyCh: replyCh,
				method:  req.Method(),
				params:  params,
				ctx:     ctx,
			}

			select {
			case t.requests <- inbound:
			case <-t.done:
				return nil, ErrClosed
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			// Block until the caller calls Reply or ReplyError.
			select {
			case rr := <-replyCh:
				if rr.isErr {
					// Return a *jrpc2.Error so that jrpc2 preserves the
					// caller-supplied code instead of mapping it to SystemError.
					return nil, jrpc2.Errorf(jrpc2.Code(rr.code), "%s", rr.msg)
				}
				return rr.result, nil
			case <-t.done:
				return nil, ErrClosed
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		// OnStop is called once when the client shuts down.
		OnStop: func(_ *jrpc2.Client, _ error) {
			t.doneOnce.Do(func() { close(t.done) })
		},
	}

	// channel.Line frames records as newline-terminated strings — matches the
	// Codex server's newline-delimited JSON transport.
	// Wrap the reader with versionFixer so that responses missing the
	// "jsonrpc":"2.0" field (as emitted by the Codex app-server binary) are
	// accepted by jrpc2's strict version check.
	ch := channel.Line(versionFixerReader(rwc), rwc)
	t.client = jrpc2.NewClient(ch, opts)
	return t
}

// Call sends a JSON-RPC request and decodes the result into result.
func (t *StdioTransport) Call(ctx context.Context, method string, params any, result any) error {
	select {
	case <-t.done:
		return ErrClosed
	default:
	}
	if result == nil {
		// jrpc2.CallResult(ctx, method, params, nil) panics or errors when the
		// server returns a null/empty result. Use a discard sink instead.
		var discard json.RawMessage
		return t.client.CallResult(ctx, method, params, &discard)
	}
	return t.client.CallResult(ctx, method, params, result)
}

// Notify sends a JSON-RPC notification (no reply expected).
func (t *StdioTransport) Notify(ctx context.Context, method string, params any) error {
	select {
	case <-t.done:
		return ErrClosed
	default:
	}
	return t.client.Notify(ctx, method, params)
}

// Requests returns a channel of server-initiated requests.
func (t *StdioTransport) Requests() <-chan *Request { return t.requests }

// Notifications returns a channel of server notifications.
func (t *StdioTransport) Notifications() <-chan Notification { return t.notes }

// Done is closed when the transport is permanently terminated.
func (t *StdioTransport) Done() <-chan struct{} { return t.done }

// Close shuts down the transport.
func (t *StdioTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		err = t.client.Close()
		t.doneOnce.Do(func() { close(t.done) })
	})
	return err
}

// versionFixerReader wraps an io.Reader and injects `"jsonrpc":"2.0"` into
// every JSON object line that lacks it. The Codex app-server binary omits that
// field from responses; jrpc2 rejects any message without it.
func versionFixerReader(r io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			line := scanner.Bytes()
			line = injectJSONRPCVersion(line)
			_, err := pw.Write(append(line, '\n'))
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		pw.CloseWithError(scanner.Err())
	}()
	return pr
}

var jsonrpcVersionField = []byte(`"jsonrpc":"2.0"`)

// injectJSONRPCVersion inserts `"jsonrpc":"2.0",` after the opening `{` of a
// JSON object if the field is not already present.
func injectJSONRPCVersion(line []byte) []byte {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return line
	}
	if bytes.Contains(trimmed, jsonrpcVersionField) {
		return line
	}
	// Insert after the opening brace.
	result := make([]byte, 0, len(trimmed)+len(jsonrpcVersionField)+1)
	result = append(result, '{')
	result = append(result, jsonrpcVersionField...)
	if len(trimmed) > 1 && trimmed[1] != '}' {
		result = append(result, ',')
	}
	result = append(result, trimmed[1:]...)
	return result
}
