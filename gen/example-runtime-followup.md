# Example Runtime Follow-Up

Date: 2026-06-22

## Issue

Live verification of `examples/sdk/codex-go-usage/existing-app-server-multi-agent-search` showed that the public helper:

- `WaitForStructuredOutput`

can surface a JSON value that does not match the caller's requested target shape, even when the example supplies an explicit `OutputSchema`.

Observed failure:

```text
wait for structured output: decode structured output: json: cannot unmarshal object into Go value of type []main.resultEntry
```

## Why it matters

The example requested a top-level JSON array (`[]resultEntry`), but the helper returned a JSON object candidate first. That means one of these is true:

1. the runtime produced schema-nonconforming output and the helper exposed it directly, or
2. the helper selected an enclosing/object wrapper candidate before the schema-target payload.

In either case, a caller that relies only on `WaitForStructuredOutput` can still fail on a real run even though the turn completed successfully.

## Additional live blockers seen on 2026-06-22

Two environment/runtime failures also showed up during verification:

1. forcing `CODEX_MODEL=gpt-5.1` caused provider `400` failures on the connected runtime because that model was not available there
2. after removing the forced model, the app-server still returned `usageLimitExceeded`, so no assistant output was generated for the turn

Later verification against the deployed WebSocket app-server progressed further:

- client initialization succeeded
- `thread/start` succeeded
- `turn/start` succeeded

but the turn still failed before structured output was delivered with:

```text
stream disconnected before completion: stream closed before response.completed
```

That indicates the current blocker on the deployed WS path is now a mid-turn stream termination, not client startup or schema handling.

These are separate from the helper-selection issue above. They block end-to-end example verification even when the example code path is otherwise correct.

## Example mitigation

The example now uses:

1. `Events()` for live item/turn capture
2. `WaitForTurn`-style reconciliation through `thread/read`
3. local JSON extraction from event payloads, turn payloads, and message text

This keeps the example on public APIs while avoiding exclusive dependence on `WaitForStructuredOutput`.

## Suggested SDK follow-up

- tighten structured-output candidate selection so it prefers the schema-target payload over enclosing JSON objects
- optionally expose the raw structured payload bytes alongside decode errors for easier caller recovery/debugging
