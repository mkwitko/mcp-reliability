# openllmetry MCP instrumentation scenario (Streamable HTTP)

This scenario runs the MCP Python SDK `mcp==1.30.0` with no built-in OTel. Both sides are instrumented only by traceloop's `opentelemetry-instrumentation-mcp==0.62.4`, through `McpInstrumentor().instrument()` in `otel_setup.py`. mcp 2.x is not used because the instrumentation fails on it (traceloop/openllmetry#4413).

The server is a FastMCP server with the tools `echo` and `fail` (`isError`), served at `127.0.0.1:8766/`. The client opens an `agent step` root span and calls `echo`, `fail` and `missing`. It has two flags:

- `--meta`: passes `call_tool(..., meta={})`.
- `--legacy-transport`: uses the deprecated `streamablehttp_client`, which is the only Streamable HTTP factory the instrumentation wraps.

```
examples/openllmetry-python/run.sh
```

`run.sh` uses its own collector, compose project `col-openllmetry` on port 4319, writing to `examples/collector/out-openllmetry/`. It writes three fixtures:

- `testdata/openllmetry-default.jsonl`
- `testdata/openllmetry-with-meta.jsonl`
- `testdata/openllmetry-legacy-transport.jsonl`

Findings: `docs/research/openllmetry-validation.md`.
