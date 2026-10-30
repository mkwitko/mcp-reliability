# Project vision

Status: draft, 2026-10-30. Licence: Apache-2.0.

External claims carry a URL and the date the research files verified them (2026-10-30). Detail lives
in `docs/research/`. Statements marked HYPOTHESIS are not backed by a source.

## One sentence

`mcp-reliability` is a local-first Go CLI that reads a trace file and tells you whether your MCP
client and server emit the telemetry the MCP spec and OpenTelemetry say they should.

```
mcp-reliability doctor trace.jsonl
```

## The problem

MCP now has a normative rule for trace context. Spec revision 2026-07-28 reserves
`_meta.traceparent`, `tracestate` and `baggage`, and says their values MUST follow W3C formats
(https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx,
verified 2026-10-30). OpenTelemetry has MCP semantic conventions on top of that, status Development
(https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md, verified
2026-10-30).

Nothing checks either. The official conformance suite has zero telemetry coverage: no `traceparent`
hits in code or issues (https://github.com/modelcontextprotocol/conformance, verified 2026-10-30).
Weaver validates attributes but not across spans, and cannot read OTLP JSON files (see
`02-competitive-analysis.md`).

Meanwhile SDK maintainers keep hitting instrumentation bugs that a checker would flag:

- python-sdk #3174: JSON-RPC error responses leave client spans UNSET
  (https://github.com/modelcontextprotocol/python-sdk/issues/3174, verified 2026-10-30).
- typescript-sdk #1231: instrumentation marks successful requests as aborted
  (https://github.com/modelcontextprotocol/typescript-sdk/issues/1231, verified 2026-10-30).
- php-sdk #583: OTel context lost across handler fibers
  (https://github.com/modelcontextprotocol/php-sdk/issues/583, verified 2026-10-30).
- openllmetry #4374: client never injects context for ordinary tool calls
  (https://github.com/traceloop/openllmetry/issues/4374, verified 2026-10-30).

## Questions the tool answers

Given an OTLP JSON trace file, `doctor` answers, deterministically and with no network:

1. Is every `_meta.traceparent` / `tracestate` / `baggage` value in W3C format (SEP-414)?
2. Do MCP spans carry `mcp.method.name` (Required), and, where applicable, `gen_ai.tool.name`,
   `error.type`, `rpc.response.status_code` (conditionally required)?
3. For one MCP call, do the client span and the server span share a trace id?
4. Is the server span's parent span id present in the file, or is the server span orphaned or
   disconnected?
5. Does a failed tool call (`isError: true`, or a JSON-RPC error) leave the span status UNSET? This
   is the python-sdk #3174 pattern.
6. Are opt-in sensitive attributes (`gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`) present
   in a file that is about to be shared?

Output is a list of findings with rule id, span id and the spec text it comes from. Exit code is
non-zero on violations, so it works in CI.

## How we get test traces

`examples/` holds a scenario harness: a go-sdk v1.8.0 client and server over Streamable HTTP, with
hand-rolled OTel middleware built on `mcp.Middleware` and `Params.GetMeta/SetMeta`. Flags inject
controlled failures: no propagation, missing `error.type`, leaked arguments. Running `doctor` over
those traces produces a compatibility report. Other SDKs (Python v2 built-in OTel, TypeScript PR
#3115) become later scenarios. Sources for the SDK behaviour: `01-ecosystem-research.md`.

## Principles

- Local-first. Reads a file. No account, no collector, no network.
- Deterministic. Same file, same findings. No LLM in the rule path.
- Spec-cited. Every rule names the spec or semconv text it enforces, and its stability status.
- Narrow. We check contracts; we do not collect, store or display telemetry.
- Upstream first. Where a fix belongs in another project (see below), we send it there.

## What it will not become

- Not a backend, dashboard or trace viewer. Jaeger, Tempo, Langfuse, Phoenix and others do that.
- Not a new instrumentation library. Existing ones are small and fragmented, so the work goes
  upstream: client-side inject for mcp-otel-go, openllmetry #4374, go-contrib proposal #7616, and a
  conformance scenario for `_meta.traceparent` format.
- Not a replacement for weaver. We may call weaver for attribute validation later rather than
  duplicate it.
- Not an MCP server conformance suite or security scanner. That is the conformance repo and tools
  like checkmcp.
- Not an AI product. An AI layer (diagnosis, suggested fixes) is deferred until deterministic rules
  exist and demand shows. If it ever ships: BYOK, structured output, every claim tied to evidence
  refs (span ids), off by default.
- Not a proxy that rewrites traffic. The spec text says nothing about intermediaries injecting
  `traceparent` (HYPOTHESIS: permitted but untested; 2026-10-30 research file `mcp-ecosystem.md`
  section 1.3).

## Honest risk

Demand is moderate and comes from SDK maintainers, not end users. The research found no issue from
an end user asking for an MCP trace linter, and none in the conformance or weaver trackers
(https://github.com/modelcontextprotocol/conformance and https://github.com/open-telemetry/weaver,
verified 2026-10-30). Demand for a separate verification tool is unproven (research file
`platforms-and-tooling.md`, section C.5).

The semconv is Development and may change. If SDKs absorb correct instrumentation by default (Python
v2 already does; TypeScript PR #3115 is open), the need shrinks. The project is sized so that
failing costs little: a small CLI, a rule list, and upstream contributions that stay useful either
way.

## Success looks like

- At least one SDK maintainer uses `doctor` output in a bug report or CI job.
- A `_meta.traceparent` format scenario lands in the conformance suite.
- The compatibility report covers three or more SDKs.
