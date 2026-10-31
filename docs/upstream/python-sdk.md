# Upstream drafts: modelcontextprotocol/python-sdk

Evidence: `docs/research/python-sdk-v2-validation.md`, fixtures
`testdata/python-sdk-default.jsonl` and `testdata/cross-python-client-go-server.jsonl`,
mcp 2.3.0, verified 2026-10-30. Nothing below has been sent. Check for
duplicates (python-sdk #3715 covers client attributes) before opening.

## Draft issue 1: client span ignores tool `isError`

**Title:** OTel client span stays Unset when the tool result has `isError: true`

**Body:**

With `mcp==2.3.0`, the client span created in `shared/jsonrpc_dispatcher.py`
(around lines 383-420) sets ERROR status and `error.type` for a JSON-RPC error
response (the #3174 fix), but not for a successful response whose
`CallToolResult.isError` is true. The server span for the same call does set
`error.type=tool_error` (`server/_otel.py`, lines 45-69).

Semantic conventions, `docs/gen-ai/mcp.md` footnote [1] on `error.type`:
instrumentation SHOULD set `tool_error` when the tool result reports an error.
Recording Errors asks that both ends of an operation carry the same
`error.type`.

Reproduction: `examples/python-sdk-streamable-http/run.sh` in
github.com/mkwitko/mcp-reliability, then
`mcp-reliability doctor testdata/python-sdk-default.jsonl` reports
`MCP-ERR-003: client and server spans for "tools/call fail" disagree on
failure: the client span lacks error.type`.

Suggested change: after the response is parsed in the dispatcher, if the
result is a `CallToolResult` with `isError`, set `error.type="tool_error"` and
status ERROR, matching `server/_otel.py`.

## Draft issue 2: client marks caller-error JSON-RPC codes as span failures

**Title:** OTel client span sets ERROR status for `-32602` and other caller-error codes

**Body:**

Since 2.3.0 every JSON-RPC error response marks the client span ERROR with
`error.type` set to the code. The MCP semantic conventions, `mcp.md`
footnote [5], say the codes `-32700`, `-32600`, `-32601`, `-32602` (also used
for unknown tools) and `-32002` "indicate that the caller sent a request the
receiver could not serve and SHOULD NOT be considered errors"; only other codes
SHOULD be considered errors. `rpc.response.status_code` should still be set.

Reproduction: Python client against a server that returns `-32602` for an
unknown tool (the Go SDK does), fixture
`testdata/cross-python-client-go-server.jsonl`; doctor reports `MCP-ERR-005`.

## Draft issue 3: client span name and attributes (extends #3715)

Client spans are named `MCP send {method} {name}` and carry only
`mcp.method.name` and `jsonrpc.request.id`. `mcp.md` says the span name SHOULD
be `{mcp.method.name} {target}`, and `gen_ai.tool.name` is Conditionally
Required on tool calls, `gen_ai.operation.name=execute_tool` Recommended.
This is likely covered by the TODO quoted in #3715; add the evidence there
rather than opening a new issue.
