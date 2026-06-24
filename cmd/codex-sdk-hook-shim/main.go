// codex-sdk-hook-shim bridges a Codex hook invocation to a hookbridge.Listener
// via a Unix domain socket.
//
// Usage:
//
//	codex-sdk-hook-shim [--socket /path/to/socket]
//
// The hook payload is read from stdin (JSON). The response is written to stdout.
// If CODEX_HOOK_SOCKET is set, it is used as the default socket path.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	socket := flag.String("socket", os.Getenv("CODEX_HOOK_SOCKET"), "path to hook bridge Unix socket")
	flag.Parse()

	if *socket == "" {
		fmt.Fprintln(os.Stderr, "codex-sdk-hook-shim: socket path required (--socket or CODEX_HOOK_SOCKET)")
		os.Exit(1)
	}

	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-sdk-hook-shim: read stdin: %v\n", err)
		os.Exit(1)
	}

	// The listener reads one newline-delimited request per connection, so the
	// payload must be a single line terminated by a newline.
	payload = append(bytes.TrimRight(payload, "\n"), '\n')

	conn, err := net.Dial("unix", *socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-sdk-hook-shim: dial %s: %v\n", *socket, err)
		os.Exit(1)
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		fmt.Fprintf(os.Stderr, "codex-sdk-hook-shim: write payload: %v\n", err)
		os.Exit(1)
	}

	// Signal EOF to the server so it knows the payload is complete.
	if uc, ok := conn.(*net.UnixConn); ok {
		_ = uc.CloseWrite()
	}

	response, err := io.ReadAll(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-sdk-hook-shim: read response: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(response); err != nil {
		fmt.Fprintf(os.Stderr, "codex-sdk-hook-shim: write stdout: %v\n", err)
		os.Exit(1)
	}
}
