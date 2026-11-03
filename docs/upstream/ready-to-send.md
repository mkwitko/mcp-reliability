# Ready-to-send upstream texts (NOT sent)

Final bodies prepared 2026-11-03 from the research in this directory. Duplicate check done the same day (python-sdk #3174 closed, #3715 open and related; openllmetry #4374 open; mcp-otel-go has no issues; conformance has nothing on traceparent; typescript-sdk PR #3115 has no comment on -32602). Nothing below has been posted. Sending needs the maintainer of this repo.

## python-sdk issue: client span ignores isError

With `mcp==2.3.0`, the client span created in `shared/jsonrpc_dispatcher.py` sets ERROR status and `error.type` for a JSON-RPC error response (the #3174 fix), but not for a successful response whose `CallToolResult.isError` is true. The server span for the same call does set `error.type=tool_error` (`server/_otel.py`), so the two ends of one call disagree.

The OTel GenAI semantic conventions for MCP (`docs/gen-ai/mcp.md`, note on `error.type`) say instrumentation SHOULD set `error.type` to `tool_error` when the tool result reports an error.

**Reproduction** (Python client and server, Streamable HTTP, default OTel setup):
```
git clone https://github.com/mkwitko/mcp-reliability && cd mcp-reliability
examples/python-sdk-streamable-http/run.sh      # needs docker, uv, go
go run ./cmd/mcp-reliability doctor testdata/python-sdk-default.jsonl
```
Output includes:
```
[WARN] MCP-ERR-003: client and server spans for "tools/call fail" disagree on failure: the client span lacks error.type
```
Captured trace and write-up: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/python-sdk-v2-validation.md

**Suggested change:** after the response is parsed in the dispatcher, if the result is a `CallToolResult` with `isError`, set `error.type="tool_error"` and status ERROR on the client span, matching `server/_otel.py`. Happy to send a PR if you agree with the direction.

## python-sdk issue: caller-error codes marked as span failures

Since 2.3.0 (#3174) every JSON-RPC error response marks the client span ERROR with `error.type` set to the code. The OTel GenAI semantic conventions for MCP (`docs/gen-ai/mcp.md`, footnote on `error.type`) say the codes `-32700`, `-32600`, `-32601`, `-32602` and `-32002` "indicate that the caller sent a request the receiver could not serve and SHOULD NOT be considered errors"; only other codes should be. `rpc.response.status_code` should still carry the code.

The most common case is calling an unknown tool: the Go SDK server (and the TypeScript SDK) answer `-32602`, and the Python client span becomes ERROR with `error.type=-32602`.

**Reproduction** (Python 2.3.0 client against a Go SDK server):
```
git clone https://github.com/mkwitko/mcp-reliability && cd mcp-reliability
examples/python-sdk-streamable-http/run.sh
go run ./cmd/mcp-reliability doctor testdata/cross-python-client-go-server.jsonl
```
```
[WARN] MCP-ERR-005: span "tools/call missing" is marked failed (status=Error, error.type present=true) for a JSON-RPC code the semconv says SHOULD NOT be considered an error
```
Write-up: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/python-sdk-v2-validation.md

**Suggested change:** in the dispatcher, when the error code is one of the five caller-error codes, set `rpc.response.status_code` but leave the span status Unset and skip `error.type`. The server side already has the equivalent classification. I can send a PR.

## openllmetry #4374 comment

Reproduced with `opentelemetry-instrumentation-mcp==0.62.4` (latest on PyPI) on `mcp==1.30.0`, Streamable HTTP, Python client and server both instrumented. Every server span starts a new trace: `call_tool` builds `CallToolRequestParams(..., _meta=None)` unless the caller passes `meta=`, so the `if meta is not None` guard in the client wrapper never injects.

Two details that may help when fixing:
- Passing `meta={}` from the application restores propagation (workaround fixture `openllmetry-with-meta`).
- The transport-level writer (`InstrumentedStreamWriter.send`) does `setdefault("_meta", {})` and injects correctly, but it is only installed for the deprecated `streamablehttp_client`; the current `streamable_http_client` is not wrapped, so clients following the recommended API get no injection at all.

Reproduction, captured OTLP traces for the three variants, and file/line references from the installed wheel: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/openllmetry-validation.md (`examples/openllmetry-python/run.sh`).

## mcp-otel-go issue: client-side inject

Hi, thanks for the library. I was checking how it fits the OTel MCP semantic conventions and found one gap I would like to ask about before sending anything.

`mcpotel.Middleware` extracts `traceparent` from `params._meta` on the server (`mcpotel/propagation.go`), but there is no counterpart that injects it on the client. The semconv says instrumentation SHOULD inject context into `params._meta` "when creating a request or notification" (`docs/gen-ai/mcp.md`, "Context propagation"), and the MCP draft spec reserves `traceparent`, `tracestate` and `baggage` there (SEP-414). `metaCarrier.Set` exists but nothing calls it.

Effect: with a go-sdk client and an `mcpotel` server, every server span is a trace root unless the application instruments the client itself. Captured trace (six root server spans in six traces) and write-up: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/mcp-otel-go-validation.md

go-sdk v1.8.0 already has what is needed: `Client.AddSendingMiddleware` (and `Server.AddSendingMiddleware` for server-initiated requests such as sampling or elicitation). A sketch that works against v1.8.0 and links the server span to the client trace:

```go
func ClientMiddleware(cfg Config) mcp.Middleware {
    // start a SpanKindClient span named "{method} {target}", then:
    meta := params.GetMeta()
    if meta == nil {
        meta = map[string]any{}
    }
    propagator.Inject(ctx, metaCarrier(meta))
    params.SetMeta(meta)
    // call next, record the outcome the same way the server path does
}
```

Would you accept a PR that adds `mcpotel.ClientMiddleware(Config)`, reusing `metaCarrier`, with the same redaction defaults and the same `Filter` option? Happy to adjust the API to what you prefer.

## mcp-otel-go issue: status Ok and error.type values

Two smaller points found while checking v0.2.1 (commit `2049e1c`) against the OTel MCP semantic conventions, with go-sdk v1.8.0 and the in-memory transport. Both are low risk; I can send PRs if you agree.

**1. Successful spans get status `Ok`.** `Middleware` calls `span.SetStatus(codes.Ok, "")` on success (`mcpotel/middleware.go`). The semconv for MCP defers to the general "Recording errors" document, which says: "Span Status Code MUST be left unset if the instrumented operation has ended without any errors" (semantic-conventions v1.44.0, `docs/general/recording-errors.md`). Setting `Ok` also removes the backend's ability to tell "instrumentation did not decide" from "success". Proposed fix: drop the `else` branch and add a test asserting `codes.Unset` on success.

**2. `error.type` holds Go type names.** Observed: a tool handler returning an error gives `error.type=*errors.errorString`; calling an unknown tool (JSON-RPC `-32602`) gives status `Error` and `error.type=*jsonrpc2.WireError`. The semconv note on `error.type` asks for `tool_error` when the result has `isError: true`, and for the JSON-RPC error code as a string otherwise; it also says `-32700/-32600/-32601/-32602/-32002` SHOULD NOT be considered errors on the server span. Proposed fix: `extractToolError` returns `tool_error` for every `IsError` result; when `errors.As(err, *jsonrpc.Error)` matches, set `error.type` and `rpc.response.status_code` to the code and leave status unset for the five caller-error codes. `RedactError` would still apply to the status description, so the privacy default stays.

Captured traces and doctor output: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/mcp-otel-go-validation.md

## conformance issue: SEP-414 trace context scenario

The draft spec (`basic/index.mdx`, "OpenTelemetry trace context", SEP-414) reserves `traceparent`, `tracestate` and `baggage` in `_meta` and says: "When present, their values MUST follow W3C Trace Context and W3C Baggage formats respectively." The suite has no check for this today (`src/seps/` has no `sep-414.yaml`; the `request-metadata` scenario only covers the `io.modelcontextprotocol/*` keys). I would like to propose one and ask before writing code, per AGENTS.md.

**Client scenario `trace-context-meta`** (a `Scenario`): the mock server records every request and notification; for each message whose `params._meta` carries one of the three reserved keys, validate the value.
- `sep-414-client-traceparent-format`: FAILURE if any observed value is not a string matching W3C Trace Context (version `00`, non-zero trace-id and parent-id); SUCCESS if all observed values are valid; SKIPPED if the client never sent one, since the requirement applies only "when present".
- `sep-414-client-tracestate-format`, `sep-414-client-baggage-format`: same rule for the other two keys.

Open point: the reference `everything-client` sends no trace context today, so these checks would be SKIPPED against it. The positive example would need a branch that sets `_meta.traceparent`; a negative example client sends `traceparent: "not-a-traceparent"`.

**Server scenario `server-trace-context-meta`** (a `ClientScenario`): send `tools/list` with the spec's own example values in `_meta` and check the server still serves it. The spec has no MUST for the server side, so this would be WARNING at most. Whether the suite also wants a check that servers tolerate unknown vendor-prefixed `_meta` keys is a question for you; I found no sentence in the spec that requires it.

Motivation: in practice clients do send malformed or absent trace context, and the only MUST in SEP-414 is the format. I have a longer write-up with check ids, severity mapping and sketches in the suite's conventions: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/upstream/conformance-traceparent-scenario.md

If this fits, I can send the PR with the SEP yaml, both scenarios, the examples and the vitest negative case.

## typescript-sdk PR #3115 comment

I built this branch at `5a07386` (`pnpm install --frozen-lockfile && pnpm -r build`), ran a Streamable HTTP client and server with the Node OTel SDK, and checked the exported spans against the OTel MCP semantic conventions. Propagation works out of the box: every `tools/call` SERVER span shares the client's trace and is a child of its CLIENT span, with `mcp.method.name`, `jsonrpc.request.id`, `mcp.protocol.version`, `gen_ai.operation.name` and `gen_ai.tool.name` set. Nice work.

One deviation, on the client side only: calling an unknown tool returns JSON-RPC `-32602`, and the CLIENT span is marked `Error` with `error.type` set. The semconv (`docs/gen-ai/mcp.md`, footnote on `error.type`) says `-32700/-32600/-32601/-32602/-32002` "SHOULD NOT be considered errors"; the server span in this branch already follows that (status unset, `rpc.response.status_code` only), so client and server disagree about the same call. The Python SDK 2.3.0 client has the same behaviour, for what it is worth.

Minor: the server span for the unknown tool is named `tools/call` without the tool name, while the client names it `tools/call missing`. The docs explain the choice; just noting it affects client/server pairing in backends that group by span name.

Captured OTLP trace, commands and the full check output: https://github.com/mkwitko/mcp-reliability/blob/v0.1.0/docs/research/typescript-sdk-validation.md
