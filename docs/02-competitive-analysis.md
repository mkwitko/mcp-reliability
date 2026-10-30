# Competitive analysis

All data from `docs/research/*.md`, verified 2026-10-30. Source keys `[n]` resolve to URLs in the
list after the matrix. "Active" is last push or release near 2026-10-30. "?" means not found in the
research. Entries marked (SS) rest on a search summary of a vendor page, not a page read in full.

## Matrix

| Project | What it does | OTLP native | MCP-aware | Propagation mechanism | Validates traces | Interop testing | License | Active |
|---|---|---|---|---|---|---|---|---|
| MCP conformance suite [1] | Wire-protocol checks on JSON-RPC traffic | No | Yes (protocol only) | None tested | No (messages only) | Yes, protocol not telemetry | MIT to Apache-2.0 transition notice | Yes (2026-10-26) |
| Weaver live-check [2] | Validates telemetry samples against a semconv registry | gRPC listener only; HTTP is open issue #1563 | Only via registry in semconv-genai | None | Yes, per sample; no cross-span checks; span-type work open (#1851, #1859) | No | Apache-2.0 | Yes (2026-10-29) |
| mcp-otel-go [3] | Go server middleware: spans + duration metric | Via OTel SDK | Yes | `_meta` extract only (propagator) | No | No | MIT | Yes (2026-10-11), 5 stars, solo |
| opentel-mcp [4] | TS/Python server spans, silent-failure detection | Via OTel SDK | Yes | `_meta.traceparent`/`tracestate` extract only | No | Own conformance suite shared Py/JS | MIT (no root LICENSE) | Yes (2026-10-26), 6 stars, solo |
| openllmetry MCP [5] | Python auto-instrumentation | Yes | Yes | `_meta` inject + extract; client inject broken (#4374) | No | No | Apache-2.0 | Yes (2026-10-19); open MCP bugs |
| openinference MCP [6] | Context propagation only, no spans | Yes | Yes | `_meta` inject + extract, global propagator | No | No | Apache-2.0 | Yes (2026-10-30) |
| AgentCat [7] | MCP product analytics SaaS + optional OTLP export | Optional exporter | Yes | None; trace id = sha256(session id) | No | CI compat workflow only | MIT (SDKs) | Yes (2026-10-30) |
| dylibso/mcp-otel [8] | Demo of tracing over MCP | ? | Yes | Custom `_meta.__traceContext` | No | No | BSD-3-Clause | No (2026-02-14) |
| Python SDK built-in [9] | Default-on server OTel middleware, inject/extract helpers | Via OTel API | Yes | `_meta` inject + extract | No | Own tests | MIT (HYPOTHESIS, unchecked) | Yes (v2.3.0 2026-10-02) |
| TS SDK PR #3115 [10] | Proposed built-in tracing and metrics | Via OTel | Yes | `params._meta` | No | ? | ? | Open PR, not released |
| MCPJam inspector [11] | Interactive MCP test/eval platform | No evidence | Yes | None (own message timeline) | No telemetry validation | OAuth conformance, CI runner | Custom, mixed | Yes (2026-10-30) |
| Langfuse [12] | LLM observability backend | Traces only | No MCP-specific attribute handling found | Docs advise `_meta` injection (SS) | No | No | MIT + ee (NOASSERTION) | Yes |
| Phoenix [13] | LLM observability backend | Yes (HYPOTHESIS on endpoints) | Via openinference only | Via openinference | No | No | Elastic License 2.0 | Yes |
| LangSmith [14] | LLM tracing platform | Traces only (SS) | None found | None | No | No | Proprietary; SDK repo archived 2026-10-16 | Platform yes |
| Braintrust [15] | LLM evals/tracing platform | Traces only (SS) | None found | None | No | No | Proprietary; SDK Apache-2.0 | ? |
| Grafana Tempo [16] | Trace backend | Yes | No | None | No | No | AGPL-3.0 | Yes |
| Jaeger [17] | Trace backend | Yes | No | None | No | No | Apache-2.0 | Yes |
| Sentry [18] | Error/perf platform, MCP monitoring in JS/Python SDKs | Capable; MCP via own SDK | Yes | SDK-based, `mcp.*` spans (SS) | No | No | FSL (not re-verified) | Yes |
| Datadog [19] | LLM Obs "MCP Clients" page | ? | Yes, client side (SS) | Auto-instrumentation (SS) | No | No | Proprietary | Yes |
| Grafana AI Observability [20] | Cloud dashboard for LLM/MCP via OpenLIT | Yes | Yes (SS) | ? | No | No | Proprietary (Cloud) | Yes |
| checkmcp [21] | 25 spec-cited server conformance checks | No | Yes | None | No telemetry checks | Active probes, not telemetry | MIT | Yes (2026-09-06), 1 star |

## Sources (verified 2026-10-30)

1. https://github.com/modelcontextprotocol/conformance
2. https://github.com/open-telemetry/weaver (`crates/weaver_live_check/README.md`); issues #1563,
   #1851, #1859 under the same repo
3. https://github.com/olgasafonova/mcp-otel-go
4. https://github.com/Thirumalaiboobathi/opentel-mcp
5. https://github.com/traceloop/openllmetry, issue
   https://github.com/traceloop/openllmetry/issues/4374
6. https://github.com/Arize-ai/openinference
7. https://github.com/agentcathq/agentcat-typescript-sdk
8. https://github.com/dylibso/mcp-otel
9. https://github.com/modelcontextprotocol/python-sdk (`src/mcp/shared/_otel.py`,
   `src/mcp/server/_otel.py`)
10. https://github.com/modelcontextprotocol/typescript-sdk/pull/3115
11. https://github.com/MCPJam/inspector
12. https://github.com/langfuse/langfuse ;
    https://langfuse.com/docs/observability/features/mcp-tracing
13. https://github.com/Arize-ai/phoenix
14. https://docs.langchain.com/langsmith/trace-with-opentelemetry
15. https://github.com/braintrustdata/braintrust-sdk
16. https://github.com/grafana/tempo
17. https://github.com/jaegertracing/jaeger
18.
    https://github.com/getsentry/sentry-javascript/blob/develop/packages/core/src/integrations/mcp-server/index.ts
19. https://docs.datadoghq.com/llm_observability/monitoring/mcp_client
20. https://grafana.com/docs/grafana-cloud/monitor-applications/ai-observability/
21. https://github.com/gregkozakiewicz/checkmcp

Other same-name checkmcp repos exist (H129hj/checkmcp, phamcongdanh98/CheckMCP); no canonical one.
Not read beyond descriptions.

## Reading the matrix

- Validation of traces: only weaver does it. No project checks across spans or reads OTLP JSON
  files.
- Interop testing: no project tests telemetry interop. The conformance suite tests protocol; the
  others test their own code.
- Propagation: the shared contract is `_meta.traceparent`. Outliers are dylibso (custom key) and
  AgentCat (no wire propagation).
- Backends (Langfuse, Phoenix, Tempo, Jaeger, LangSmith, Braintrust) consume whatever is emitted;
  none found that checks it.

## Actual gaps, with evidence

1. **No telemetry coverage in the official conformance suite.** README search for `traceparent`,
   `telemetry`, `otel` returned zero hits; code search for `traceparent` returned zero; issue search
   for telemetry/observability/trace/otel/SEP-414 returned none. The `request-metadata` scenario
   tests `io.modelcontextprotocol/*` keys, not trace keys. Source:
   https://github.com/modelcontextprotocol/conformance.
2. **Weaver cannot read an OTLP JSON file.** Inputs are an OTLP gRPC listener, text, or weaver's own
   sample JSON. OTLP/HTTP is open request https://github.com/open-telemetry/weaver/issues/1563.
   Source: weaver README.
3. **No cross-span checks anywhere.** Weaver assesses each sample against the registry; no check for
   remote parent, shared trace id or orphans was found. Span-type support is still open:
   https://github.com/open-telemetry/weaver/issues/1851,
   https://github.com/open-telemetry/weaver/issues/1859. This is the absence of docs, so treat as
   HYPOTHESIS that it is a deliberate non-goal.
4. **Silent propagation failures are real and unchecked.** openllmetry #4374 (client never injects);
   mcp-otel-go and opentel-mcp are extract-only; no Go client-side injector found. Source:
   `docs/research/instrumentation-libraries.md` section 3.
5. **Span status bugs recur.** python-sdk #3174 (error responses leave client span UNSET):
   https://github.com/modelcontextprotocol/python-sdk/issues/3174. Python SDK also needed #3629 to
   record JSON-RPC errors on client spans:
   https://github.com/modelcontextprotocol/python-sdk/pull/3629 (listed as closed in the research
   file; PR vs issue not distinguished there).
6. **Two non-interoperable outliers.** dylibso `__traceContext` and AgentCat session-hash trace ids
   cannot join a caller's trace.
7. **No OSS MCP-aware trace linter found** in GitHub searches for "otel trace lint", "span
   validator", "otlp json cli analyzer" (https://github.com, searched 2026-10-30; absence is
   suggestive only). Adjacent: Tracetest semconv analyzer plugin, generic and dormant since
   2025-06-03 (https://github.com/kubeshop/tracetest); a hosted Apify semconv validator
   (https://apify.com/michiganpublictech/otel-semconv-validator, SS).

What is not a gap: server-side extract in Go (mcp-otel-go), Python server spans (SDK built-in). See
`03-problem-opportunities.md`.
