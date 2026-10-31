# Upstream review: olgasafonova/mcp-otel-go

Date: 2026-10-30. Status: research and drafts. Nothing here has been sent. No issue or PR was opened.

Reviewed commit: `2049e1c` (2026-10-11), `mcpotel/` package, go-sdk v1.8.0, otel v1.47.0 (from its `go.mod`).
Compared against: `open-telemetry/semantic-conventions-genai` `docs/gen-ai/mcp.md` and `model/mcp/*.yaml` at `6f98aef` (2026-10-30), and `semantic-conventions` v1.44.0 `docs/general/recording-errors.md`.
Scratch clone and proof tests: `/tmp/claude-504429705/-home-mauricio-oliveira-Documentos-Coding-futuro-mcp-reliability/9c14ea2d-4fa7-4d9e-aeeb-db21c3e3a5c0/scratchpad/mcp-otel-go/mcpotel/` (`zz_probe_test.go`, `zz_inject_test.go`; run with `go test -run 'TestProbe|TestInjectSketch' -v ./mcpotel/`). Both files are scratch only and were not added to this repo.

Repo state: 0 issues ever (`gh issue list --state all` is empty). Recent PRs are all dependabot or CI.

## 0. Summary

| Question | Answer |
|----------|--------|
| (a) Does the server middleware lose the tool name because of `*mcp.CallToolParamsRaw`? | **No. The premise is wrong.** `toolName()` handles both types (`spans.go:66-74`). Tool name is kept. Verified by a running test. |
| (b) Client-side inject? | **Confirmed absent.** Server `Extract` only. |
| (c) Semconv attributes | Sets 6 attributes. Misses 2 Conditionally Required or Recommended ones that matter, and gets 2 behaviours wrong (span status on success, `error.type` values). Details in section 3. |
| (d) Notifications | Instrumented by default. Only the opt-in `DefaultProtocolFilter` drops 4 notification methods. Our research note said "explicitly skips four notification methods"; that is misleading and should be corrected. |

Real gaps worth upstream text: G1 (client inject), G2 (span status Ok on success), G3 (`error.type` and JSON-RPC error classification), G4 (missing recommended attributes). Not a gap: tool name type assertion.

## 1. (a) `CallToolParamsRaw`

Code:

```go
// mcpotel/spans.go:66-74
func toolName(params mcp.Params) string {
	switch p := params.(type) {
	case *mcp.CallToolParamsRaw:
		return p.Name
	case *mcp.CallToolParams:
		return p.Name
	}
	return ""
}
```

The comment above it (`spans.go:63-65`) says the server receives `CallToolParamsRaw` and the client receives `CallToolParams`. The author already knew. `typednil_test.go:70` also builds a `*mcp.ServerRequest[*mcp.CallToolParamsRaw]`.

Proof, run against go-sdk v1.8.0 with an in-memory server (`zz_probe_test.go`), client calls `tools/call` with `_meta.traceparent`:

```
SPAN name="tools/call greet" kind=server trace=4bf92f3577b34da6a3ce929d0e0e4736 parent=00f067aa0ba902b7 remote=true status=Ok/""
    mcp.method.name=tools/call
    gen_ai.tool.name=greet
```

Span name carries `greet`, `gen_ai.tool.name=greet` is set, and the remote parent is linked. So for this library, the failure mode we feared does not occur. Other libraries that assert `*mcp.CallToolParams` on the server side should still be checked; that is a useful doctor fixture (MCP-SEM-002 fires when `gen_ai.tool.name` is missing) but not an issue for this repo.

Action: no issue. If we mention go-sdk's Raw type in docs, cite `spans.go:66-74` as the correct handling.

## 2. (b) No client-side inject

Evidence:
- `grep -rn -E "Inject|SendingMiddleware" --include=*.go` (non-test) returns nothing in `mcpotel/`.
- `Middleware()` returns an `mcp.Middleware` meant for `server.AddReceivingMiddleware` (`middleware.go:150-159`). Package doc says "for MCP servers built with the official go-sdk" (`spans.go:1-2`).
- `metaCarrier.Set` exists (`propagation.go:163-170`) but nothing calls it; it is a no-op on a nil map.
- README has no client section (`grep -n -i client README.md` only hits the `clientInfo` mention).

Feasibility, proven (`zz_inject_test.go`): go-sdk v1.8.0 has `Client.AddSendingMiddleware` (`client.go:1135`) and `Server.AddSendingMiddleware` (`server.go:1845`). A 10-line sending middleware calling `propagator.Inject(ctx, metaCarrier(meta))` and `params.SetMeta(meta)` links server spans to the client trace:

```
SERVER SPAN "server/discover" trace=d307674c...ebc3ab528e9 remoteParent=5506f9cb1c743371
SERVER SPAN "tools/call greet" trace=d307674c...ebc3ab528e9 remoteParent=5506f9cb1c743371
CLIENT trace=d307674c...ebc3ab528e9 span=5506f9cb1c743371
```

Caveat: the sketch injects the ambient span, not a dedicated MCP CLIENT span. A proper PR should start a `SpanKindClient` span named `{method} {target}` and inject that span's context.

### Draft issue (not sent)

Title: Client-side trace context injection (`params._meta`) is not provided

Body:

> Hi, thanks for the library. I was checking how it fits the OTel MCP semantic conventions and found one gap I would like to ask about before sending anything.
>
> `mcpotel.Middleware` extracts `traceparent` from `params._meta` on the server (`mcpotel/propagation.go`, `middleware.go:179`), but there is no counterpart that injects it on the client. The semconv says instrumentation SHOULD inject context into `params._meta` "when creating a request or notification" (`docs/gen-ai/mcp.md`, "Context propagation"), and MCP draft `basic/index.mdx` reserves `traceparent`, `tracestate` and `baggage` there (SEP-414).
>
> Effect: with a go-sdk client and an `mcpotel` server, every server span is a trace root, unless the client is instrumented some other way. I checked that a Go client has no maintained option for this today.
>
> go-sdk v1.8.0 already has what is needed: `Client.AddSendingMiddleware` (and `Server.AddSendingMiddleware` for server-initiated requests such as sampling or elicitation). A sketch that works against v1.8.0, with a server span linked to the client trace:
>
> ```go
> func ClientMiddleware(cfg Config) mcp.Middleware { /* start CLIENT span, then: */
>     meta := params.GetMeta(); if meta == nil { meta = map[string]any{} }
>     propagator.Inject(ctx, metaCarrier(meta))
>     params.SetMeta(meta)
> }
> ```
>
> Would you accept a PR that adds `mcpotel.ClientMiddleware(Config)`, reusing `metaCarrier`, with a `SpanKindClient` span named `{method} {target}`? I would keep the same redaction defaults and the same `Filter` option. Happy to adjust the API to what you prefer.

Evidence lines to attach: `propagation.go:163-170` (Set unused), `spans.go:1-2` (server only), my test output above.

### PR sketch (diff-level)

New file `mcpotel/client.go` (about 60 lines):
- `func ClientMiddleware(cfg Config) mcp.Middleware`, built on the existing `resolve(cfg)`.
- Per call: skip when `r.filter` says so; `target := extractTarget(method, req)` (already handles `*mcp.CallToolParams`, the client type); `ctx, span := r.tracer.Start(ctx, spanName(...), trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(requestAttrs(...)...))`.
- Inject: `meta := params.GetMeta(); if meta == nil { meta = map[string]any{} }; r.propagator.Inject(ctx, metaCarrier(meta)); params.SetMeta(meta)`. Guard params with `isNilRef` as the existing code does (`typednil.go`).
- After `next`: same error recording as the server path (reuse `callErrMsg`, `recordError`).
- Note: `req.GetParams()` on notifications may be a typed nil; `isNilRef` already covers that.

Tests (`mcpotel/client_test.go`): reuse `setupServer` and `connect` helpers; assert client span and server span share a trace id and the server parent is the client span id; assert a `Filter` skip injects nothing; assert existing user-supplied `_meta` keys are preserved (do not replace the map).

Docs: README "Client side" section; fix the package doc "for MCP servers".

Compatibility note for the doctor: once merged, MCP-PROP-001 stops firing for Go client + Go server pairs.

## 3. (c) Attributes versus current `mcp.md`

What the code sets (`spans.go:14-32`, `middleware.go:208-223`): `mcp.method.name`, `mcp.session.id`, `mcp.resource.uri` (scheme only by default), `gen_ai.tool.name`, `gen_ai.prompt.name`, `error.type`. Span name `{method} {target}` follows the convention. Span kind SERVER is correct.

Observed (probe, v1.8.0):

```
tools/call boom   status=Error/"*errors.errorString"   error.type=*errors.errorString
tools/call nope   status=Error/"*jsonrpc2.WireError"   error.type=*jsonrpc2.WireError   (unknown tool, JSON-RPC -32602)
tools/call greet  status=Ok/""
```

| Semconv attribute | Level in mcp.md | mcp-otel-go | Verdict |
|-------------------|-----------------|-------------|---------|
| `mcp.method.name` | Required | set | ok |
| `gen_ai.tool.name` | Cond. Required (tool) | set | ok |
| `gen_ai.prompt.name` | Cond. Required (prompt) | set | ok |
| `mcp.resource.uri` | Cond. Required (resource requests) | set; scheme only by default (deliberate privacy choice) | ok, documented |
| `mcp.session.id` | Recommended | set when present | ok |
| `error.type` | Cond. Required on failure | set, but wrong values (G3) | gap |
| `jsonrpc.request.id` | Cond. Required "when the client executes a request" | not set | gap (G4) |
| `rpc.response.status_code` | Cond. Required if response has an error code | not set | gap (G3) |
| `mcp.protocol.version` | Recommended | not set | gap (G4) |
| `gen_ai.operation.name` | Recommended; SHOULD be `execute_tool` for tool calls | not set | gap (G4) |
| `network.transport`, `network.protocol.*`, `client.address/port` | Recommended | not set | minor |
| Span status on success | Recording errors: "MUST be left unset" | set to `Ok` | gap (G2) |

Our own README rules would flag these: MCP-ERR-001/002 relate to `error.type` and status; no rule currently checks `jsonrpc.request.id` or `mcp.protocol.version`, and for good reason (they are not Required).

### G2. Span status `Ok` on success (strongest gap)

`semantic-conventions` v1.44.0 `docs/general/recording-errors.md:44-45`: "Span Status Code MUST be left unset if the instrumented operation has ended without any errors." `mcp.md` links to that document for span status. Code: `middleware.go:194` does `span.SetStatus(codes.Ok, "")`.

Draft issue (not sent):

> Title: Successful spans have status `Ok`; semconv says it MUST stay unset
>
> `Middleware` calls `span.SetStatus(codes.Ok, "")` on success (`mcpotel/middleware.go:194`). The semantic conventions for MCP defer to the general "Recording errors" document, which says the span status code MUST be left unset if the operation ended without errors (`docs/general/recording-errors.md` in semantic-conventions v1.44.0, "Recording errors on spans"). Setting `Ok` also stops downstream processors from distinguishing "instrumentation did not decide" from "success", and some backends treat it as a user override.
>
> Reproduction against v1.8.0 and the in-memory transport: a successful `tools/call` yields `Status.Code=Ok`. Proposed fix: remove the `else` branch; keep `Error` handling as is. I found no existing test that asserts `codes.Ok`, but I would add one asserting `Unset`. I can send the PR.

PR sketch: `middleware.go`: delete the `else { span.SetStatus(codes.Ok, "") }` branch (lines 193-195). Add a test asserting `codes.Unset` on success (no current test asserts `codes.Ok`).

### G3. `error.type` values and JSON-RPC error classification

mcp.md, `error.type` note [1]:
- When a `CallToolResult` has `isError: true`, `error.type` SHOULD be `tool_error`.
- When the response has a JSON-RPC error code that classifies as an error, `error.type` SHOULD be that code as a string; `rpc.response.status_code` carries the same.
- `-32700, -32600, -32601, -32602, -32002` SHOULD NOT be considered errors on the server span (server `[5]`).

Observed: tool handler error gives `error.type=*errors.errorString`, unknown tool (`-32602`) gives `Error` status and `error.type=*jsonrpc2.WireError`. `extractToolError` falls back to `"tool_error"` only when `GetError()` is nil (`spans.go:101-110`). The default redaction (`errorTypeName`, `middleware.go:245-247`) is a Go type name, which is neither of the two spec values and is a Go-internal type for JSON-RPC failures. `jsonrpc.Error` (alias of `jsonrpc2.WireError`) is exported in go-sdk v1.8.0 (`jsonrpc/jsonrpc.go:21`), so the code can be read with `errors.As`.

Caveat: the privacy-first default is a deliberate maintainer choice (README, `RedactError`). The proposal keeps it for the span status description, and only changes `error.type`, which is low cardinality by definition.

Draft issue (not sent):

> Title: `error.type` records Go type names; semconv asks for `tool_error` or the JSON-RPC code
>
> Observed on go-sdk v1.8.0: a tool handler returning an error produces `error.type=*errors.errorString`; calling an unknown tool produces status `Error` and `error.type=*jsonrpc2.WireError`. In `docs/gen-ai/mcp.md` (note on `error.type`) the values are `tool_error` for `isError: true` results, and the JSON-RPC error code string for JSON-RPC errors; `-32602` and the other "caller error" codes should not be marked as errors on a server span. Suggest: (1) `isError` results get `error.type=tool_error`, regardless of `GetError()`; (2) when `errors.As(err, *jsonrpc.Error)` matches, set `error.type` and `rpc.response.status_code` to `strconv.Itoa(code)`, and leave status unset for the five caller-error codes; (3) keep `RedactError` for the status description only.

PR sketch: in `spans.go`, `extractToolError` returns `"tool_error"` unconditionally for `IsError`; in `middleware.go`, `callErrMsg` gets a branch before `redact(err)`: `var je *jsonrpc.Error; if errors.As(err, &je) { code := strconv.Itoa(int(je.Code)); ... }`; `recordError` writes `rpc.response.status_code`. Add a `classifyJSONRPC(code)` helper returning isError bool. Tests: three probes from `zz_probe_test.go`.

### G4. Missing recommended attributes

Smallest useful additions, each cheap: `mcp.protocol.version` (from the session's negotiated version or `_meta["io.modelcontextprotocol/protocolVersion"]` in the 2026-07-28 wire), `gen_ai.operation.name=execute_tool` on `tools/call`, and `jsonrpc.request.id` when the request has an id (`req.GetRawID()` style accessor if present in v1.8.0; I did not verify the accessor name, so check before the PR). Present these as one issue with the table above, low priority, because all are Recommended or Conditionally Required rather than Required. Suggested wording: "Not blocking, but `mcp.md` lists these; happy to send a small PR for any you want."

## 4. (d) Notifications

Behaviour, verified with a client `NotifyProgress` call (probe):

```
SPAN name="notifications/progress" kind=server trace=185802cc... parent=0000000000000000 remote=false status=Ok/""
    mcp.method.name=notifications/progress
```

- The receiving middleware sees notifications and instruments them by default, with SERVER kind, correct `mcp.method.name`, and `_meta` extraction through the same carrier.
- `DefaultProtocolFilter` (`middleware.go:270-329`) is opt-in (`Config.Filter`). It drops nine methods: `notifications/initialized`, `notifications/cancelled`, `notifications/progress`, `notifications/roots/list_changed`, `ping`, `tools/list`, `resources/list`, `resources/templates/list`, `prompts/list`. Four of these are notifications. README line 201 says every method is instrumented by default.
- Correction for `docs/research/instrumentation-libraries.md` "Practical gaps" item 4: "mcp-otel-go explicitly skips four notification methods" should read "skips four notification methods only when the opt-in `DefaultProtocolFilter` is configured".
- Real notification gap: no sender side. Notifications cannot carry context from this library because there is no client or sending middleware (G1). Semconv says inject "when creating a request or notification". The G1 PR covers it if the sending middleware also handles notification params (same `GetMeta/SetMeta`).
- Server-initiated requests (sampling, elicitation, roots/list) are also uninstrumented: no `Client.AddReceivingMiddleware` equivalent is offered, only `Server.AddReceivingMiddleware`. The `ClientMiddleware` in G1 should be documented for `Server.AddSendingMiddleware` too, and a `Middleware` use on `Client.AddReceivingMiddleware` is already possible since the type is `mcp.Middleware`; this is untested in the repo.

## 5. Corrections for our own docs

1. `docs/research/instrumentation-libraries.md` "Practical gaps" list item 4 (notifications, around line 167): see section 4 above.
2. Section 2.1 gap list ("no `jsonrpc.request.id`, no `rpc.response.status_code`, no `mcp.protocol.version`" as HYPOTHESIS): now CONFIRMED by the `Attr*` constants in `spans.go:14-32` and by the probe output. Add `gen_ai.operation.name` and `network.transport` to the list.
3. Add G2 (span status `Ok`) and G3 (`error.type` values), which the research did not list.
4. Any statement that the Raw/non-Raw params type loses the tool name for mcp-otel-go is false.

## 6. Order of upstream work

1. Open one issue for G1 (client inject) first, ask before coding. It is the roadmap M3 item 1 and the maintainer has no open issues, so the first contact sets the tone.
2. G2 as a separate small issue; it is a one-line fix with a normative citation.
3. G3 and G4 as one combined issue, lower priority.

Roadmap M3 text says "mcp-otel-go: client-side inject (currently server extract only)": confirmed accurate.
