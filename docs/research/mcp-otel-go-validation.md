# mcp-otel-go validation (E1 / H1)

Date: 2026-10-30. Harness: `examples/mcp-otel-go/` (own Go module; `cd examples/mcp-otel-go && go run . -role server`, `go run . -role client [-inject]`; full run `examples/mcp-otel-go/run.sh`).

## Versions

| Component | Version |
|-----------|---------|
| `github.com/olgasafonova/mcp-otel-go` | v0.2.1 (latest tag; `bcbc1d9`) |
| `github.com/modelcontextprotocol/go-sdk` | v1.8.0 |
| `go.opentelemetry.io/otel` (+ sdk, otlptracehttp) | v1.47.0 |
| Go toolchain | go1.27.3 |
| Collector | `otel/opentelemetry-collector-contrib:0.146.0`, fileexporter, own compose project `col-mcpotel` on port 4320 |

Server: `mcpotel.Middleware(mcpotel.Config{ServiceName: "mcpotel-server"})`, default options, tools `echo` (ok), `fail` (`IsError: true`), and `missing` (not registered, so JSON-RPC -32602). Client: plain go-sdk client, root span "agent step", no MCP middleware, global tracer provider and W3C TraceContext+Baggage propagator set, OTLP/HTTP export. Scenario (b) adds only the 10-line sending middleware from `docs/upstream/mcp-otel-go.md` (`-inject`).

## Raw span shape (scenario a, default)

All server spans are SERVER kind, scope `github.com/olgasafonova/mcp-otel-go/mcpotel`, service `mcpotel-server`. The client emitted exactly one span, `agent step` (INTERNAL, service `mcpotel-client`), because the library has no client side.

| Span name | Status | Attributes |
|-----------|--------|------------|
| `server/discover` | Ok | `mcp.method.name`, `mcp.session.id` |
| `initialize` | Ok | `mcp.method.name`, `mcp.session.id` |
| `notifications/initialized` | Ok | `mcp.method.name`, `mcp.session.id` |
| `tools/call echo` | Ok | `mcp.method.name`, `mcp.session.id`, `gen_ai.tool.name=echo` |
| `tools/call fail` | Error, message `tool_error` | same + `error.type=tool_error` |
| `tools/call missing` | Error, message `*jsonrpc2.WireError` | same + `error.type=*jsonrpc2.WireError` |

Differences from the upstream doc's expectations (`docs/upstream/mcp-otel-go.md` section 3, probed at `2049e1c`): status Ok on success confirmed (G2). For an `isError` tool result, `error.type` is `tool_error` in v0.2.1 (the doc expected a Go type name for handler errors; that case, a handler returning a Go error, was not exercised here). For an unknown tool the Go type name `*jsonrpc2.WireError` is confirmed (G3), with status Error for what is a -32602 caller error. Not set: `gen_ai.operation.name`, `rpc.response.status_code`, `jsonrpc.request.id`, `mcp.protocol.version`. Every notification and discovery span is instrumented (no filter by default).

## H1 verdict (manual, by trace id)

Scenario (a), default, client trace `123bc8113516a547018a005578c58b61`. Server spans, none sharing it:

| Server span | Trace id | Parent |
|-------------|----------|--------|
| `server/discover` | `c7841f67b00e37a06d9cdff527b85081` | none |
| `initialize` | `b7befbb5a6935915f9aa665ba86bd97e` | none |
| `notifications/initialized` | `65f4820a37f531def3ce9b1d20eaa88e` | none |
| `tools/call echo` | `7dc3e5dc096cbcf37fb14822399af544` | none |
| `tools/call fail` | `8c7f40437c66e9bf38fadf443fce6493` | none |
| `tools/call missing` | `46c1b1d8128fcef9b2290ed1ff48f75b` | none |

**H1 reproduced for mcp-otel-go v0.2.1 with go-sdk v1.8.0 in a default setup**: 6 of 6 server spans are trace roots, in 6 distinct traces, none in the client's trace. Caveat: this is the documented design (server-side extract only), so a go-sdk client needs its own inject. The finding is "default Go client plus this server loses propagation", not "the library mis-extracts".

Scenario (b), client with the inject sketch, client trace `06c5d14b87c5ee7d5fd711464f695dba`: all 6 server spans have that trace id and parent `40d4477dbaeec174` (the `agent step` span). The server-side extract works, and the missing piece is exactly the client inject (G1). The sketch injects the ambient span, so server spans are children of "agent step", not of an MCP CLIENT span.

## Doctor output (verbatim)

`go run ./cmd/mcp-reliability doctor testdata/mcp-otel-go-default.jsonl`

```
MCP Observability Report
file: testdata/mcp-otel-go-default.jsonl
spans: 7

[FAIL] MCP-ERR-004: span "server/discover" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [2b35f8b0b8c0a741]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "initialize" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [3f9f1f6d3115c8d5]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "notifications/initialized" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [e031a68f9bdc2040]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "tools/call echo" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [8b9879f295a56467]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[INFO] MCP-SEM-004: tools/call span "tools/call echo" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [8b9879f295a56467]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[INFO] MCP-SEM-004: tools/call span "tools/call fail" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [f5b565388059e868]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[INFO] MCP-SEM-004: tools/call span "tools/call missing" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [b2936bd6e9f9d8c4]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 4 FAIL, 0 WARN, 3 INFO
```

`go run ./cmd/mcp-reliability doctor testdata/mcp-otel-go-client-inject.jsonl`

```
MCP Observability Report
file: testdata/mcp-otel-go-client-inject.jsonl
spans: 7

[FAIL] MCP-ERR-004: span "server/discover" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [c10c867a2ef69510]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "initialize" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [907bca426f6645b3]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "notifications/initialized" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [e9b9da533d9ba84e]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[FAIL] MCP-ERR-004: span "tools/call echo" has status Ok; status MUST be left Unset when the operation ends without errors
        spans: [70f84710d287b95e]
        ref: https://opentelemetry.io/docs/specs/semconv/general/recording-errors/
[INFO] MCP-SEM-004: tools/call span "tools/call echo" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [70f84710d287b95e]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[INFO] MCP-SEM-004: tools/call span "tools/call fail" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [7f3b4279441a64c0]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[INFO] MCP-SEM-004: tools/call span "tools/call missing" lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)
        spans: [979501dfbe8d37a2]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 4 FAIL, 0 WARN, 3 INFO
```

Both exit 1 (FAIL present). `doctor -json` counts are identical for the two fixtures: `MCP-ERR-004` 4, `MCP-SEM-004` 3.

## True positives versus rule gaps

True positives:
- MCP-ERR-004 x4 per fixture (`server/discover`, `initialize`, `notifications/initialized`, `tools/call echo` have status Ok). Each is a real G2 violation of the Recording Errors MUST. Fires equally in both scenarios, as expected.
- MCP-SEM-004 x3 (INFO) on the three `tools/call` spans, missing `gen_ai.operation.name=execute_tool`. Real, Recommended only (G4).

Rule gaps (doctor missed a real problem):
1. **Scenario (a) propagation loss is not detected.** MCP-PROP-001 pairs a root SERVER span with a CLIENT MCP span of the same call. A client with no MCP instrumentation produces no CLIENT span, so there is nothing to pair with and the 6 orphan roots pass silently. Doctor says nothing about H1 on the fixture where H1 is true. A candidate rule: SERVER MCP spans that are trace roots with no `mcp.*` CLIENT span anywhere in the file, reported at INFO ("no CLIENT span and no parent: client may not inject"). Risk: legitimately root-server traffic from non-instrumented clients, so INFO at most.
2. **-32602 recorded as failure is not detected.** MCP-ERR-005 needs a parsed JSON-RPC code (`rpc.response.status_code`), which the library does not set. `tools/call missing` has status Error and `error.type=*jsonrpc2.WireError` for a caller-error code; doctor passes it. A fallback could match `error.type` containing `WireError`/`jsonrpc` (a Go-specific heuristic) but cannot recover the code.
3. No rule flags a Go type name in `error.type` as non-conformant (value not `tool_error`, not a numeric code, not `_OTHER` or an exception class). MCP-ERR-003 is about CLIENT/SERVER pair consistency and does not apply with no CLIENT span.

No false FAILs in either fixture.

## Fixture rows for `TestFixtures`

| Fixture | Rules (from `doctor -json`) |
|---------|------------------------------|
| `mcp-otel-go-default` | `MCP-ERR-004`: 4, `MCP-SEM-004`: 3 |
| `mcp-otel-go-client-inject` | `MCP-ERR-004`: 4, `MCP-SEM-004`: 3 |
