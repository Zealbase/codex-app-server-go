//go:build ignore

// generate_schema.go runs `codex app-server generate-json-schema` and writes
// the resulting v2 schema bundle into the pinned schema file used by the SDK.
// The output is normalized (keys sorted) so that repeated runs are stable.
//
// Usage:
//
//	go run scripts/generate_schema.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	// Resolve the output path relative to this file's directory.
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		self = "scripts/generate_schema.go"
	}
	scriptsDir := filepath.Dir(self)
	sdkRoot := filepath.Dir(scriptsDir)
	outFile := filepath.Join(sdkRoot, "internal", "protocol", "schema", "codex_app_server_protocol.v2.schemas.json")

	// Write the schema to a temp directory first, then copy the v2 file.
	tmpDir, err := os.MkdirTemp("", "codex-schema-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("codex", "app-server", "generate-json-schema", "--out", tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running codex app-server generate-json-schema: %v\n", err)
		os.Exit(1)
	}

	src := filepath.Join(tmpDir, "codex_app_server_protocol.v2.schemas.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading generated schema: %v\n", err)
		os.Exit(1)
	}

	// Normalize: unmarshal then re-marshal with sorted keys for stability.
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing generated schema JSON: %v\n", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(parsed); err != nil {
		fmt.Fprintf(os.Stderr, "error re-encoding schema: %v\n", err)
		os.Exit(1)
	}
	// json.Encoder does NOT sort keys in Go's standard library — we need a
	// sorted marshal.  Use json.Marshal with a map traversal is also
	// non-deterministic.  Re-encode via json.Marshal which is stable for
	// structs; for arbitrary maps we need a helper.
	//
	// Since Go's encoding/json marshals map keys in sorted order (as of Go
	// 1.12+), we can round-trip through map[string]any to get sorted output.
	normalized, err := marshalSorted(parsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error normalizing schema: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outFile, normalized, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing schema to %s: %v\n", outFile, err)
		os.Exit(1)
	}

	fmt.Printf("Schema written to %s (%d bytes)\n", outFile, len(normalized))
}

// marshalSorted marshals v to indented JSON with map keys sorted.
// Go's encoding/json already sorts map keys, so a simple MarshalIndent suffices.
func marshalSorted(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
