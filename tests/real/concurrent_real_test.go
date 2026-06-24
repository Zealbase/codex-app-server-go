package real_test

import (
	"context"
	"os"
	"testing"
	"time"

	codexgo "github.com/zealbase/codex-app-server-go"
	"golang.org/x/sync/errgroup"
)

func TestReal_ConcurrentThreads(t *testing.T) {
	skipIfNoEndpoint(t)
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < 3; i++ {
		g.Go(func() error {
			thread, err := client.StartThread(gctx, codexgo.WithThreadModel(realModel()))
			if err != nil {
				return err
			}
			defer thread.Close()
			_, err = thread.Run(gctx, "say ok")
			return err
		})
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent threads: %v", err)
	}
}

func TestReal_MaxThreadsSemaphore(t *testing.T) {
	skipIfNoEndpoint(t)
	endpoint := os.Getenv("CODEX_REAL_ENDPOINT")

	// Build a client that respects CODEX_TRANSPORT, same as newRealClient.
	var opts []codexgo.Option
	if key := os.Getenv("CODEX_API_KEY"); key != "" {
		if os.Getenv("CODEX_TRANSPORT") == "ws" {
			opts = append(opts, codexgo.WithWSBearerToken(key))
		} else {
			opts = append(opts, codexgo.WithHTTPBearerToken(key))
		}
	}
	if os.Getenv("CODEX_TRANSPORT") == "ws" {
		dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		opts = append(opts, codexgo.WithWSTransport(dialCtx, endpoint))
	} else {
		opts = append(opts, codexgo.WithHTTPTransport(endpoint))
	}
	opts = append(opts, codexgo.WithMaxThreads(1))

	client, err := codexgo.New(opts...)
	if err != nil {
		t.Fatalf("New (maxThreads=1): %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First thread takes the only slot.
	first, err := client.StartThread(ctx, codexgo.WithThreadModel(realModel()))
	if err != nil {
		t.Fatalf("first StartThread: %v", err)
	}

	secondStarted := make(chan struct{})
	go func() {
		second, err := client.StartThread(ctx, codexgo.WithThreadModel(realModel()))
		if err == nil {
			second.Close()
		}
		close(secondStarted)
	}()

	// The second StartThread must block until the first releases its slot.
	select {
	case <-secondStarted:
		t.Fatal("second StartThread did not block while slot was held")
	case <-time.After(500 * time.Millisecond):
		// expected: still blocked
	}

	first.Close() // release the slot

	select {
	case <-secondStarted:
		// expected: second now proceeds
	case <-time.After(8 * time.Second):
		t.Fatal("second StartThread did not unblock after first closed")
	}
}
