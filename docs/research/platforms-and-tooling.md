# Platforms, tooling, trace formats and demand evidence for MCP observability

Research date: 2026-10-30. All "verified" dates below are 2026-10-30 unless stated.

Evidence labels:

- **CONFIRMED**: I read the repository file, doc page or issue directly (via `gh api` or a web fetch).
- **SEARCH-SUMMARY**: the claim comes from a web-search result summary of a vendor page that I did not open in full. Treat as probable, not verified.
- **HYPOTHESIS**: my inference. Needs a spike before relying on it.

---

## 0. Executive summary

1. **The official conformance suite does not test observability or telemetry at all.** CONFIRMED: no `traceparent`, `baggage` or OpenTelemetry hits in the conformance README or its scenario sources, and no open or closed issue about tracing. It is a wire-protocol suite (schema checks on every JSON-RPC message, lifecycle, tools, auth, SSE). See section A.9.
2. **But the spec now mandates trace context in `_meta`.** CONFIRMED: MCP spec 2026-07-28 reserves `traceparent`, `tracestate` and `baggage` in `_meta` and requires W3C formats when present (SEP-414, status Final). So there is a normative rule that nothing in the official conformance suite checks. That is a real gap.
3. **OTel has an MCP semantic convention and, importantly, a machine-readable registry for it.** CONFIRMED: `open-telemetry/semantic-conventions-genai` contains `model/mcp/{registry,spans,metrics,common}.yaml`. Status is "Development". An open issue (#437) says it lags the 2026-07-28 protocol (stateless, no session).
4. **`weaver registry live-check` already does semconv validation of spans, attributes, metrics and logs against a registry**, and accepts custom Rego policies. CONFIRMED. We should not rebuild attribute-level semconv validation. What it does not do: read OTLP JSON files (its file input is its own sample JSON, its OTLP input is a live gRPC listener only), and it does not check trace topology (parent/child linkage, propagation, orphan spans). Details in B.2.
5. **No existing tool checks MCP trace propagation correctness.** I found no GitHub tool that checks "does the server span have the client span as parent across the MCP hop". Tracetest has a semconv analyzer plugin (generic), and an Apify actor validates OTLP JSON against semconv. Neither is MCP-aware as far as I could verify.
6. **Platforms split into two groups.** (a) Tools that display MCP traffic (MCPJam, Sentry, Datadog, Grafana AI Observability) and (b) generic OTLP backends (Langfuse, Phoenix, LangSmith, Braintrust, Tempo, Jaeger, Honeycomb) with no MCP-specific understanding found. Several of the "MCP" items in the Tempo, Jaeger and Phoenix trackers are MCP servers that expose the backend to agents, not observability of MCP.
7. **Demand evidence is moderate, not thin, but it is SDK-maintainer demand, not end-user pain.** Many SDK issues exist (SEP-414 rollout across 9 SDKs, TS SDK #1264, Python SDK #3715). Genuine "my traces are broken" developer complaints are mostly blog posts, not issues. No Reddit or HN threads surfaced. Details in Part C.
8. **Go parsing is straightforward.** CONFIRMED: `go.opentelemetry.io/collector/pdata/ptrace` has `JSONUnmarshaler.UnmarshalTraces`, and `go.opentelemetry.io/proto/otlp` + `protojson` works too. Fileexporter output is newline-delimited `ExportTraceServiceRequest` JSON.

---

## Part A. Platforms

### A.1 MCPJam Inspector and the official MCP Inspector

| Item | MCPJam inspector | modelcontextprotocol/inspector |
|---|---|---|
| Repo | https://github.com/MCPJam/inspector (2,296 stars, pushed 2026-10-30) | https://github.com/modelcontextprotocol/inspector (11,196 stars, pushed 2026-10-29) |
| License | Custom: LICENSE says "Portions of this software are licensed as follows" (GitHub reports NOASSERTION). Not a plain OSS license; read before reuse. | MIT to Apache-2.0 transition (LICENSE text, GitHub reports NOASSERTION) |
| What it does for MCP | "Testing and evaluation platform to chat, inspect, and debug MCP servers, MCP apps, and ChatGPT apps". Playground with Chat/Trace/Raw views, OAuth conformance debugger (protocol versions 2025-03-26 to 2026-07-28), CI runner for conformance, E2E tests and evals. | "Deterministic testing tools for MCP servers". Interactive tool/resource/prompt tester. |
| OTLP ingest | No evidence. Its "trace" is a timeline of JSON-RPC messages and agent steps inside its own UI. A code search for otlp/opentelemetry in the repo returned nothing. | No evidence found. Search for otel/tracing issues returned nothing. |
| MCP span/attribute understanding | N/A (not OTel based) | N/A |
| Interop / trace validation | OAuth conformance checks and "conformance" runs in CI (README). No trace validation. | None found |
| Self-host | Yes (repo has Dockerfile-style deployment assets and railway.json; hosted version also exists) | Yes, runs locally via npx |
| Relevant issues | MCPJam #2273 "Export prompt tracing" (open, 2026-05-25). This is the only trace-related item I found. | None found |

Sources: https://github.com/MCPJam/inspector (README, LICENSE), https://github.com/MCPJam/inspector/issues/2273, https://github.com/modelcontextprotocol/inspector. Verified 2026-10-30. CONFIRMED (README text read; "no OTLP" is absence of search hits, so treat as strongly likely rather than proven).

Relevance: these are interactive debuggers. They show what is on the wire for one session. They do not read OTel traces from production.

### A.2 Langfuse

- Repo https://github.com/langfuse/langfuse, 36,766 stars, pushed 2026-10-30. License: MIT with ClickHouse copyright and "portions licensed as follows" (ee folder is separate). GitHub reports NOASSERTION. Self-hostable (Helm values appear in issue #13759). CONFIRMED (LICENSE header, issue list).
- OTLP: accepts OTLP traces at `/api/public/otel`. Traces only, no metrics or logs (a third-party doc says metrics/log exports return errors). SEARCH-SUMMARY, source https://langfuse.com/docs/opentelemetry/introduction and https://docs.docker.com/ai/docker-agent/community/opentelemetry.md.
- MCP specific: has a docs page "MCP tracing" (https://langfuse.com/docs/observability/features/mcp-tracing). Per the search summary it states client and server traces are separate by default, and recommends injecting W3C trace context into `_meta` so they link. This is guidance on propagation, not a validator. SEARCH-SUMMARY.
- MCP span understanding: no MCP-specific attribute handling found. HYPOTHESIS that it maps `gen_ai.*` generically.
- Interop testing / linting: none found.
- MCP-tracing issues: none about MCP tracing as a feature. The "MCP" issues in the Langfuse repo are about Langfuse's own MCP server (auth, tool allowlist) and one about tracing Codex Desktop MCP calls: https://github.com/langfuse/langfuse/issues/17969 (open, 2026-09-26, "Codex Desktop code-mode MCP calls omitted from tracing plugin").

### A.3 Braintrust

- SDK repo https://github.com/braintrustdata/braintrust-sdk (Apache-2.0, 29 stars, JS tracing and evals library). The platform itself is a hosted product; I did not verify a self-host option (self-host exists only as a paid enterprise tier, HYPOTHESIS).
- OTLP: traces endpoint `https://api.braintrust.dev/otel/v1/traces`, traces only, `x-bt-parent` header for project. SEARCH-SUMMARY, https://www.braintrust.dev/docs/kb/ingest-otlp-log-records-into-braintrust.
- MCP-specific features: none found. Interop testing: none found. Open MCP tracing issues: none found.

### A.4 LangSmith

- OTLP endpoint `https://api.smith.langchain.com/otel`, `x-api-key` header, self-host base path `/api/v1`. SEARCH-SUMMARY, https://docs.langchain.com/langsmith/trace-with-opentelemetry. Traces only (same Docker doc as above).
- SDK repo `langchain-ai/langsmith-sdk` is MIT but **archived 2026-10-16**; SDKs moved to `langsmith-python` and `langsmith-javascript`. CONFIRMED via `gh api`.
- Platform is proprietary, self-host is an enterprise option. SEARCH-SUMMARY.
- MCP-specific: none found. Interop testing: none found.

### A.5 Arize Phoenix and OpenInference

- Phoenix: https://github.com/Arize-ai/phoenix, 12,077 stars. License **Elastic License 2.0** (LICENSE read), which restricts offering it as a managed service. Self-hostable. CONFIRMED.
- OTLP: native (it is OpenInference/OTel based). HYPOTHESIS for exact endpoints, not checked.
- MCP specifics: the only MCP-related instrumentation is `openinference-instrumentation-mcp` (Python). Its README says it "only enables context propagation so that the span active when making an MCP tool call can be connected to those generated when executing it. It does not generate any telemetry." CONFIRMED. https://github.com/Arize-ai/openinference/tree/main/python/instrumentation/openinference-instrumentation-mcp
- Issues: openinference #1616 "MCP instrumentation and context propagation is not working" (closed 2025-05-14); #1477 "mcp python instrumentation" (open, 2025-04-03); #1543 "MCP sampling propagation" (open, 2025-04-21). Most Phoenix "[mcp]" issues (e.g. #13975, #16596) are about Phoenix's own MCP server, not tracing MCP.
- Interop testing / linting: none found.

### A.6 Grafana Tempo, Grafana AI Observability, Jaeger

- **Tempo**: https://github.com/grafana/tempo, AGPL-3.0, self-hostable, OTLP native. CONFIRMED (license via API). No MCP-tracing issues. Tempo issues mentioning "mcp" are about the Tempo MCP server's TraceQL docs (#7375). That is the "agent queries traces" direction.
- **Grafana AI Observability** (Grafana Cloud): per docs it covers "LLMs, vector databases, GPUs and MCP servers", has an "MCP Observability" dashboard and uses the OpenLIT SDK. SEARCH-SUMMARY, https://grafana.com/docs/grafana-cloud/monitor-applications/ai-observability/. I could not confirm whether it uses OTel's MCP semconv attribute names (`mcp.method.name` etc.) or OpenLIT's own. HYPOTHESIS: OpenLIT's own naming.
- **Jaeger**: https://github.com/jaegertracing/jaeger, Apache-2.0, OTLP native (v2 is built on the OTel Collector). CONFIRMED license. Open issues mention an MCP server and "AI gateway" (#9957, #9639, #9215): again Jaeger exposed to agents, not MCP observed by Jaeger. No MCP-span understanding.

### A.7 "CheckMCP"

No single canonical project. GitHub has several same-name repos (all small):

- https://github.com/gregkozakiewicz/checkmcp (MIT, 1 star, pushed 2026-09-06): "npx checkmcp", 25 spec-cited conformance checks (handshake, schemas, errors, robustness, security) against spec 2025-11-25; also an in-memory test harness. Active probes with invalid arguments. **No telemetry or trace checks** in the README I read. CONFIRMED.
- https://github.com/H129hj/checkmcp (MIT, 0 stars): "quality/security/context-cost audit & score". README not read; description only.
- https://github.com/phamcongdanh98/CheckMCP: empty description, 0 stars.
- https://github.com/treffinger/CheckMCP is a Checkmk monitoring MCP server (name collision, unrelated).

Related server checkers surfaced by web search (SEARCH-SUMMARY): mcp-server-checker (50+ checks), MCP Release (`npx -y @mcp-release/cli check`), jensneuhaus/mcpchecker. All check server behavior, not traces.

### A.8 Sentry, Datadog, Honeycomb

- **Sentry**: first-class "MCP monitoring" for JS and Python. `Sentry.wrapMcpServerWithSentry()` creates spans with op `mcp.server`, names like `tools/call <name>`, attributes such as `mcp.tool.name` and `mcp.tool.result.is_error`; `recordInputs`/`recordOutputs` privacy flags. Source file exists: https://github.com/getsentry/sentry-javascript/blob/develop/packages/core/src/integrations/mcp-server/index.ts (path confirmed via code search). Docs: https://docs.sentry.io/platforms/javascript/guides/nextjs/mcp-monitoring/ (SEARCH-SUMMARY). Sentry's own spans use `mcp.*` attributes; whether they are fully aligned with the OTel MCP semconv is HYPOTHESIS. Sentry is OTLP-capable but MCP monitoring is SDK-based. Self-host: Sentry is self-hostable under FSL. (not re-verified)
- **Datadog**: "MCP Clients" page under LLM/Agent Observability. Tool spans with server name, auto-instrumentation for official Python/JS SDKs via ddtrace/dd-trace; tool spans linked to parent LLM span; tool spans not billed. SEARCH-SUMMARY, https://docs.datadoghq.com/llm_observability/monitoring/mcp_client. Client-side focus. Proprietary, not self-hostable.
- **Honeycomb**: no Honeycomb-built MCP feature found. It is a generic OTLP target in third-party guides. SEARCH-SUMMARY, https://mcpcat.io/guides/monitor-mcp-performance-opentelemetry.

### A.9 modelcontextprotocol/conformance (IMPORTANT)

Repo https://github.com/modelcontextprotocol/conformance, 139 stars, pushed 2026-10-26, license text is the MCP MIT-to-Apache-2.0 transition notice. Verified 2026-10-30. CONFIRMED unless noted.

**What it tests.** README overview: for clients it starts a test server, runs the client, captures MCP protocol interactions and checks them against the spec. For servers it connects as a client, sends test requests, checks responses. CLI: `npx @modelcontextprotocol/conformance client|server --scenario ... --suite all|core|extensions|backcompat|auth|metadata|draft|sep-835 --spec-version 2025-11-25|2026-07-28`. Supports `--expected-failures` baseline YAML and a `--requirements <revision>` mode.

Server scenario files (`src/scenarios/server/`): caching, dns-rebinding, elicitation-defaults/enums, http-standard-headers, input-required-result, json-schema-2020-12, lifecycle, prompts, resources, session-lifecycle, sse-multiple-streams, sse-polling, stateless, tasks, tools, skills. Client scenario files: auth, draft-result-fields, elicitation-defaults, http-base, http-custom-headers, http-standard-headers, initialize, json-schema-2020-12-preservation, json-schema-ref-deref, mrtr-client, request-metadata, sse-retry, tools_call, skills.

**Wire-schema checks.** Every scenario validates each JSON-RPC message against the spec JSON schema for the negotiated version and emits synthetic `wire-schema-valid` and `wire-schema-harness-error` checks. This is schema validation of messages, not of telemetry.

**Is observability or telemetry in scope? No.** Evidence:

- Searching the README for `traceparent`, `telemetry`, `opentel`, `otel` returned zero hits.
- `src/scenarios/client/request-metadata.ts`, `server/lifecycle.ts` and `server/stateless.ts` contain no `trace`, `baggage` or `414` strings.
- Code search for `traceparent` in the repo returned 0 results.
- Issue search over the repo for telemetry, observability, trace, otel, opentelemetry, traceparent returned `[]`. Search for "SEP-414" returned nothing.
- The suite has no OTLP receiver, no span capture, nothing that reads telemetry. It only captures MCP protocol traffic.

**Subtlety.** `request-metadata` exists, so `_meta` handling is tested in principle. But `_meta` checks are about the protocol-level fields (`io.modelcontextprotocol/*` keys), not the reserved `traceparent`/`tracestate`/`baggage` keys. Related open issue: https://github.com/modelcontextprotocol/conformance/issues/506 "Everything-server rejects valid _meta on stateful initialize" (open, 2026-09-18). Not about trace context, but shows the `_meta` surface is under active work.

Other relevant conformance issues (all open): #418 "Wire-schema validation does not see raw-HTTP / inline-mock scenario traffic" (2026-07-27), #435 "Add conformance scenario: logging/setLevel and log notifications" (2026-08-02), #266 / #296 SEP-2575 tests. The maintainers are busy with the 2026-07-28 draft (stateless lifecycle), auth (DPoP, WIF), and header validation.

**Implication.** The spec rule (traceparent/tracestate/baggage in `_meta` MUST follow W3C formats) has no automated checker in the official suite. HYPOTHESIS: a PR adding a `trace-context-meta` scenario would need the harness to observe a request's `_meta` and, to check the propagation end to end, an OTLP side channel the harness does not have. A format-only check (is `traceparent` a valid W3C string when present) fits the existing wire-capture design. This is a possible upstream contribution rather than a competitor.

---

## Part B. Trace file formats and analyzers

### B.1 OTLP JSON file encoding

**Collector `fileexporter`** (https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter, README read). CONFIRMED:

- Stability: traces, metrics, logs are `alpha`; profiles `development`. "There is no guarantee that exact field names will remain stable."
- File format table: with no compression and `format: json` (default) with no `encoding`, the file is "Newline-delimited OTLP JSON, one line per batch". `format: proto` is length-prefixed (4-byte big-endian size + message). With `exporter.file.nativeCompression` enabled and `compression: zstd`, JSON lines sit inside a standard zstd stream. With nativeCompression disabled, zstd output is per-batch compressed and length-prefixed and "not readable by `zstd -d`".
- Supports rotation (`rotation:` block, default 100 MB), and `group_by` resource attribute to split files.
- `otlpjsonfilereceiver` (https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/otlpjsonfilereceiver): reads "Protobuf JSON encoding" files serialised per the OTLP File Exporter spec; watches a directory by `include` glob; if a file changes it re-reads the whole file; per the fileexporter README it "only reads uncompressed, newline-delimited OTLP JSON". Contrib distribution only, traces/metrics/logs alpha.

**Spec for the file format**: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/file-exporter.md. Status "Development". Describes JSON lines where each line is a `TracesData`/`MetricsData`/`LogsData` message. Note the nuance: the spec says each line is a `*Data` message (`resourceSpans` at top level), and the collector writes the same shape as `ExportTraceServiceRequest` (also `resourceSpans` at top level), so the two are interchangeable for traces. I read the spec header; the exact per-line type wording I did not read in full.

**Protobuf JSON mapping** (https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md, "JSON Protobuf Encoding"). CONFIRMED: standard proto3 JSON mapping with the deviation that `traceId`/`spanId` are case-insensitive hex strings, not base64. Remaining standard behavior (from the proto3 JSON spec, HYPOTHESIS to be checked in a spike): lowerCamelCase field names, parsers should also accept the original snake_case names, enums as integers or names, 64-bit ints (`startTimeUnixNano`) as decimal strings, attributes as `[{"key":"k","value":{"stringValue":"v"}}]` with `intValue` as a string, unknown fields ignored.

Implication for a parser: be lenient. Accept both ints and strings for nano timestamps, both `kind: 2` and `SPAN_KIND_SERVER`, empty `parentSpanId` as root, and do not assume one line equals one trace (a line is a batch, and spans of one trace are spread across lines and files).

### B.2 `weaver registry live-check` (open-telemetry/weaver)

Repo https://github.com/open-telemetry/weaver, Apache-2.0, 521 stars, pushed 2026-10-29. Docs read: `crates/weaver_live_check/README.md` and `docs/usage.md`. CONFIRMED.

**What it does.** "Live check is a developer tool for assessing sample telemetry and providing findings for improvement." It loads a semantic convention registry, ingests samples, runs advisors, and outputs a report. Signals covered: metric, span, log, resource (finding `signal_type` values). Finding levels: `violation`, `improvement`, `information`; non-zero exit on any violation by default (`--fail-on` adjustable). Built-in advisors include `missing_attribute` ("Attribute `hello` does not exist in the registry"); report mentions checks like undefined enum variants, deprecation, stability ("not_stable").

**Input formats** (README table):

| `--input-source` | `--input-format` | Meaning |
|---|---|---|
| `otlp` (default) | n/a | Starts an OTLP **gRPC** listener; stream messages; stop on SIGINT/SIGHUP, `/stop` admin endpoint or inactivity timeout |
| file path | `text` | Attribute names or `name=value` pairs |
| stdin | `text` | same |
| file path | `json` | "JSON file with an array of samples" (weaver's own sample schema, e.g. `crates/weaver_live_check/data/span.json`) |
| stdin | `json` | same |

The report README states the live-check report format "is not OTLP JSON or OTLP protobuf, and it is not directly ingestible by an OTLP" receiver. Weaver's `--input-format json` is not documented as OTLP JSON. So **weaver cannot directly read a fileexporter file**. OTLP/HTTP input is an open request: https://github.com/open-telemetry/weaver/issues/1563 (open, 2026-07-04, "support OTLP/HTTP (protobuf) input in addition to gRPC").

**Custom registry and policies.** `--advice-policies <dir>` loads Rego policies in package `live_check_advice`; policy input includes `input.sample.span`, `input.resource`, `input.instrumentation_scope`, `input.registry_attribute`, `input.registry_group`, plus registry-derived `data`. `--advice-data` loads extra JSON/YAML into OPA data. **Matchers** (CEL expressions in `.weaver.toml`, `[[live-check.matchers]]`) pair a span with a registry signal because span names are free-form; matchers need a v2 registry. An issue lists related work: #1851 "Support span type in live-check" (open, 2026-10-21), #1859 "validate span names against span name templates" (open, 2026-10-26). So span-level checking is still being built.

**Could it validate MCP spans against a custom registry?**

- CONFIRMED: an MCP registry already exists in YAML in `open-telemetry/semantic-conventions-genai` (`model/mcp/registry.yaml`, `spans.yaml`, `metrics.yaml`, `common.yaml`; also `docs/registry/attributes/mcp.md`). Weaver is that repo's own validation tool (it has `policies/` and `templates/`).
- HYPOTHESIS (not tried): `weaver registry live-check -r <semconv-genai registry>` with a CEL matcher such as `when = '"mcp.method.name" in attributes'` should validate attribute names, enum values and requirement levels on MCP spans. Needs a v2-format registry; I did not confirm whether `model/mcp` is v1 or v2 format.
- HYPOTHESIS: to feed an OTLP JSON file to weaver, replay the file through an OTel Collector (`otlpjsonfile` receiver to `otlp` gRPC exporter pointed at weaver's listener). This works only because weaver speaks gRPC. It needs a Collector binary, which defeats a "single local CLI" story.

**What weaver does not do** (CONFIRMED by absence in docs; HYPOTHESIS that it is a deliberate non-goal): it assesses each sample entity against the registry. I found no cross-span check: no "this server span has a remote parent", "client and server spans share a trace id", "span count per trace", "orphan spans", or "`traceparent` present in `_meta` and equal to the span context". Those are exactly the MCP propagation failures people complain about (Part C). Weaver's #1567 "scenario driver" issue is about test orchestration, not topology.

**Conclusion on duplication.** Attribute-level and enum-level semconv validation for MCP spans: weaver already covers this in principle, so do not reimplement it. If we want to offer it, integrate (emit a weaver-compatible sample JSON or shell out to weaver). The uncovered, defensible niche is trace-topology and protocol-level reliability checks on stored traces (propagation correctness, disconnected spans, `isError` results vs span status, missing `_meta` context), plus reading files directly.

### B.3 Other trace formats and viewers

- **Jaeger JSON** (UI export / upload): shape `{"data":[{"traceID","spans":[{"spanID","operationName","references":[{"refType":"CHILD_OF"}],"startTime","duration","tags","logs","processID"}],"processes":{"p1":{"serviceName","tags"}}}]}`. CONFIRMED in part: jaeger-ui `transform-trace-data.ts` reads `span.processID` and `data.processes[processID]`, and handles `startTime`; `refType` exists in `jaeger-idl` `model/v1/spanref.go`. Times are microseconds (HYPOTHESIS, from my knowledge of the format; not re-read). Jaeger UI repo: https://github.com/jaegertracing/jaeger-ui.
- **Zipkin v2 JSON**: array of spans with `traceId` (16 or 32 lowercase hex), `id`, `parentId`, `kind`, `localEndpoint`, `timestamp` (µs), `duration`, `tags` as string map. CONFIRMED for `traceId`/`id` constraints from https://github.com/openzipkin/zipkin-api/blob/master/zipkin2-api.yaml. Zipkin repo Apache-2.0, 17,474 stars.
- **otel-desktop-viewer** (https://github.com/CtrlSpice/otel-desktop-viewer, Apache-2.0, 1,301 stars, active): local OTLP receiver and UI; docs mention reconstructing "standard OTLP JSON" from its store and importing OTLP JSON (e2e `imports.spec.ts`). Not a validator.
- **otel-tui** (https://github.com/ymtdzzz/otel-tui, Apache-2.0, 1,112 stars): terminal viewer; has a `--from-json-file` flag "exported by JSON exporter". CONFIRMED from README grep. Viewer, not a validator.
- **otel-cli** (https://github.com/equinix-labs/otel-cli, Apache-2.0, 723 stars): creates spans from shell. Not an analyzer.
- **otelbin**: config visualizer for Collector pipelines (otelbin.io). I could not find the repo via the API; not relevant to traces. Not verified.
- **Tracetest** (https://github.com/kubeshop/tracetest, 1,328 stars, last push 2025-06-03, NOASSERTION license): trace-based testing with assertions and a "Semantic Conventions" analyzer plugin (https://docs.tracetest.io/analyzer/plugins/otel-semantic-conventions, SEARCH-SUMMARY). Looks dormant (no push for 16 months). Generic, no MCP awareness found.
- **OTel Semantic Convention Validator** (Apify actor): validates OTLP/JSON traces/metrics/logs against semconv 1.24 to 1.44. Hosted, not an OSS CLI. SEARCH-SUMMARY, https://apify.com/michiganpublictech/otel-semconv-validator.
- GitHub searches for "otel trace lint", "span validator", "otlp json cli analyzer", "trace analyzer cli otlp" returned **no relevant OSS repo**. (Searches are keyword-weak on GitHub; absence is suggestive, not proof.)
- MCP-specific proxy/observer projects exist but are tiny: `td-02/mcp-observer` (4 stars, proxy MCP traffic, inspect traces, replay calls, diff schemas), `pmcfadin/mcp-observability` (5 stars, MCP server over an observability stack). Not OTel-trace linters.

### B.4 Go libraries

CONFIRMED by reading source on 2026-10-30:

- `go.opentelemetry.io/collector/pdata` (module path from `pdata/go.mod`, go 1.26.0). `pdata/ptrace/json.go` defines `type JSONMarshaler struct{}` with `MarshalTraces(td Traces) ([]byte, error)` and `type JSONUnmarshaler struct{...}` with `UnmarshalTraces(buf []byte) (Traces, error)`. This parses one JSON document (one `ExportTraceServiceRequest`), so read the file line by line and call it per line. pdata gives a convenient in-memory API (`ResourceSpans().At(i)`), at the cost of a heavier dependency tree (pdata is a standalone module, so it does not pull the full Collector).
- `go.opentelemetry.io/proto/otlp` (repo `open-telemetry/opentelemetry-proto-go`, Apache-2.0, 46 stars, pushed 2026-10-28; module `go.opentelemetry.io/proto/otlp`, go 1.26.0, subdirs include `collector/trace` and `trace`). Use `collectortrace.ExportTraceServiceRequest` with `google.golang.org/protobuf/encoding/protojson`. HYPOTHESIS (standard usage, not run): `protojson.UnmarshalOptions{DiscardUnknown: true}` handles hex `traceId`? Careful: stock protojson expects base64 for `bytes` fields, while OTLP JSON uses hex. This is a known gotcha. Whether the generated package patches it, I did not check. pdata's unmarshaler handles the hex deviation internally, which is a strong reason to prefer pdata. Verify in a spike before choosing.
- Recommendation (HYPOTHESIS): use pdata `ptrace.JSONUnmarshaler` for OTLP JSON, hand-roll small decoders for Jaeger and Zipkin JSON (plain `encoding/json`).

---

## Part C. Demand evidence

Method: `gh search issues --owner <org>` with queries for traceparent, trace context, opentelemetry, observability, distributed tracing, plus web search for blogs and forums. All issue states as of 2026-10-30.

### C.1 Spec and SEP level (shows the problem was recognised and resolved at spec level)

| Date | State | URL | Summary |
|---|---|---|---|
| 2025-03-27 | closed 2025-12-05 | https://github.com/modelcontextprotocol/modelcontextprotocol/issues/246 | "Include OpenTelemetry Trace identifiers as part of the MCP client -> server protocol". 16 comments. Origin of SEP-414. |
| 2025-04-25 | Final | https://modelcontextprotocol.io/seps/414-request-meta (repo file `docs/seps/414-request-meta.mdx`) | SEP-414 (Adrian Cole): document OTel trace-context propagation conventions via `_meta`. |
| 2026-06-06 | closed 2026-08-23 | https://github.com/modelcontextprotocol/modelcontextprotocol/issues/2868 | Tracking issue: SEP-414 implemented in TS, C#, Go, Java, PHP, Python, Ruby, Rust SDKs; Kotlin (#800) and Swift (#230) unchecked at last read. |
| 2025-04-01 | closed | https://github.com/open-telemetry/semantic-conventions/issues/2043 | "Semantic conventions for Model Context Protocol". |
| 2026-08-05 | open | https://github.com/open-telemetry/semantic-conventions-genai/issues/437 | MCP semconv out of date vs protocol 2026-07-28 (stateless, no session id). Based on a reproducible Go SDK v1.7.0 run. |
| 2026-04-10 | closed | https://github.com/open-telemetry/semantic-conventions-genai/issues/92 | `gen_ai.tool.call.arguments`/`result` missing from MCP server span attribute table but present in examples (shows spec/doc drift a linter could catch). |

### C.2 SDK-level pain (real maintainers hitting real gaps)

| Date | State | URL | Summary |
|---|---|---|---|
| 2025-04-03 | closed | https://github.com/modelcontextprotocol/python-sdk/issues/421 | Adding OpenTelemetry to MCP SDK |
| 2025-05-21 | closed | https://github.com/modelcontextprotocol/python-sdk/issues/766 | How to instrument `resources` and `prompts` |
| 2025-12-04 | open | https://github.com/modelcontextprotocol/typescript-sdk/issues/1231 | Undici instrumentation marks successful MCP requests as aborted (premature `AbortController.abort()` in SSE/StreamableHTTP). 9 comments. Shows instrumentation producing wrong spans. |
| 2025-12-08 | open | https://github.com/modelcontextprotocol/typescript-sdk/issues/1264 | "OpenTelemetry integration out of the box" (part of v2 SDK work). 2 comments. |
| 2026-07-26 | closed | https://github.com/modelcontextprotocol/python-sdk/issues/3174 | [v2] JSON-RPC error responses leave client OTel spans UNSET (a span-status correctness bug). 10 comments. |
| 2026-09-08 | closed | https://github.com/modelcontextprotocol/python-sdk/issues/3473 | Client sends empty `_meta:{}` on every request; strict servers reject with HTTP 400 (a `_meta` interop failure). |
| 2026-10-10 | open | https://github.com/modelcontextprotocol/php-sdk/issues/583 | Handler fibers give integrations no hook to carry OTel context (context lost across async boundary). |
| 2026-10-15 | open | https://github.com/modelcontextprotocol/python-sdk/issues/3715 | Client: expose outbound request seam; code TODO says "move the otel span + inject into an outbound middleware". 6 comments. |
| 2026-05-26 | closed | https://github.com/modelcontextprotocol/php-sdk/issues/336 | [2026-07-28] Transport & observability headers/metadata |
| 2026-10-08 | open | https://github.com/modelcontextprotocol/financial-services-interest-group/issues/16 | Proposal: lineage parent key in `_meta` as a Financial Services extension (adjacent: compliance lineage, not OTel). |

### C.3 Observability vendor and OTel-side items

| Date | State | URL | Summary |
|---|---|---|---|
| 2025-05-13 | closed 2025-05-14 | https://github.com/Arize-ai/openinference/issues/1616 | MCP instrumentation and context propagation is not working. Closed within a day. |
| 2025-04-21 | open | https://github.com/Arize-ai/openinference/issues/1543 | MCP sampling propagation |
| 2025-11-04 | open | https://github.com/open-telemetry/opentelemetry.io/issues/8330 | Blog post: OpenTelemetry MCP Server (agent queries OTel, not MCP observability) |
| 2026-05-26 | closed | https://github.com/open-telemetry/opentelemetry-python-genai/issues/94 | Add MCP invocation types and TelemetryHandler support |
| 2026-09-26 | open | https://github.com/langfuse/langfuse/issues/17969 | Codex Desktop MCP calls omitted from tracing plugin |
| 2026-05-25 | open | https://github.com/MCPJam/inspector/issues/2273 | Export prompt tracing |

### C.4 Blog and forum signals (SEARCH-SUMMARY, not read in full)

- https://dev.to/rupa_tiwari_dd308948d710f/mcp-observability-how-to-trace-every-tool-call-in-production-21cp
- https://dev.to/focused_dot_io/agent-traces-need-to-cross-the-mcp-boundary-focused-labs-5f2l (title states the exact problem: traces stop at the MCP boundary)
- https://last9.io/blog/mcp-server-monitoring/ (failed tool call returns a successful JSON-RPC response with `isError` set, so HTTP/RPC error rates miss it)
- https://mcpplaygroundonline.com/blog/mcp-observability-opentelemetry (one logical operation becomes two unconnected traces; conventions unstable)
- https://signoz.io/blog/mcp-observability-with-otel, https://stacklok.com/blog/bridging-the-observability-gap-in-mcp-servers-with-toolhive/, https://tyk.io/docs/nightly/ai-management/mcp-gateway/mcp-observability

No Reddit or Hacker News threads surfaced. I did not search those sites directly beyond one combined web query, so their absence is weak evidence.

### C.5 Honest assessment of demand

- **Strong**: the problem space is real and recognised. The spec now has a normative rule; nine SDKs implemented it; OTel has MCP semconv; four vendors ship MCP monitoring (Sentry, Datadog, Grafana, Langfuse docs).
- **Moderate**: SDK maintainers repeatedly hit instrumentation correctness bugs (wrong span status #3174, aborted spans #1231, lost context #583, empty `_meta` rejections #3473). These are exactly bugs a trace checker would flag.
- **Thin**: I found **no issue from an end user saying "my MCP traces are disconnected, how do I verify?"** and no request for a trace linter or conformance check for telemetry in any repo I searched, including the conformance repo (0 hits) and weaver (0 MCP-specific issues). Most "MCP" hits in Tempo, Jaeger, Phoenix and Langfuse trackers concern those tools' own MCP servers.
- Practical reading: demand for "MCP instrumentation exists and should work" is clear; demand for a separate verification tool is unproven and is HYPOTHESIS. The vendor blogs complaining about broken propagation are the best proxy, and they are marketing content.
- The newest spec (2026-07-28) removed sessions and moved to stateless per-request `_meta`, which makes `_meta`-based propagation the only propagation path over stdio. That raises the importance of correct `_meta` handling, but the evidence is inference.

---

## Appendix: key sources (all verified 2026-10-30)

- Conformance: https://github.com/modelcontextprotocol/conformance (README, `src/scenarios/**`, issues)
- MCP spec basic page, `_meta` trace context: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx
- OTel MCP semconv: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md and `model/mcp/`
- Weaver: https://github.com/open-telemetry/weaver (`crates/weaver_live_check/README.md`, `docs/usage.md`)
- fileexporter and receiver: https://github.com/open-telemetry/opentelemetry-collector-contrib
- OTLP JSON mapping: https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md
- pdata JSON: https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/ptrace/json.go
