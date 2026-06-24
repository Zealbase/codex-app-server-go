package transport

import (
	"context"
	"errors"
	"io"
)

// Reconnect replaces the current session with a fresh stream from opener.
func (t *JSONRPCTransport) Reconnect(ctx context.Context) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrClosed
	}
	opener := t.opener
	if opener == nil {
		t.mu.Unlock()
		return ErrReconnectUnsupported
	}
	t.mu.Unlock()

	conn, err := opener(ctx)
	if err != nil {
		return err
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		_ = conn.Close()
		return ErrClosed
	}
	oldConn := t.conn
	t.conn = conn
	t.nextID = 0
	pending := t.flushPendingLocked()
	oldCancel := t.cancel
	t.sessionID++
	sid := t.sessionID
	sctx, cancel := context.WithCancel(context.Background())
	t.sessionCtx = sctx
	t.cancel = cancel
	t.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}
	if oldConn != nil {
		_ = oldConn.Close()
	}
	for _, ch := range pending {
		select {
		case ch <- callResult{err: ErrDisconnected}:
		default:
		}
	}
	go t.readLoop(conn, sid, sctx)
	return nil
}

// NewReconnecting opens the first stream via opener and allows Reconnect.
func NewReconnecting(ctx context.Context, opener Opener) (*JSONRPCTransport, error) {
	if opener == nil {
		return nil, errors.New("opener is required")
	}
	conn, err := opener(ctx)
	if err != nil {
		return nil, err
	}
	return newTransport(conn, opener), nil
}

// OpenStdio wraps stdio pipes into a reconnect-capable opener-friendly stream.
func OpenStdio(stdin io.ReadCloser, stdout io.WriteCloser) io.ReadWriteCloser {
	return Combine(stdin, stdout)
}
