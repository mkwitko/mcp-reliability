# MCP Python SDK v2: first external validation of `doctor`

Date: 2026-10-30. Author: Claude, for the maintainer. Status: evidence from real captures plus the source code of the installed package.

## 1. Setup and versions

- `mcp` 2.3.0 from PyPI. It is the latest release, from 2026-10-02; earlier 2.x releases are 2.0.0, 2.0.1, 2.1.0, 2.1.1 and 2.2.0. It pulls in `mcp-types` 2.3.0 and `opentelemetry-api` 1.45.2. The license is MIT (`mcp-2.3.0.dist-info/METADATA`).
- `opentelemetry-sdk` 1.45.2 and `opentelemetry-exporter-otlp-proto-http` 1.45.2. Python 3.12, uv 0.11.16.
- Go side: `examples/go-sdk-streamable-http`, built on go-sdk v1.8.0.
- Collector: `otel/opentelemetry-collector-contrib:0.146.0` with the file exporter, the same as the other fixtures.
- Harness: `examples/python-sdk-streamable-http/` (`run.sh`). The client calls `echo`, `fail` and `missing` under one `agent step` root span, the same as the Go scenario. The only OTel code in the harness is the exporter setup. Every MCP span comes from the SDK.
- Semconv compared against: `open-telemetry/semantic-conventions-genai` `docs/gen-ai/mcp.md` at commit `6f98aef` (2026-10-30).
- Both sessions negotiated protocol `2026-07-28`, so the handshake is `server/discover` rather than `initialize`.

## 2. What the SDK really does (source read)

Paths are relative to `site-packages/mcp/` of `mcp` 2.3.0.

| What | Where | Fact |
|---|---|---|
| Default on, server | `server/lowlevel/server.py:451` | `self.middleware = [OpenTelemetryMiddleware()]`. The same line is in the 2.0.0, 2.0.1, 2.1.0, 2.1.1 and 2.2.0 wheels I downloaded. **The claim "default since 2.1.1" is wrong: it has been the default since 2.0.0.** |
| Server span | `server/_otel.py:35-42` | Kind SERVER, name `{method}` or `{method} {params.name}`, parent `extract_trace_context(ctx.meta)` |
| Server attributes | `server/_otel.py:21-33` | `mcp.method.name`, `mcp.protocol.version`, `jsonrpc.request.id`. On `tools/call` also `gen_ai.operation.name=execute_tool` and `gen_ai.tool.name`. On `prompts/get` also `gen_ai.prompt.name` |
| Server errors | `server/_otel.py:45-69` | For `MCPError` and `ValidationError`: `error.type` = `rpc.response.status_code` = the code, plus ERROR status. Other exceptions: `error.type=<class>`, the exception is recorded, ERROR status. `isError: true`: `error.type=tool_error`, ERROR status |
| Client span | `shared/jsonrpc_dispatcher.py:383-391` | Kind CLIENT, name `MCP send {method} {params.name}`, attributes `mcp.method.name` and `jsonrpc.request.id` **only** |
| Client injection | `shared/jsonrpc_dispatcher.py:393` and `shared/_otel.py:39-41` | `opentelemetry.propagate.inject(params._meta)`, which uses the global propagator. The Python default is tracecontext plus baggage |
| Client JSON-RPC errors | `shared/jsonrpc_dispatcher.py:417-420` | Sets `error.type`, `rpc.response.status_code` and ERROR status. This is the #3174 fix. It is absent from the 2.0.0-2.2.0 wheels |
| Client `isError` | none | The dispatcher never looks inside the result, so a client span for a tool error stays Unset and has no `error.type` |
| Notifications | `shared/jsonrpc_dispatcher.py:464+` (`notify`) | No span, no injection |
| Extract guard | `shared/_otel.py:44-60` | A missing, malformed or invalid `traceparent` returns `None`, so the span falls back to the ambient parent instead of becoming an orphan |
| Tracer | `shared/_otel.py:14` | `get_tracer("mcp-python-sdk")`, no version |
| Side effect | `client/session.py:1125-1127` | After a tool call, the client fetches output schemas, which issues an extra `tools/list` that appears in the trace |

## 3. What was emitted (testdata/python-sdk-default.jsonl, 11 spans)

The trace tree below was produced by parsing the file. `T0` and `T1` are trace ids.

```
T0 CLIENT  "MCP send server/discover"        {mcp.method.name, jsonrpc.request.id}           (root: handshake happens before "agent step")
T0  SERVER "server/discover"                 {mcp.method.name, mcp.protocol.version, jsonrpc.request.id}
T1 INTERNAL "agent step"
T1  CLIENT "MCP send tools/call echo"        {mcp.method.name, jsonrpc.request.id}           status Unset
T1   SERVER "tools/call echo"                {mcp.method.name, mcp.protocol.version, jsonrpc.request.id, gen_ai.operation.name, gen_ai.tool.name}
T1  CLIENT "MCP send tools/list"             (implicit, from the output-schema fetch)
T1   SERVER "tools/list"
T1  CLIENT "MCP send tools/call fail"        status Unset, no error.type
T1   SERVER "tools/call fail"                + error.type=tool_error, status ERROR
T1  CLIENT "MCP send tools/call missing"     status Unset, no error.type
T1   SERVER "tools/call missing"             + error.type=tool_error, status ERROR  (MCPServer maps an unknown tool to isError, not to a JSON-RPC error)
```

Propagation works end to end with no configuration: every SERVER span is a child of its CLIENT span. None of the 11 spans has a value for any of these attributes: `mcp.session.id`, `network.*`, `server.address`, `server.port`, `jsonrpc.protocol.version`, `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`. The demo argument string does not appear in the file.

## 4. Doctor output (verbatim)

```
$ go run ./cmd/mcp-reliability doctor testdata/python-sdk-default.jsonl
MCP Observability Report
file: testdata/python-sdk-default.jsonl
spans: 11

[WARN] MCP-SEM-002: tools/call span "MCP send tools/call echo" lacks gen_ai.tool.name (Conditionally Required when the operation relates to a tool)
        spans: [f0243d72c02b0292]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-SEM-002: tools/call span "MCP send tools/call fail" lacks gen_ai.tool.name (Conditionally Required when the operation relates to a tool)
        spans: [57dda773cdfa5621]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-SEM-002: tools/call span "MCP send tools/call missing" lacks gen_ai.tool.name (Conditionally Required when the operation relates to a tool)
        spans: [906226e27ccbf3d3]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-ERR-003: client and server spans for "tools/call fail" disagree on failure: the client span lacks error.type
        spans: [57dda773cdfa5621 1d3565baf2a73866]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
[WARN] MCP-ERR-003: client and server spans for "tools/call missing" disagree on failure: the client span lacks error.type
        spans: [906226e27ccbf3d3 1beb54564c9fc75d]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md

Result: 0 FAIL, 5 WARN, 0 INFO
exit=0
```

## 5. Classification

| Finding | Verdict | Evidence |
|---|---|---|
| MCP-SEM-002 x3 (client `tools/call` without `gen_ai.tool.name`) | **True positive** | The semconv client-span table marks `gen_ai.tool.name` "Conditionally Required: When operation is related to a specific tool" (`mcp.md` client table). The SDK sets only two attributes (`jsonrpc_dispatcher.py:390`), even though the tool name is already in scope at line 382 and is used in the span name. |
| MCP-ERR-003 x2 (client lacks `error.type` on `isError`) | **True positive** | Client-span footnote [1]: "When CallToolResult is returned with `isError` set to `true`, this attribute SHOULD be set to `tool_error`". The server middleware does this (`server/_otel.py:66-69`); the client dispatcher does not. A backend that only sees the client side counts these as successes. |

**Rule bugs found** (not visible in this capture, but demonstrated on a derived file):

1. **MCP-PROP-001 misses broken propagation for Python clients (a false negative).** The rule pairs a CLIENT span with a SERVER span on the key `(mcp.method.name, gen_ai.tool.name)` (`internal/rules/rules.go`, `propagation`). The Python client never sets `gen_ai.tool.name`, so its `tools/call` key is `("tools/call", "")` while the server's is `("tools/call", "echo")`, and the two never match. I copied the fixture to the scratchpad, gave every SERVER span a new trace id and removed its parent, then ran doctor. PROP-001 fired only for `server/discover` and `tools/list`, and the three broken `tools/call` hops went unreported. Suggested fix: match on `jsonrpc.request.id` plus method when both sides carry it (Python sets it on both sides), or fall back to the method alone when the client has no tool name. This needs an extra fixture; I did not commit one because it would be synthetic.
2. Possible issue: there is no rule for the span name. The client name `MCP send tools/call echo` breaks the SHOULD "`{mcp.method.name} {target}`" (`mcp.md`, client span "Span name"), and nothing reports it. If a rule is added it should be WARN, because the text is a SHOULD. The same goes for `gen_ai.operation.name` missing on client `tools/call` spans (Recommended, footnote [6] "SHOULD be set to `execute_tool`").

**Semconv drift:**

- The spec method `server/discover` (protocol 2026-07-28) is in neither the semconv well-known `mcp.method.name` list nor `knownMethods` in `rules.go`. Detection still works because `looksMCP` matches on the `mcp.*` attributes. A span that has only a name, with no attributes, would not be recognised.
- The SDK points `error.type` at the JSON-RPC code string, for example `-32602`, exactly as footnote [1] says. No drift there.

**Not tested (gaps):** JSON-RPC error responses on the server side over the wire. `MCPServer` turns an unknown tool into `isError` (`server/mcpserver/server.py:456`), so this scenario cannot produce one. The Go server does produce one, but that direction crashed (section 7).

## 6. python-sdk issues

- **#3174** ("[v2] JSON-RPC error responses leave client OpenTelemetry spans UNSET"), closed 2026-10-02. **Reproduced on 2.2.0 and confirmed fixed in 2.3.0.** I ran a scratch probe: a Python server whose tool raises `MCPError(-32602)`, over Streamable HTTP, with an in-memory exporter on the client. It is not a committed fixture. Results:
  - `mcp 2.3.0`: `CLIENT 'MCP send tools/call rpc_fail' ERROR {'error.type': '-32602', 'rpc.response.status_code': '-32602'}`
  - `mcp 2.2.0`: `CLIENT 'MCP send tools/call rpc_fail' UNSET {}`

  The fix is `jsonrpc_dispatcher.py:417-420`. In the default capture the bug cannot show, because no JSON-RPC error crosses the wire. Note that #3174 covers JSON-RPC errors only. The `isError` gap that MCP-ERR-003 found is a separate problem, and as far as I know nobody has reported it.
- **#3715** ("Client: expose an outbound request seam"), open. **The root cause behind both true positives is confirmed.** The TODO quoted in the issue is at `jsonrpc_dispatcher.py:384-385` in 2.3.0. Without that seam, a user cannot add `gen_ai.tool.name`, `gen_ai.operation.name` or `error.type=tool_error` to the client span except by wrapping private functions. This capture does not exercise the issue's notification point (`notify()` has no span and no injection) because protocol 2026-07-28 sends no `notifications/initialized`.

## 7. Cross-SDK

- **Go example client to Python server** (`testdata/cross-go-client-python-server.jsonl`, 9 spans): **propagation links across SDKs.** Every Python SERVER span is a child of its Go CLIENT span, in the same trace, including `server/discover`. Doctor reports 0 findings, which is correct: our Go client sets `gen_ai.tool.name` and `error.type=tool_error`, and Python's server does the same. The Go client got `isError` for `missing` here, because `MCPServer` does not return a JSON-RPC error for an unknown tool.
- **Python client to Go example server: not captured. The Go example server crashes.** The trace shows `panic: nil pointer dereference` in `(*InitializedParams).GetMeta` called from `serverMiddleware` (`examples/go-sdk-streamable-http/main.go`, the `if p := req.GetParams(); p != nil` line in `serverMiddleware`). go-sdk v1.8.0 passes middleware a typed-nil `*InitializedParams` when the peer sends a notification without `params`, and `GetMeta` is promoted from the embedded `Meta` value, so it dereferences nil. The Python client fell back to `initialize` plus a param-less `notifications/initialized` against the Go server. This is a bug in our example and a sharp edge in go-sdk, not in the Python SDK. Suggested fix: `p != nil && !reflect.ValueOf(p).IsNil()`. I did not apply it because that file was being edited concurrently. After the fix, this direction would also exercise #3174 over the wire, since the Go server returns a JSON-RPC error for `missing`.

## 8. Bottom line

The Python SDK v2 is the first SDK that propagates out of the box: with no configuration, client and server join one trace, even across SDKs. Doctor's two findings on it are true positives with a common root cause (#3715): the client span is thin. Doctor also has a real false negative: PROP-001 depends on the client setting `gen_ai.tool.name`. That should be fixed before the tool is pointed at Python-client traces in which propagation is actually broken.

## Addendum 2026-10-30 (after rules MCP-SEM-003/004, MCP-ERR-004/005 and the harness fix)

- Python client to Go server now captures (`testdata/cross-python-client-go-server.jsonl`) after the typed-nil fix in the Go harness.
- New true positive: the Python 2.3.0 client marks `-32602` (unknown tool, returned by the Go server) with status ERROR and `error.type`. mcp.md footnote [5] lists `-32602` among codes that SHOULD NOT be considered errors (MCP-ERR-005). This is the same #3174 fix overshooting: it marks every JSON-RPC error as a failure.
- MCP-PROP-001 now pairs by `jsonrpc.request.id`, so the Python client's missing `gen_ai.tool.name` no longer hides a broken link.
