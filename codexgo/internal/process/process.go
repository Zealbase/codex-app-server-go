package process

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ManagedProcess wraps an os.Cmd and provides a 3-stage shutdown:
//  1. Close the process's stdin (EOF signal)
//  2. SIGTERM after ShutdownGrace (default 5s)
//  3. SIGKILL after KillGrace (default 2s)
type ManagedProcess struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	done     chan struct{}
	doneOnce sync.Once

	waitErr   error
	waitErrMu sync.Mutex

	ShutdownGrace time.Duration
	KillGrace     time.Duration
}

// Start spawns the process and returns stdin writer + stdout reader for transport wiring.
func Start(ctx context.Context, binaryPath string, args []string, extraEnv []string, workDir string) (*ManagedProcess, io.WriteCloser, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}

	p := &ManagedProcess{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		done:          make(chan struct{}),
		ShutdownGrace: 5 * time.Second,
		KillGrace:     2 * time.Second,
	}

	go func() {
		err := cmd.Wait()
		p.waitErrMu.Lock()
		p.waitErr = err
		p.waitErrMu.Unlock()
		p.doneOnce.Do(func() { close(p.done) })
	}()

	return p, stdin, stdout, nil
}

// Wait blocks until the process exits. Returns the exit error (nil on success).
func (p *ManagedProcess) Wait() error {
	<-p.done
	p.waitErrMu.Lock()
	defer p.waitErrMu.Unlock()
	return p.waitErr
}

// Done returns a channel closed when the process exits.
func (p *ManagedProcess) Done() <-chan struct{} {
	return p.done
}

// Shutdown runs 3-stage shutdown: EOF → SIGTERM → SIGKILL.
// It is safe to call multiple times.
func (p *ManagedProcess) Shutdown(ctx context.Context) error {
	// Stage 1: close stdin to signal EOF.
	_ = p.stdin.Close()
	if p.waitWithin(ctx, p.ShutdownGrace) {
		return nil
	}

	// Stage 2: SIGTERM.
	_ = p.signal(syscall.SIGTERM)
	if p.waitWithin(ctx, p.KillGrace) {
		return nil
	}

	// Stage 3: SIGKILL.
	_ = p.signal(syscall.SIGKILL)
	<-p.done
	return nil
}

// waitWithin blocks until the process exits, the timeout elapses, or ctx is
// cancelled. Returns true if the process exited.
func (p *ManagedProcess) waitWithin(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-p.done:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (p *ManagedProcess) signal(sig os.Signal) error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Signal(sig)
}
