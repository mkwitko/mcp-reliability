# E1/H1 on traceloop openllmetry's MCP instrumentation

Date: 2026-10-30. Scenario: `examples/openllmetry-python/` (`run.sh`). Fixtures: `testdata/openllmetry-default.jsonl`, `testdata/openllmetry-with-meta.jsonl`, `testdata/openllmetry-legacy-transport.jsonl`.

## Verdict

**H1 reproduced.** With the instrumentation installed on both sides and an ordinary `ClientSession.call_tool(name, args)` over `streamable_http_client`, nothing is propagated. Every server span is a trace root in its own trace:

| request (id) | client trace | server trace |
|---|---|---|
| tools/call echo (1) | `c3b29625051aa552841de138b891c898` | `8d968ebe7a4e90bc1b4a095a57613f72` |
| tools/list (2) | `c3b29625051aa552841de138b891c898` | `990c5a5dde49b118bc22c75ae586e2b8` |
| tools/call fail (3) | `c3b29625051aa552841de138b891c898` | `ce973440216e4757e778cb43ba6d9ed3` |
| tools/call missing (4) | `c3b29625051aa552841de138b891c898` | `9366f3af8414c7932b490cd9352ee953` |
| initialize (0) | `526373e678f22ab65c5ac424fdb1ae2e` | `1e48cb530723a0a4a2f8138e439fff8a` |

traceloop/openllmetry#4374 is **reproduced** in 0.62.4. One caveat applies. A second injection path exists, and it rescues propagation, but only when the client uses the deprecated `streamablehttp_client`. See the variants below.

`doctor` does **not** detect the break: there is no MCP-PROP-001, because openllmetry emits only INTERNAL spans. This is a false negative and a rule gap (see below).

## Versions

- `opentelemetry-instrumentation-mcp==0.62.4`, the latest on PyPI (uploaded 2026-09-29). It declares `_instruments = ("mcp >= 1.6.0",)` (`instrumentation.py:29`).
- `mcp==1.30.0`, the latest 1.x (2026-09-07). mcp 2.x is not usable: with `mcp==2.3.0`, `McpInstrumentor().instrument()` raises `PathResolutionError: unable to resolve attribute 'streamablehttp_client' ... on <module 'mcp.client.streamable_http'>` (I reproduced it on 2026-10-30; this is traceloop/openllmetry#4413). mcp 2.3.0 also has no `mcp/shared/session.py`, so the `BaseSession.send_request` patch (`instrumentation.py:116-120`) has nothing to wrap. mcp 1.x has no built-in OTel, so every span here comes from openllmetry.
- `opentelemetry-sdk==1.45.2`, `opentelemetry-exporter-otlp-proto-http==1.45.2`. Resolved transitively: `opentelemetry-instrumentation==0.66b2`, `opentelemetry-semantic-conventions-ai==0.5.1`.
- Python 3.12, uv 0.11.16, collector `otel/opentelemetry-collector-contrib:0.146.0`.

## Installed-source evidence

Paths are relative to `examples/openllmetry-python/.venv/lib/python3.12/site-packages/`. Line numbers are from the installed 0.62.4 wheel, not GitHub main, which has drifted (the line numbers in `instrumentation-libraries.md` section 2.4 are from main).

Path 1 is the `send_request` patch (`opentelemetry/instrumentation/mcp/instrumentation.py`). It is always active and wraps `mcp.shared.session.BaseSession.send_request` (lines 116-120).

```python
# 196-205
            if params:
                if hasattr(args[0].root.params, "meta"):
                    meta = args[0].root.params.meta

            # Handle trace context propagation
            if meta and len(args) > 0:
                carrier = {}
                TraceContextTextMapPropagator().inject(carrier)
                meta.traceparent = carrier["traceparent"]
                args[0].root.params.meta = meta
```

- `mcp/client/session.py:397-406` (mcp 1.30.0): `call_tool` builds `CallToolRequestParams(..., _meta=None)` unless the caller passes `meta=`. The guard at line 201 is therefore false, and nothing is injected. This is #4374.
- The patch injects before the `{tool}.tool` span starts (line 307). So even when it fires, the traceparent names the caller's span (here `agent step`), not the client tool span. It writes only `traceparent`. `tracestate` and `baggage` are dropped, and it uses a hard-coded W3C propagator rather than the global one.
- `@dont_throw` (line 187, `utils.py:80`) swallows any exception from the patch.

Path 2 is the transport stream writer (`InstrumentedStreamWriter.send`, lines 584-640). For every outgoing `JSONRPCRequest` it does `request.params.setdefault("_meta", {})` (line 629) and `propagate.get_global_textmap().inject(meta)` (line 631). This runs inside a `ResponseStreamWriter` span (line 603), which is the parent the server sees. It creates `_meta` when absent, so it has no #4374 problem. However, it is only installed by `_transport_wrapper`, which wraps `mcp.client.streamable_http.streamablehttp_client` (lines 100-107). In mcp 1.30.0 that function is `@deprecated("Use `streamable_http_client` instead.")` (`mcp/client/streamable_http.py:709-711`). The recommended `streamable_http_client` (line 619), which `mcp/client/session_group.py:29,323` also uses, is not wrapped. A client that follows the current API therefore gets only Path 1.

On the server side, `StreamableHTTPServerTransport.connect` is wrapped (lines 108-114). `InstrumentedStreamReader.__aiter__` (lines 558-568) extracts `_meta` with the global propagator and attaches it while the message is handed to the session. `ServerSession.__init__` (lines 93-99, 165-183) carries that context to the handler. The server spans are the `ResponseStreamWriter` spans for the response writes. No handler span exists for `mcp.server.fastmcp.FastMCP`: `fastmcp_instrumentation.py:26-38` hooks only the standalone `fastmcp` package (`fastmcp.tools.tool_manager`, `fastmcp.FastMCP`).

## Variants and results

All variants use the same tools and calls (`echo`, `fail` returning `isError`, `missing`). `client.py` also triggers one `tools/list`: mcp 1.x lists tools after the first call to validate output schemas. FastMCP 1.x reports the unknown tool `missing` as an `isError` result, not as a JSON-RPC error.

| variant | client flags | propagation |
|---|---|---|
| `openllmetry-default` | `streamable_http_client`, `call_tool(name, args)` | **broken** for every request (table above) |
| `openllmetry-with-meta` | `--meta` (`call_tool(..., meta={})`) | tools/call joins trace `7bded01b78f701f13d9cc4f059da3c7e`. Each server span's parent is `f5e8feb4383d8552` = `agent step`, a sibling of the client `echo.tool` span, not its child. `tools/list` (`6d086c877def1f1010e67710e835f0f6`) and `initialize` (`499012f79e881a168b599675175b257f`) still break, because the SDK sends them with `meta=None`. |
| `openllmetry-legacy-transport` | `--legacy-transport` (deprecated `streamablehttp_client`) | **works** for all requests. Example: echo, server span `bc68ece18cfa8061` in trace `9e12f04ea89e26c198a0207dfc4116f3`, parent `cabded9918e56e36` = the client `ResponseStreamWriter` under `echo.tool`. `initialize` also joins (`7052aa4131add74e440bc8fcce8118b5`). |

So by default, with the current mcp 1.x API, propagation is broken. Passing a non-None `meta` fixes tool calls only, and attaches them one level too high. The deprecated transport factory fixes everything.

## Raw span shape

Every span is `kind=1` (INTERNAL). The scope is `opentelemetry.instrumentation.mcp.instrumentation` version `0.62.4`. No span has `mcp.method.name`, `gen_ai.tool.name`, `gen_ai.operation.name`, `jsonrpc.request.id` or `rpc.response.status_code`. The constant `MCP_METHOD_NAME` exists in `semconv_ai/__init__.py:314`, but 0.62.4 never sets it.

Client spans:

- `initialize.mcp`, `tools/list.mcp` (`{method}.mcp`, line 339): `traceloop.entity.input`, `traceloop.entity.output`. On success, status is `Ok` (line 385).
- `echo.tool`, `fail.tool`, `missing.tool` (`{tool}.tool`, line 295): `traceloop.span.kind=tool`, `traceloop.entity.name=<tool>`, `traceloop.entity.input` (`{"tool_name":..., "arguments":{...}}`, which includes the argument value `hello secret-token-123`), `traceloop.entity.output`. On success, status is `Ok`. On `isError`, status is `Error` with the tool text as description and `error.type=tool_error` (line 381).
- `ResponseStreamWriter` (legacy transport only): `mcp.request.id`. It also appears once with no attributes, as a root, for the `notifications/initialized` write.

Server spans:

- `ResponseStreamWriter`: `mcp.request.id`, `mcp.response.value` (the serialized result, so it includes `hello secret-token-123`). On `isError`, status is `Error` with the tool text. There is no `error.type`.

Content capture is on by default: `utils.py:17-30` returns true unless `TRACELOOP_TRACE_CONTENT` is set to something other than `true`.

## doctor output (default fixture)

```
$ go run ./cmd/mcp-reliability doctor testdata/openllmetry-default.jsonl
MCP Observability Report
file: testdata/openllmetry-default.jsonl
spans: 11

[FAIL] MCP-SEM-001: span "ResponseStreamWriter" looks like an MCP span but lacks mcp.method.name (Required)
        spans: [fcf59f757e9f81b1]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[FAIL] MCP-SEM-001: span "ResponseStreamWriter" looks like an MCP span but lacks mcp.method.name (Required)
        spans: [0bec435045ea90b4]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[FAIL] MCP-SEM-001: span "ResponseStreamWriter" looks like an MCP span but lacks mcp.method.name (Required)
        spans: [39c142d356d8bb4b]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[FAIL] MCP-SEM-001: span "ResponseStreamWriter" looks like an MCP span but lacks mcp.method.name (Required)
        spans: [5a64eefe36b96c0f]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[FAIL] MCP-SEM-001: span "ResponseStreamWriter" looks like an MCP span but lacks mcp.method.name (Required)
        spans: [6c0b727d60f7e96d]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-ERR-001: span "ResponseStreamWriter" failed (status=Error, rpc.response.status_code present=false) but lacks error.type (Conditionally Required if the operation fails)
        spans: [5a64eefe36b96c0f]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-ERR-001: span "ResponseStreamWriter" failed (status=Error, rpc.response.status_code present=false) but lacks error.type (Conditionally Required if the operation fails)
        spans: [6c0b727d60f7e96d]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 5 FAIL, 2 WARN, 0 INFO
exit status 1
```

`doctor -json` rule counts:

- default: `MCP-SEM-001`: 5, `MCP-ERR-001`: 2.
- with-meta: `MCP-SEM-001`: 5, `MCP-ERR-001`: 2.
- legacy-transport: `MCP-SEM-001`: 10, `MCP-ERR-001`: 2. The extra five come from the client `ResponseStreamWriter` spans, which carry `mcp.request.id`.

The finding sets are identical for the broken (default) and the partly fixed (with-meta) traces. doctor cannot tell them apart.

## Which findings are real, and what doctor misses

Findings it raises:

- MCP-SEM-001 (5): **true positive in substance, imprecise target.** The server side emits no semconv MCP span at all, and no span in the file has `mcp.method.name`. The rule picks these spans only because `looksMCP` (`internal/rules/rules.go:67-79`) matches any `mcp.*` key, and openllmetry's `mcp.request.id` / `mcp.response.value` are vendor names, not semconv ones (semconv uses `jsonrpc.request.id`). The flagged span is an INTERNAL "response write" span, which the semconv does not define as an MCP client or server span. FAIL is defensible, since the server has nothing that conforms, but the message points at the wrong thing.
- MCP-ERR-001 (2): **true positive.** The server spans for `fail` and `missing` have status Error and no `error.type`. The client `{tool}.tool` span does set `error.type=tool_error`, but the server never does. The same "INTERNAL span" caveat applies.

Rule gaps (false negatives) found with this library:

1. **The propagation break is missed.** MCP-PROP-001 only pairs `SpanKindClient` and `SpanKindServer` spans (`rules.go:207`, `rules.go:229`), and every openllmetry span is INTERNAL. H2's "detects that failure" therefore does not hold for openllmetry traces. A kind-agnostic pairing would need a join key, and none exists: the client tool spans carry no request id, and the server spans carry `mcp.request.id` but no method or tool name.
2. **`{tool}.tool` and `{method}.mcp` spans are invisible.** They have no `mcp.*`, `gen_ai.tool.*` or `jsonrpc.request.id` attributes, and their names do not match `{method}` or `{method} {target}`. So doctor reports no SEM-001, SEM-003 or ERR-004 for them, although they lack `mcp.method.name` and set status `Ok` on success (`instrumentation.py:385`, which would be MCP-ERR-004 FAIL).
3. **Payload capture is missed.** MCP-PRIV-001 only knows the semconv names (`rules.go:61-64`). This file has tool arguments and results, including the test secret, in `traceloop.entity.input`, `traceloop.entity.output` and `mcp.response.value`, and capture is on by default.
4. **The with-meta mis-parenting is not checkable.** The server span hangs off `agent step` instead of the client tool span. Without CLIENT/SERVER kinds or a request id on the client span, no rule could see it.
5. **There is no wire check for Python.** MCP-PROP-004 needs `-wire`, and only the Go example writes a wire capture.

Gaps 1 and 2 have a common cause: doctor assumes the OTel MCP semconv shape (span kind, `mcp.method.name`). openllmetry uses Traceloop conventions. Teaching doctor `traceloop.*` would be vendor-specific (ADR territory). A cheaper path is an INFO rule along the lines of "MCP-looking spans exist but none is CLIENT/SERVER with `mcp.method.name`: propagation and pairing checks were skipped", so a clean report is not read as "propagation OK". This is a HYPOTHESIS and needs its own spec-driven pass.

## Reproduce

```
export PATH=$PATH:$HOME/.local/go-sdk/go/bin
examples/openllmetry-python/run.sh      # collector on :4319 (project col-openllmetry), server on :8766
go run ./cmd/mcp-reliability doctor testdata/openllmetry-default.jsonl
```
