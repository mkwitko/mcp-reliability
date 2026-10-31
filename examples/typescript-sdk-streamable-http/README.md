# TypeScript SDK scenario (Streamable HTTP)

Two variants, same tool surface and arguments as the Python example (`echo`, `fail` returning `isError`, `missing` not registered; the client calls all three under one `agent step` root span with `{"text":"hello secret-token-123"}`). The only OpenTelemetry code is the exporter setup in `otel_setup.mjs` (NodeSDK, OTLP/HTTP to `127.0.0.1:4318`, default `tracecontext,baggage` propagator). Plain Node ESM, no build step.

## Variants

- **Baseline** (this directory): `@modelcontextprotocol/sdk` 1.32.2 (latest on npm, verified 2026-10-31), `@opentelemetry/sdk-node` 0.300.0, `@opentelemetry/exporter-trace-otlp-http` 0.300.0, `@opentelemetry/api` 1.10.0, `zod` 4.6.7. Locked in `package-lock.json`. Node v24.12.0.
- **`pr3115/`**: the open PR modelcontextprotocol/typescript-sdk#3115, branch `feat/protocol-instrumentation`, commit `5a073867a783066cc673ff07f7943aebe6165787` (2026-10-30), package version 2.4.0 (a v2-API build, so `server.mjs`/`client.mjs` there differ from the baseline). `build-tarballs.sh` clones that commit, builds with pnpm and packs `core`, `client` and `server` into `pr3115/vendor/` (gitignored). `package.json` depends on those tarballs by path, for capture only. There is no committed lockfile there because tarball hashes are local. Not a published release.

Collector: `otel/opentelemetry-collector-contrib:0.146.0` (`examples/collector`).

## Run

```
export PATH=$PATH:$HOME/.local/go-sdk/go/bin
examples/typescript-sdk-streamable-http/pr3115/build-tarballs.sh   # optional, for the pr3115 scenario
examples/typescript-sdk-streamable-http/run.sh
go run ./cmd/mcp-reliability doctor testdata/typescript-sdk-default.jsonl
```

Port 4318 must be free. `run.sh` restarts the collector per scenario and writes:

| Fixture | Setup | Shows |
|---|---|---|
| `typescript-sdk-default` | 1.32.2 client -> 1.32.2 server | Only the client's `agent step` span exists: no MCP spans, nothing in `_meta` |
| `cross-go-client-typescript-server` | Go harness client -> 1.32.2 server | Go spans only; the server adds nothing and the trace is not continued |
| `cross-typescript-client-go-server` | 1.32.2 client -> Go harness server | Go server spans are all roots: the TS client injects nothing |
| `typescript-sdk-pr3115` | #3115 client -> #3115 server | Built-in CLIENT and SERVER spans in one trace |

The #3115 scenario is skipped when `pr3115/vendor/*.tgz` is missing.

## Notes

- With no instrumentation the baseline fixtures have no MCP spans, so `doctor` has nothing to check and prints "no findings" for `typescript-sdk-default`. That is an absence of data, not a pass.
- The 1.32.2 server is stateless (a new `McpServer` and transport per request) and serves any path so the Go client can reach it. For an unknown tool it returns a tool result with `isError`, not a JSON-RPC error; the #3115 build returns JSON-RPC `-32602`.
- #3115 uses protocol 2026-07-28 (`server/discover`) by default (`versionNegotiation: auto`); `client.mjs --legacy` forces the old handshake.
