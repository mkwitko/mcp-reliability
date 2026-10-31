# MCP TypeScript SDK: validation of `doctor`

Date: 2026-10-31. Author: Claude, for the maintainer. Status: evidence from real captures; the SDK source was read only for the PR's docs page.

## 1. Setup and versions

- Baseline: `@modelcontextprotocol/sdk` 1.32.2, the `latest` dist-tag on npm (verified 2026-10-31). Contains no OpenTelemetry code (`grep -ri "opentelemetry\|traceparent"` over its `dist/esm` finds nothing).
- PR build: https://github.com/modelcontextprotocol/typescript-sdk (PR #3115, branch `feat/protocol-instrumentation`), commit `5a073867a783066cc673ff07f7943aebe6165787`, committed 2026-10-30, subject "feat: openTelemetry: false turns off one instance's instrumentation". Packages `@modelcontextprotocol/{core,client,server}` 2.4.0, built with pnpm 10.26.1 (`pnpm install --frozen-lockfile && pnpm -r build` succeeded first try) and packed with `pnpm pack`. Branch state may have moved since; this is an unreleased build.
- OTel: `@opentelemetry/sdk-node` 0.300.0, `exporter-trace-otlp-http` 0.300.0, `api` 1.10.0. Node v24.12.0. Collector `otel/opentelemetry-collector-contrib:0.146.0`. Go side: `examples/go-sdk-streamable-http` on go-sdk v1.8.0.
- Harness: `examples/typescript-sdk-streamable-http/` (`run.sh`). Same scenario as the Python one.

## 2. What the PR does (docs read: `docs/advanced/opentelemetry.md` at the commit above)

- Opt-in is implicit: "Register an OpenTelemetry SDK and they show up." `Client`, `McpServer` and `Server` emit spans and metrics for every request and notification except `notifications/message`. `openTelemetry: false` turns one instance off.
- Without an SDK: "nothing is recorded and nothing is added to `params._meta`."
- Trace context travels in `params._meta` (SEP-414) via the global propagator. The docs warn that the `NodeSDK` default propagator also writes W3C Baggage into `_meta`.
- Scopes `@modelcontextprotocol/client` and `@modelcontextprotocol/server`. Docs say spans do not record tool arguments or results; they do record JSON-RPC error messages and resource URIs.
- Per the docs, a server treats `-32700/-32600/-32601/-32602/-32002` as caller errors, not errors.

## 3. What was emitted

`testdata/typescript-sdk-default.jsonl` (1 line): a single span, `agent step`, from the client. No MCP span on either side; the server process exports nothing.

`testdata/typescript-sdk-pr3115.jsonl` (1 line, 8 spans), shape from parsing the file:

```
INTERNAL "agent step"                      (client)
 CLIENT "tools/call echo|fail|missing"     {mcp.method.name, jsonrpc.request.id, mcp.protocol.version, gen_ai.operation.name, gen_ai.tool.name} (+ error.type on fail, + rpc.response.status_code and error.type on missing)
  SERVER "tools/call echo"                 + network.protocol.name, network.transport
  SERVER "tools/call fail"                 + error.type
  SERVER "tools/call"  (the missing call)  + rpc.response.status_code, no gen_ai.tool.name in the name
ROOT SERVER "server/discover"              (handshake, own trace)
```

The server span for the unknown tool is named `tools/call` without the tool name; the docs say a tool name is kept only once a handler has recognised it. Propagation works: every `tools/call` SERVER span shares the client's trace and is a child of its CLIENT span.

## 4. Doctor output (verbatim)

```
$ go run ./cmd/mcp-reliability doctor testdata/typescript-sdk-default.jsonl
MCP Observability Report
file: testdata/typescript-sdk-default.jsonl
spans: 1

[PASS] no findings

Result: 0 FAIL, 0 WARN, 0 INFO
exit=0
```

```
$ go run ./cmd/mcp-reliability doctor testdata/cross-go-client-typescript-server.jsonl
MCP Observability Report
file: testdata/cross-go-client-typescript-server.jsonl
spans: 7

[PASS] no findings

Result: 0 FAIL, 0 WARN, 0 INFO
exit=0
```

```
$ go run ./cmd/mcp-reliability doctor testdata/cross-typescript-client-go-server.jsonl
MCP Observability Report
file: testdata/cross-typescript-client-go-server.jsonl
spans: 6

[INFO] MCP-PROP-005: 5 MCP SERVER spans are trace roots and the file has spans from 2 services but no MCP CLIENT span: the client side is not instrumented for MCP or does not inject trace context into params._meta
        spans: [7ff5bd339f034700 038c3d1f528f63a7 9abb55590389bb94 38365169111c3ac1 b3ceba6e450a2e87]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 0 FAIL, 0 WARN, 1 INFO
exit=0
```

```
$ go run ./cmd/mcp-reliability doctor testdata/typescript-sdk-pr3115.jsonl
MCP Observability Report
file: testdata/typescript-sdk-pr3115.jsonl
spans: 8

[WARN] MCP-ERR-005: span "tools/call missing" is marked failed (status=Error, error.type present=true) for a JSON-RPC code the semconv says SHOULD NOT be considered an error (caller-side codes -32700/-32600/-32601/-32602/-32002)
        spans: [339a98cfd01a84e5]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-ERR-003: client and server spans for "tools/call" disagree on failure: the server span lacks error.type
        spans: [339a98cfd01a84e5 b9d71531b22d232e]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 0 FAIL, 2 WARN, 0 INFO
exit=0
```

## 5. Classification and what H1 says

| Finding | Verdict | Evidence |
|---|---|---|
| `typescript-sdk-default` "no findings" | **Misleading pass** | The file has no MCP spans, so no rule applies. The correct reading is "SDK 1.32.2 emits nothing". Doctor's message at `rules.go` (MCP-looking spans but no CLIENT/SERVER) only fires when some MCP-looking span exists. Candidate rule or message: "no MCP spans found". HYPOTHESIS, not decided |
| `cross-typescript-client-go-server` MCP-PROP-005 | **True positive** | The 1.32.2 client injects no `_meta.traceparent`; the Go server spans are roots |
| `cross-go-client-typescript-server` "no findings" | Correct but weak | The Go client injects, the 1.32.2 server ignores it and emits no spans; doctor sees only a client side |
| `typescript-sdk-pr3115` MCP-ERR-005 | **True positive** | The #3115 client marks a `-32602` response as ERROR with `error.type`. mcp.md footnote 5 (as encoded in rule MCP-ERR-005) lists it as a caller error. The PR's docs say the server follows this; the client does not |
| `typescript-sdk-pr3115` MCP-ERR-003 | **Consequence of the above** | The server span correctly lacks `error.type` for `-32602`; the disagreement is the client's marking. Same root cause, counted twice |

H1 (an SDK either gives correct, connected traces by default or doctor finds the gap): **held for the published 1.32.2** (no spans, no propagation; PROP-005 shows it cross-SDK, though the all-quiet single-SDK case is not flagged) **and for the #3115 build** (connected traces by default; one real client-side deviation found, the same one as Python SDK 2.3.0). Caveat: #3115 is an open PR; the claims hold for commit `5a07386` only.

## 6. Not tested

Stdio, metrics (`mcp.*.operation.duration`), baggage leakage into `_meta` with the NodeSDK default propagator (the docs warn of it; no wire capture was made), cross-SDK runs with the #3115 build, and the legacy `initialize` handshake of #3115 (`client.mjs --legacy`).
