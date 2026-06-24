# Deployed Server Test Gap

Date: 2026-06-22

## Current state

The deployed Helm release at `codex-server.localdev.com` is healthy and now runs with:

- `--dangerously-bypass-approvals-and-sandbox`

The chart exposes the Codex app-server over native WebSocket transport:

- `ws://codex-server.localdev.com`

## Gap

The public `sdk/codex-go` client and its e2e suite are still stdio-only.

That means the deployed server cannot yet be used as a drop-in target for:

- `sdk/codex-go/tests/e2e/...`

because those tests currently launch or attach to a stdio JSON-RPC process, while the deployed server expects WebSocket connections.

## What was added now

To at least use the deployed environment in the repo test surface today, the e2e Makefile now includes:

- `make deployed-smoke`

This verifies:

1. `GET /healthz`
2. `GET /readyz`

against `CODEX_SERVER_BASE` (default `http://codex-server.localdev.com`).

## Required follow-up for full remote e2e

One of these is needed before the SDK e2e suite can run against the deployed server:

1. public WebSocket transport support in `sdk/codex-go`, or
2. a supported stdio<->WebSocket bridge used by the test harness
