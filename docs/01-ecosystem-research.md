# Ecosystem research: state of MCP observability

Synthesis of three research files. Detail and method notes:

- `docs/research/mcp-ecosystem.md` (spec, SDKs, semconv)
- `docs/research/instrumentation-libraries.md` (libraries, propagation evidence)
- `docs/research/platforms-and-tooling.md` (platforms, trace formats, demand)

All claims below were verified on 2026-10-30 per those files, with the URL shown. HYPOTHESIS marks
inference not backed by a read source.

## 1. Spec

- Latest revision 2026-07-28 is stateless: no `initialize`, no `Mcp-Session-Id`; version and
  capabilities travel in `_meta` on every request (SEP-2567, SEP-2575). Source:
  https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/changelog.mdx
  (2026-10-30). Earlier revisions stay deployed, so an observer must handle both modes.
- `traceparent`, `tracestate`, `baggage` are reserved unprefixed `_meta` keys; values MUST follow
  W3C Trace Context and Baggage formats. Source:
  https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx
  (2026-10-30). SEP-414 merged 2026-02-26, status Final:
  https://github.com/modelcontextprotocol/modelcontextprotocol/pull/414 (2026-10-30). It is not in
  2025-11-25, so before 2026-07-28 this was a convention.
- The spec defines no telemetry export, metrics or telemetry capability. HTTP-header forwarding of
  `_meta` (SEP-2028) is open and unsponsored:
  https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2028 (2026-10-30).
- Two error channels: JSON-RPC `error.code`, and `result.isError` on `tools/call`. Source: spec
  `basic/index.mdx` above. OTel maps `isError` to `error.type = tool_error`.
- Timeouts have no wire error. Cancel is stream close (HTTP) or `notifications/cancelled` (stdio).
  HYPOTHESIS: a server-side observer sees cancels, not timeouts.

## 2. OTel semantic conventions

- Location:
  https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md
  (2026-10-30). The old `semantic-conventions` path is a stub.
- Status: Development. Nothing MCP-specific is Stable. `rpc.response.status_code` is Release
  Candidate.
- Required: `mcp.method.name`. Conditionally required: `error.type` (on failure),
  `gen_ai.tool.name`, `gen_ai.prompt.name`, `jsonrpc.request.id`, `mcp.resource.uri`,
  `rpc.response.status_code`. Opt-in and sensitive: `gen_ai.tool.call.arguments`,
  `gen_ai.tool.call.result`.
- Span names `{mcp.method.name} {target}`; kind CLIENT on the sender, SERVER on the receiver.
  Context is injected into `params._meta` with the configured propagator; the receiver uses it as
  remote parent.
- An MCP `tools/call` span is "compatible with" GenAI `execute_tool`:
  https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md
  (2026-10-30).
- Churn: open issue #437 says the semconv lags the 2026-07-28 protocol:
  https://github.com/open-telemetry/semantic-conventions-genai/issues/437 (2026-10-30). The method
  table still lists `initialize` and `ping` (HYPOTHESIS: `server/discover` and task methods are
  missing).

## 3. SDKs

| SDK | State | Source (all 2026-10-30) |
|---|---|---|
| Go v1.8.0 (2026-09-14) | No OTel. Maintainers declined it (go-sdk#59 `not_planned`; #934 "no SDK work needed"). Hooks: `Add{Sending,Receiving}Middleware`, `Meta map[string]any` with `GetMeta/SetMeta`. | https://github.com/modelcontextprotocol/go-sdk, issues #59, #934 |
| Go `Request.GetID` | On main, closed 2026-10-28 (#1264). HYPOTHESIS: not in v1.8.0; unverified. | https://github.com/modelcontextprotocol/go-sdk/issues/1264 |
| Python v2.3.0 (2026-10-02) | Built-in OTel server middleware, default-on (PR #2995), inject/extract via global propagators. v1.x has none. Open: #3715 outbound seam, #2387 richer attributes. | https://github.com/modelcontextprotocol/python-sdk |
| TypeScript 2.4.0 (2026-10-23) | Constants for trace keys only. Built-in tracing is open PR #3115 (2026-10-27), not released. | https://github.com/modelcontextprotocol/typescript-sdk/pull/3115 |
| Java | Instrumentation PRs #20062, #20100 open, unmerged. | https://github.com/open-telemetry/opentelemetry-java-instrumentation |

SEP-414 tracking (#2868) lists TS, C#, Go, Java, PHP, Python, Ruby and Rust ticked; Kotlin and Swift
unticked: https://github.com/modelcontextprotocol/modelcontextprotocol/issues/2868 (2026-10-30). The
Go "implementation" is "no SDK work needed".

OTel contrib has no merged MCP package in Go, Python or JS. go-contrib #7616 is open and idle since
2025-09: https://github.com/open-telemetry/opentelemetry-go-contrib/issues/7616 (2026-10-30).

## 4. Instrumentation libraries

Wire contract is shared: nearly every library reads or writes `_meta.traceparent` via a propagator.
Source: `docs/research/instrumentation-libraries.md` section 1 and 3 (2026-10-30).

- Compatible on the wire: mcp-otel-go, opentel-mcp, openllmetry, openinference,
  AmirK-S/mcp-opentelemetry, Python SDK built-in.
- Outliers: dylibso/mcp-otel uses `_meta.__traceContext` (https://github.com/dylibso/mcp-otel);
  AgentCat derives trace id from `sha256(sessionId)` and never reads `_meta.traceparent`
  (https://github.com/agentcathq/agentcat-typescript-sdk).
- Failure mode A, one-sided: mcp-otel-go (https://github.com/olgasafonova/mcp-otel-go) and
  opentel-mcp (https://github.com/Thirumalaiboobathi/opentel-mcp) only extract. No Go client-side
  injector found (HYPOTHESIS: none exists).
- Failure mode B, silently broken: openllmetry client injection is gated on `params.meta` being
  non-None, so ordinary calls inject nothing: https://github.com/traceloop/openllmetry/issues/4374.
- Failure mode C, no-op default: the OTel global propagator is a no-op until set; mcp-otel-go
  documents it.
- Span shape differs: openllmetry uses `{x}.tool` names and `traceloop.*` attributes, not the OTel
  MCP convention.
- Notifications: only AmirK-S injects for them; mcp-otel-go skips four notification methods.

## 5. Platforms

Two groups, per `docs/research/platforms-and-tooling.md` section 0 (2026-10-30):

- Display MCP traffic: MCPJam (https://github.com/MCPJam/inspector), Sentry
  `wrapMcpServerWithSentry`, Datadog MCP Clients page, Grafana AI Observability. The last three are
  SEARCH-SUMMARY in the research file (vendor pages not read in full).
- Generic OTLP backends: Langfuse, Phoenix, LangSmith, Braintrust, Tempo, Jaeger. No MCP-specific
  span understanding found. Langfuse documents injecting W3C context into `_meta` (SEARCH-SUMMARY):
  https://langfuse.com/docs/observability/features/mcp-tracing.

Many "MCP" issues in Tempo, Jaeger, Phoenix and Langfuse concern those tools' own MCP servers, not
observability of MCP.

## 6. Validation tooling

- Conformance suite: wire-schema checks on JSON-RPC messages; no telemetry.
  https://github.com/modelcontextprotocol/conformance (2026-10-30).
- Weaver `registry live-check`: semconv validation of spans/attributes; OTLP gRPC listener or its
  own sample JSON only. https://github.com/open-telemetry/weaver (2026-10-30).
- No OSS MCP-aware trace linter found in GitHub searches (suggestive, not proof).

## 7. Trace file formats

- Collector `fileexporter` writes newline-delimited OTLP JSON, one batch per line; alpha stability,
  "no guarantee that exact field names will remain stable":
  https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter
  (2026-10-30).
- OTLP JSON uses hex `traceId`/`spanId`, not base64:
  https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md
  (2026-10-30).
- Go: `ptrace.JSONUnmarshaler.UnmarshalTraces` in `go.opentelemetry.io/collector/pdata`:
  https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/ptrace/json.go
  (2026-10-30).

## 8. Demand

Strong that the problem is recognised; moderate that maintainers hit bugs; thin that anyone wants a
linter. Itemised in `docs/research/platforms-and-tooling.md` Part C and summarised in
`00-project-vision.md`.

## 9. Open questions carried forward

1. Which go-sdk release first contains `Request.GetID()`?
2. Is a proxy allowed to inject or rewrite `traceparent`? Spec silent.
3. Will TS PR #3115 land, and default-on?
4. Will the semconv cover 2026-07-28 methods and drop session metrics?
5. Python SDK v2 client-side output: only shared helpers and server middleware were read.
