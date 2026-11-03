# mcp-reliability

[![ci](https://github.com/mkwitko/mcp-reliability/actions/workflows/ci.yml/badge.svg)](https://github.com/mkwitko/mcp-reliability/actions/workflows/ci.yml)

**Status: pre-alpha.** A proof of concept runs end to end. Nothing is released, stable, or validated by outside users. The module path (`github.com/mkwitko/mcp-reliability`) is provisional.

## The problem

MCP (Model Context Protocol) servers and clients can emit OpenTelemetry traces, but the result is often broken in quiet ways: the server span is not linked to the client span, failed tool calls are not marked as errors, required attributes are missing, or tool arguments are recorded in plain text. Backends show whatever they receive; nothing tells you the data is wrong.

## What it does

`mcp-reliability doctor` reads a trace file, checks it against the OpenTelemetry GenAI/MCP semantic conventions and the MCP spec (SEP-414 trace context in `_meta`), and prints findings with the span ids as evidence and a link to the text it enforces. It is deterministic, runs locally, and makes no network calls.

Severity follows the standard: FAIL only for a MUST or a Required attribute, WARN for SHOULD, Conditionally Required, or sensitive data, INFO when the evidence is ambiguous.

## Quick start

Needs Go 1.27 or newer.

```sh
git clone https://github.com/mkwitko/mcp-reliability && cd mcp-reliability

go run ./cmd/mcp-reliability doctor testdata/broken-propagation.jsonl   # findings
go run ./cmd/mcp-reliability doctor testdata/healthy.jsonl              # no findings
go run ./cmd/mcp-reliability doctor -json testdata/broken-propagation.jsonl
go run ./cmd/mcp-reliability doctor -strict testdata/broken-propagation.jsonl
```

Exit codes: `0` no FAIL (and no WARN with `-strict`); `1` a FAIL, or a WARN with `-strict`; `2` usage, read, or parse error.

Example (shortened):

```
MCP Observability Report
file: testdata/broken-propagation.jsonl
spans: 13

[WARN] MCP-PROP-001: SERVER span "server/discover" (service "example-mcp-server", trace a463d3b12d1736f9b1558c214305f73f) is a root span while CLIENT span for the same call exists in trace ad105e1c9055cb2a1669788cd31a0e28: trace context was not propagated via params._meta
        spans: [a94f716c63e74d91 3060da918f986839]
        ref: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
...
Result: 0 FAIL, 6 WARN, 0 INFO
```

## Get a trace file from your own system

`doctor` reads newline-delimited OTLP JSON, one export request per line, as written by the OpenTelemetry Collector `file` exporter. Point your OTLP exporter at a collector with this pipeline (full file: `examples/collector/config.yaml`):

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
exporters:
  file:
    path: /out/traces.jsonl
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [file]
```

Then run `mcp-reliability doctor traces.jsonl`. Trace files can contain sensitive payloads; review them before sharing.

## Rules

| ID | Severity | Checks |
|----|----------|--------|
| MCP-SEM-001 | FAIL | MCP span lacks `mcp.method.name` (Required) |
| MCP-SEM-002 | WARN | `tools/call` span lacks `gen_ai.tool.name` |
| MCP-ERR-001 | WARN | Failed span lacks `error.type` |
| MCP-ERR-002 | WARN | `error.type` set but span status is Unset |
| MCP-ERR-003 | WARN | Client and server spans of one call disagree about failure |
| MCP-PROP-001 | WARN | Server span is a trace root although a client span for the same call exists: context not propagated via `_meta` |
| MCP-PROP-002 | INFO | Parent span id not present in the file |
| MCP-PRIV-001 | WARN | Opt-in payload attributes (tool arguments/results, messages) are present |
| MCP-ERR-004 | FAIL | Span status is `Ok`; status MUST stay Unset on success |
| MCP-ERR-005 | WARN | Caller-error JSON-RPC code (-32700/-32600/-32601/-32602/-32002) recorded as a failure |
| MCP-ERR-006 | WARN | `error.type` holds a Go pointer type name instead of `tool_error` or the JSON-RPC code |
| MCP-PROP-005 | INFO | All MCP SERVER spans are roots and the file has no MCP CLIENT span although another service is present |
| MCP-PROP-006 | INFO | MCP-looking spans exist but none is CLIENT/SERVER with `mcp.method.name`; pairing checks could not run |
| MCP-COV-001 | INFO | spans loaded but none looks like MCP; no MCP rule ran |
| MCP-PRIV-002 | INFO | Vendor attributes known to hold payloads (`traceloop.entity.*`, `mcp.response.value`); project heuristic |
| MCP-SEM-003 | WARN | Span name does not follow `{mcp.method.name} {target}` |
| MCP-SEM-004 | INFO | `tools/call` span lacks `gen_ai.operation.name=execute_tool` (Recommended) |
| MCP-PROP-003 | FAIL | `_meta.traceparent` / `tracestate` / `baggage` on the wire do not follow the W3C format (SEP-414 MUST). Needs `-wire` |
| MCP-PROP-004 | INFO | Captured requests carry no `_meta.traceparent` at all. Needs `-wire` |

Condition details, quoted normative text, and false-positive risks: `docs/06-architecture.md`. Rules without a live fixture (SEM-002, ERR-002, PROP-002) are covered by synthetic-span unit tests.

## Wire checks (`-wire`)

Spans never carry the raw `_meta` values, so the SEP-414 format rule needs a second input: a newline-delimited capture of received messages, one `{"method": ..., "_meta": {...}}` object per line. The Go harness writes it with `-capture file`; a proxy or SDK hook can emit the same shape.

```sh
go run ./cmd/mcp-reliability doctor -wire testdata/bad-traceparent.wire.jsonl testdata/bad-traceparent.jsonl
```

`doctor` also accepts several trace files, or `-` to read one from stdin. `-weaver-json out.json` additionally exports the spans for OTel Weaver attribute validation (`examples/weaver/README.md`).

## Regenerate fixtures

The fixtures in `testdata/` come from a real Go SDK client and server (`examples/go-sdk-streamable-http`) with one injected defect each, captured through a local collector. Needs Docker.

```sh
make fixtures
```

## Development

```sh
make check   # gofmt, vet, test -race, staticcheck, govulncheck
```

## Compatibility report

`docs/compatibility-report.md` lists every fixture, its producer (SDK or library and version), and the rules that fire. Regenerate with `make report`.

## Status and roadmap

M0 (PoC), M1 (v0.1.0, 2026-11-03), M2 (Python and TypeScript scenarios, compatibility report) and M4 (weaver export) are done. Next: upstream issues and PRs (M3). See `docs/07-roadmap.md`. Known limits are listed in `docs/06-architecture.md`, section 12.

## Non-goals

- No dashboard, backend, or hosted service.
- No web UI. A static HTML report may come later if people ask for it (ADR-0005).
- No LLM in the core (ADR-0004).
- Does not capture traffic; it analyzes traces you already exported.

## Contributing, security, license

- [CONTRIBUTING.md](CONTRIBUTING.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md): report vulnerabilities privately
- [LICENSE](LICENSE): Apache-2.0
- AI agents: [AGENTS.md](AGENTS.md)

## Acknowledgements

- The OpenTelemetry GenAI SIG for the MCP semantic conventions this tool checks against.
- The MCP specification, SEP-414, for trace context in `_meta`.
- Prior art: mcp-otel-go, opentel-mcp, and openinference, which instrument MCP and informed the research in `docs/research/`.
