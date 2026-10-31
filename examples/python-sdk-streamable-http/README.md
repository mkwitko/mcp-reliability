# Python SDK v2 scenario (Streamable HTTP)

A real-world check that does not use our own instrumentation. Both server and
client use the official MCP Python SDK, `mcp==2.3.0` (the latest on PyPI on
2026-10-30). The only OpenTelemetry code here is the plain SDK exporter setup
(`otel_setup.py`). Every MCP span comes from the SDK itself.

Pinned versions (`pyproject.toml`, `uv.lock`): `mcp==2.3.0` (pulls
`mcp-types==2.3.0`, `opentelemetry-api>=1.28.0`), `opentelemetry-sdk==1.45.2`,
`opentelemetry-exporter-otlp-proto-http==1.45.2`. Python 3.12, uv 0.11.16.
Collector: `otel/opentelemetry-collector-contrib:0.146.0` (`examples/collector`).

## Files

- `server.py`: `MCPServer` with the tools `echo` and `fail` (`fail` returns `CallToolResult(is_error=True)`). It is the same tool surface as the Go example and serves Streamable HTTP on `127.0.0.1:8765` at path `/`, so the Go client can connect to it too.
- `client.py`: `mcp.client.Client`. It opens one root span, `agent step`, and calls `echo`, `fail`, `missing`.
- `otel_setup.py`: a TracerProvider with a BatchSpanProcessor and an OTLP/HTTP exporter to `http://localhost:4318/v1/traces`. There is no propagator setup because the opentelemetry-python default is already `tracecontext,baggage`. That differs from Go, where the default global propagator is a no-op.
- `run.sh`: captures the fixtures. It mirrors `examples/run-scenarios.sh`.

## Run

```
export PATH=$PATH:$HOME/.local/go-sdk/go/bin   # Go is only needed for the cross-SDK scenario
examples/python-sdk-streamable-http/run.sh
go run ./cmd/mcp-reliability doctor testdata/python-sdk-default.jsonl
```

`run.sh` runs `uv sync --locked` and builds the Go example. Before each scenario it restarts the collector. It writes:

- `testdata/python-sdk-default.jsonl`: Python client to Python server.
- `testdata/cross-go-client-python-server.jsonl`: Go example client (`-role client`) to Python server.

The Python client to Go example server direction is not captured. The Go example server panics in its middleware on a request with no `params` (see the validation doc).

To run it by hand: start `docker compose -f examples/collector/docker-compose.yml up -d`. Then, in this directory, run `uv run server.py` and in another terminal `uv run client.py`. Stop the server with Ctrl-C, which flushes its spans.

## What the SDK does by default (read in the installed source)

Paths are relative to `.venv/lib/python3.12/site-packages/mcp/`.

- Server spans are on by default. `server/lowlevel/server.py:451` seeds `self.middleware = [OpenTelemetryMiddleware()]`, and the comment at lines 446-448 says to drop it to opt out. This has been the case since 2.0.0: I checked the 2.0.0, 2.0.1, 2.1.0, 2.1.1 and 2.2.0 wheels, and all of them have the same line. It did not start in 2.1.1.
- Server middleware (`server/_otel.py`):
  - It produces a SERVER span named `{method} {params.name}` (lines 35-36) with the attributes `mcp.method.name`, `mcp.protocol.version` and `jsonrpc.request.id` (lines 21-26).
  - For `tools/call` it also sets `gen_ai.operation.name=execute_tool` and `gen_ai.tool.name`. For `prompts/get` it sets `gen_ai.prompt.name` (lines 28-33).
  - The parent comes from `extract_trace_context(ctx.meta)` (line 39).
  - On an `MCPError` or `ValidationError` it sets `error.type` and `rpc.response.status_code` to the code, plus ERROR status (lines 45-55). Any other exception gets `error.type=<exception class>` (lines 56-60). A result with `isError: true` gets `error.type=tool_error` and ERROR status (lines 61-69).
- Client spans and injection are also on by default. They are not middleware: they are hard-coded in `shared/jsonrpc_dispatcher.py`, `send_raw_request`.
  - The CLIENT span is named `MCP send {method} {params.name}` (line 383), with only `mcp.method.name` and `jsonrpc.request.id` (line 390).
  - `inject_trace_context(out_meta)` writes `traceparent`/`tracestate`/`baggage` into `params._meta` (line 393, helper at `shared/_otel.py:39-41`).
  - A JSON-RPC error response sets `error.type`, `rpc.response.status_code` and ERROR status (lines 417-420). That is the python-sdk#3174 fix, first released in 2.3.0.
  - The comment at lines 384-385 reads "TODO(maxisbey): move the otel span + inject into an outbound middleware once that seam exists" (python-sdk#3715).
- Notifications get no span and no injection (`notify()`, line 464 onwards).
- Tracer name: `mcp-python-sdk`, with no version (`shared/_otel.py:14`).

There is nothing to switch on. Install an OTel SDK TracerProvider and the spans appear. Because the client already injects by default, a variant with manual client injection would measure nothing new, so it was not built.
