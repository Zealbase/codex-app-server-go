package process

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"
)

// TestEchoRoundTrip spawns `cat` and verifies stdin→stdout piping works.
func TestEchoRoundTrip(t *testing.T) {
	ctx := context.Background()
	p, stdin, stdout, err := Start(ctx, "/bin/cat", nil, nil, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Shutdown(ctx)

	if _, err := io.WriteString(stdin, "hello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if line != "hello\n" {
		t.Fatalf("got %q, want %q", line, "hello\n")
	}
}

// TestShutdownEOF verifies closing stdin (EOF) lets `cat` exit during stage 1.
func TestShutdownEOF(t *testing.T) {
	ctx := context.Background()
	p, _, _, err := Start(ctx, "/bin/cat", nil, nil, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.Shutdown(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return within 5s")
	}

	select {
	case <-p.Done():
	default:
		t.Fatal("process did not exit after Shutdown")
	}
}

// TestShutdownSIGTERM verifies a process that ignores EOF is killed by signal.
func TestShutdownSIGTERM(t *testing.T) {
	ctx := context.Background()
	// `sleep` ignores stdin EOF, so stage 1 won't stop it; SIGTERM must.
	p, _, _, err := Start(ctx, "/bin/sleep", []string{"60"}, nil, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	p.ShutdownGrace = 200 * time.Millisecond
	p.KillGrace = 200 * time.Millisecond

	start := time.Now()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("Shutdown took too long: %v", elapsed)
	}
}
