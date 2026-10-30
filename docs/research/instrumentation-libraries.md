# MCP instrumentation libraries: landscape and propagation compatibility

Verification date for every claim in this file: 2026-10-30 (today), unless a different date is given. "CONFIRMED" means I cloned the repository and read the cited source file (or read the cited issue/spec text). "HYPOTHESIS" means inferred, or not checked in code. Stars, open-issue counts and push dates come from `gh api repos/OWNER/REPO` on 2026-10-30. Cloned HEAD commit dates come from `git log -1`.

## 1. Short answer to the critical question

Do libraries use different, incompatible propagation mechanisms? Mostly no, with one real exception and two real failure modes.

- CONFIRMED: the spec now settles it. MCP spec revision 2026-07-28 reserves the unprefixed `_meta` keys `traceparent`, `tracestate`, `baggage` for W3C trace context (https://raw.githubusercontent.com/modelcontextprotocol/modelcontextprotocol/main/docs/specification/2026-07-28/basic/index.mdx, lines 358 and 419-446). The OTel MCP semantic conventions say instrumentations SHOULD inject via the configured propagator into `params._meta` and the receiver extracts it as remote parent (https://raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/main/docs/gen-ai/mcp.md, "Context propagation", lines 43-66). Both read 2026-10-30.
- CONFIRMED: every serious library reads/writes `_meta.traceparent` (plus `tracestate`/`baggage` where a real propagator is used): mcp-otel-go, opentel-mcp, openllmetry, openinference (Python and JS), AmirK-S/mcp-opentelemetry, and the official Python SDK's built-in OTel middleware. Evidence in section 3.
- CONFIRMED outlier 1 (incompatible): `dylibso/mcp-otel` writes a custom object `_meta.__traceContext = {traceId, spanId, traceFlags, isRemote}`. It does not interoperate with the others. It is a 15-star demo last pushed 2026-02-14, so low practical risk.
- CONFIRMED outlier 2 (no propagation at all): AgentCat derives a trace id by hashing the session id (`sha256(sessionId)[0:32]`) in its own exporter. It never reads `_meta.traceparent`. Its traces cannot join a caller's trace.
- CONFIRMED failure mode A (same field, one-sided): the Go and `opentel-mcp` libraries only extract (server side). They do not inject on the client. openinference Python/JS and AmirK-S do both. So "same wire format" does not mean "end to end works": someone must inject on the client.
- CONFIRMED failure mode B (same field, silently broken): openllmetry's client injection is gated on `params.meta` already being non-None, so for an ordinary `call_tool()` nothing is injected (open bug traceloop/openllmetry#4374, plus my own read of the code, section 2.4).
- Residual incompatibility is in span shape, not propagation: span names, attribute sets and error attributes differ widely (section 4). Only mcp-otel-go, opentel-mcp, AmirK-S and the official Python SDK follow the OTel `mcp.method.name` / `{method} {target}` convention. openllmetry and openinference use Traceloop/OpenInference conventions, not the OTel MCP one.

## 2. Per-library findings

### 2.1 olgasafonova/mcp-otel-go

- Repo: https://github.com/olgasafonova/mcp-otel-go. 5 stars, 0 open issues (and none ever filed: `gh issue list --state all` returned nothing), MIT, last push 2026-10-11, cloned HEAD commit 2026-10-11. Maintainer: single author; commit log is mostly dependabot (CONFIRMED by `git log --format=%an`).
- Problem solved: OTel tracing and the `mcp.server.operation.duration` metric for Go MCP servers using `modelcontextprotocol/go-sdk` v1.8.0 (`go.mod`). Server side only.
- Language/SDK: Go, official go-sdk. Go 1.26 (`go.mod`).
- Shape (CONFIRMED, `mcpotel/middleware.go` lines 165-205): a go-sdk method middleware (`func(next MethodHandler) MethodHandler`). Per request it builds attributes, calls `r.propagator.Extract(ctx, carrierFor(req))` BEFORE `tracer.Start`, starts a `SpanKindServer` span named `spanName(method, target)`, calls `next`, then sets status and records duration.
- Propagation (CONFIRMED, `mcpotel/propagation.go`): `metaCarrier map[string]any` wraps `params.GetMeta()` as an OTel `TextMapCarrier` (Get/Set/Keys, string values only). It reads whatever keys the configured propagator uses, so `_meta.traceparent`/`tracestate`/`baggage` with the W3C propagator. `middleware.go` lines 60-71 say it defaults to `otel.GetTextMapPropagator()` and warns that the OTel global default is a no-op propagator. Only `Extract` is used anywhere in the middleware; there is no client-side `Inject` (grep for Inject returned nothing). HYPOTHESIS: this is deliberate (server-only scope).
- Transports: the middleware sits at the MCP method level, so it is transport-agnostic (HYPOTHESIS; I did not find transport-specific code or tests). It does not read HTTP headers.
- Span names (CONFIRMED, `spans.go`): `{method}` or `{method} {target}`; target is tool name for `tools/call`, redacted URI for `resources/read`, prompt name for `prompts/get`.
- Attributes (CONFIRMED, `spans.go` constants): `mcp.method.name`, `mcp.session.id`, `mcp.resource.uri` (default redacted to scheme only), `gen_ai.tool.name`, `gen_ai.prompt.name`, `error.type`. Metric: `mcp.server.operation.duration` (`metrics.go`). Default error text is the Go type name (privacy-first redaction).
- Errors (CONFIRMED, `middleware.go` `callErrMsg`, `spans.go` `extractToolError`): handles BOTH surfaces. A Go error from the handler chain (JSON-RPC error path) sets `codes.Error`; a `CallToolResult` with `IsError` also sets `codes.Error` with message `tool_error` or the redacted underlying error. Success sets `codes.Ok`.
- Operations: all methods get spans, with target extraction for tools/call, resources/read, prompts/get. A hard-coded skip list (`middleware.go` lines 275-278) excludes `notifications/initialized`, `notifications/cancelled`, `notifications/progress`, `notifications/roots/list_changed`. So notifications are deliberately NOT traced.
- Tests: yes, `mcpotel/*_test.go` (~1,700 lines including `propagation_test.go`, redaction, typed-nil, metric-failure, bench), CI with codecov (`.github/workflows/ci.yml`).
- Semconv: follows the OTel MCP semconv by its own README claim, and the constants match the spec names (CONFIRMED against the semconv doc, section 4).
- Contributing upstream: viable but low leverage. Single maintainer, MIT, tiny user base (5 stars), responsive code quality. A client-side `Inject` and notification coverage would be natural contributions. Risk: no issue tracker history to judge responsiveness.

### 2.2 opentel-mcp ("the real repo")

- There are several repos with similar names. The one literally named `opentel-mcp` with OpenTelemetry instrumentation for MCP servers is https://github.com/Thirumalaiboobathi/opentel-mcp (6 stars, 0 issues listed open besides closed ones, last push 2026-10-26, created 2026-07-09). Not the same as `traceloop/opentelemetry-mcp-server` (204 stars; that is an MCP server that queries traces, unrelated) or `theharithsa/opentelemetry-instrumentation-mcp` (5 stars, see 2.7).
- Problem: instrument MCP servers with OTel spans and metrics, plus "silent failure" detection (tool returns `isError:true`), failure fingerprinting, cost attribution, schema-drift detection. Packages: `packages/core` (npm `opentel-mcp` v0.17.0, JS), `packages/python`, `packages/ui`. License MIT (`packages/core/package.json` line 47; GitHub shows no SPDX because the root has no recognized LICENSE file. HYPOTHESIS that it is just missing).
- Maintainer: effectively one person (41 + 9 commits by two aliases of one account, `git log`). Closed issues were self-filed. Heavy AI-assisted-looking docs (HANDOFF.md, RUNLOG.md, 26+ ADRs).
- SDK: official TypeScript SDK (v1 and v2 adapters `kind: 'v1' | 'v2'`, `instrument.js` lines 286-309) and a Python package.
- Propagation (CONFIRMED, `packages/core/src/tracecontext/extract.js`): reads `meta.traceparent` (string) and optional `meta.tracestate` from `params._meta`, hand-parses with a W3C regex, builds a remote `SpanContext`, and uses it as parent. Extraction only; ADR 017 line 4 says a client-side shim is out of scope. Applies to `tools/call` (docstring: "from an MCP `tools/call` request's `params._meta`"). The extract.js docstring says it does not use `@opentelemetry/core` propagators. `tools/list` has no parent (ADR 017 line 251).
- Spans (CONFIRMED, `instrument.js`): `SpanKind.SERVER` spans named `{mcp.method.name} {target}` (e.g. `prompts/get {name}` line 863) and the bare method for `tools/list` (line 2285). Resources/prompts spans added by ADR 026 (line 838).
- Attributes (CONFIRMED, `attributes.js`): `mcp.method.name`, `gen_ai.tool.name`, `gen_ai.prompt.name`, `gen_ai.operation.name`, `jsonrpc.request.id`, `error.type`, `mcp.server.name`, `mcp.server.version`, `mcp.tool.argument_count`, `mcp.tool.outcome`, `mcp.tool.tokens.{input,output,total}`, `mcp.tool.model`, `gen_ai.response.model`, `mcp.tool.cost.usd`, `mcp.tool.cost.currency`, `mcp.tool.cost.budget_exceeded`, `mcp.tool.cost.budget_scope`, `mcp.tool.pricing_status`. The `mcp.tool.*` family is a custom vendor namespace, not OTel semconv.
- Errors (CONFIRMED, `instrument.js` lines 1075-1086 and 866-892): both. Thrown errors set span status ERROR plus `error.type` and a failure fingerprint with a "channel" classification (protocol error vs tool error); `result.isError === true` is treated as the "silent failure" case.
- Operations: tools/call (most complete), tools/list, resources, prompts (ADR 026). Notifications: HYPOTHESIS no (not seen).
- Transports: wraps server objects, so transport-agnostic (HYPOTHESIS); default span output goes to stderr because stdio servers own stdout (`instrument.js` line 966).
- Tests: yes, extensive (`packages/core/test/`, includes `tracecontext/extract.test.js`, `integration/trace-context-propagation.test.js`, a `conformance/` suite shared with Python).
- Contributing upstream: low. Single-maintainer, very high churn, own design language. Better as a source of ideas (silent-failure classification, `isError` vs protocol-error channel split) than a dependency.

### 2.3 AgentCat SDKs

- Real thing (CONFIRMED): AgentCat is a product analytics platform for MCP server owners, formerly MCPcat. SDKs: https://github.com/agentcathq/agentcat-typescript-sdk (127 stars, pushed 2026-10-30, MIT, 5 open issues), https://github.com/agentcathq/agentcat-python-sdk (53 stars, 2026-10-24, MIT), https://github.com/agentcathq/agentcat-go-sdk (10 stars, 2026-10-24, MIT). `agentcathq/mcpcat-go-sdk` is archived/deprecated, superseded by the Go SDK. npm package version 2.1.1 (`package.json`). Not to be confused with `LRainner/AgentCat` (Rust desktop pet) or `yong076/agentcat-connectors` (local activity collector).
- Problem: product analytics (sessions, user intent, actors, tool usage) for MCP servers, shipped to the AgentCat SaaS and optionally to OTLP, Datadog, Sentry, PostHog exporters (`src/index.ts` lines 53-57).
- Propagation (CONFIRMED, `src/modules/exporters/trace-context.ts`): none from the wire. `getTraceId(sessionId)` = `sha256(sessionId)` first 32 hex chars; `getSpanId(eventId)` = hash of event id first 16 hex chars. No `traceparent` read anywhere in `src` (grep). `_meta` is read only for `io.modelcontextprotocol/clientInfo` and protocol version (`src/modules/session.ts`). The Python SDK exporters do the same (`datadog.py` line 168, `sentry.py` lines 192, 331).
- Spans (CONFIRMED, `exporters/otlp.ts` lines 91-140): synthetic OTLP spans built from events. `name = event.eventType || "mcp.event"`, `kind = 2` (SERVER), attributes `mcp.event_type`, `mcp.session_id`, `mcp.project_id`, `mcp.resource_name`, `mcp.user_intent`, `mcp.actor_id`, `mcp.actor_name`, `mcp.client_name`, `mcp.client_version`. These are not OTel semconv names (`mcp.session_id` with underscore vs semconv `mcp.session.id`).
- Errors: HYPOTHESIS not read in detail; event model carries error info but I did not trace it.
- Transports: wraps the server object (v1 and v2 adapters), so transport-agnostic (HYPOTHESIS); has a Cloudflare workerd entry (`src/index.workerd.ts`).
- Tests: yes (91 test-related files in the TS repo) and an `mcp-compatibility.yml` workflow.
- Contributing upstream: not useful for propagation. It is a commercial analytics SDK with a different data model. Treat as a competitor/adjacent product, not a library to extend. Good evidence that a "session-hash trace id" approach exists and does not interoperate.

### 2.4 traceloop/openllmetry MCP instrumentation

- Repo: https://github.com/traceloop/openllmetry, package `packages/opentelemetry-instrumentation-mcp` v0.62.4 (`pyproject.toml`), Apache-2.0. 7,513 stars, 820 open issues repo-wide, repo pushed 2026-10-19; last commit touching the package 2026-10-19. Package contributors: four people (Dvir Rezenman, IdoGol24, SwatiPoojary, a bot). Active.
- Problem: auto-instrument the Python MCP SDK (`mcp >= 1.6.0`, `_instruments` line 32) so MCP calls appear as Traceloop-style spans, with FastMCP support (`fastmcp_instrumentation.py`).
- Transports (CONFIRMED, `instrumentation.py` lines 100-170): wraps `mcp.client.sse.sse_client`, `mcp.server.sse`, `mcp.client.stdio.stdio_client`, `mcp.server.stdio.stdio_server`, `mcp.server.session`, `mcp.client.streamable_http`, `mcp.server.streamable_http`, and `mcp.shared.session` (send_request).
- Propagation (CONFIRMED):
  - Client, `patch_mcp_client` lines 249-254: `TraceContextTextMapPropagator().inject(carrier); meta.traceparent = carrier["traceparent"]`. Only `traceparent` is written, `tracestate` is dropped, and this is inside `if meta and len(args) > 0`.
  - Because `RequestParams.meta` defaults to `None`, the injection is skipped for ordinary tool calls. This is open issue https://github.com/traceloop/openllmetry/issues/4374 ("MCP client instrumentation never propagates trace context for ordinary tool calls (regression from #3179)", open, updated 2026-07-26). The issue text matches the code I read.
  - Second injection path, `InstrumentedStreamWriter.send` lines 705-712: `request.params.setdefault("_meta", {})` then `propagate.get_global_textmap().inject(meta)` for outgoing JSON-RPC requests, so context may still flow via that path when the instrumented stream writer is active. HYPOTHESIS on whether this fully compensates for #4374; I did not run it.
  - Server, lines 654-656: `propagate.extract(request.params.get("_meta"))` then `context.attach`.
- Span names (CONFIRMED, lines 293-295, 386-434, 699-701): `mcp.client.session`, `{tool_name}.tool` (or `{method}.tool`), `{method}.mcp`, `ResponseStreamWriter`; FastMCP path also `{entity_name}.tool`. Span kinds: default (internal), not CLIENT/SERVER.
- Attributes (CONFIRMED, `semconv_ai/__init__.py` lines 117-123 and 311-314; `instrumentation.py`): `traceloop.span.kind`, `traceloop.entity.name`, `traceloop.entity.input`, `traceloop.entity.output`, `mcp.method.name`, `mcp.request.argument`, `mcp.request.id`, `mcp.session.init_options`, `error.type`. Content capture is gated (`TRACELOOP_TRACE_CONTENT`; `tests/test_content_capture_gate.py`).
- Errors (CONFIRMED, lines 474-475, 722-725): `isError` on the result sets `error.type = "tool_error"` and span status error. JSON-RPC errors are handled via exceptions (HYPOTHESIS, not traced in detail). Open bug #4569: crashes on error results with non-text content (the response writer reads `result['content'][0]['text']`, line 725).
- Operations: tools (richest), generic requests via `{method}.mcp`. Resources/prompts only as generic `{method}.mcp` spans. Notifications: HYPOTHESIS not specifically handled.
- Tests: yes, six test files under `tests/`.
- Open MCP issues (CONFIRMED via `gh issue list --search mcp`): #4653 uninstrument leaks and duplicate spans, #4612 breaks fastmcp-slim imports, #4569 non-text error results, #4413 unsupported `mcp==2.x` (`streamablehttp_client`), #4463 `@dont_throw` makes a tracing error return None and break the tool call, #4374 propagation regression, #4188 content gate. This is a sign the package is under active but strained maintenance and lags `mcp` 2.x.
- Contributing upstream: viable (active maintainers, Apache-2.0) and the highest-impact target for a Python fix: fix #4374 and add `tracestate`/`baggage`, and migrate span names/attributes to the OTel MCP convention. Caveat: the project uses its own `traceloop.*` conventions and a 820-issue backlog.

### 2.5 Arize openinference MCP instrumentation

- Repo: https://github.com/Arize-ai/openinference. 1,296 stars, 295 open issues, repo pushed 2026-10-30, Apache-2.0. MCP packages: Python `python/instrumentation/openinference-instrumentation-mcp` (last commit 2026-10-30), JS `js/packages/openinference-instrumentation-mcp` v0.2.37 (last commit 2026-10-23). Main authors: Mikyo King and a few others (`git log`).
- What it does (CONFIRMED): context propagation only. Its README says "Currently, it only enables context propagation ... It does not generate any telemetry." I confirmed zero `start_span`/`start_as_current_span` calls in both `__init__.py` and `mcp.ts`. Spans come from other OpenInference instrumentors or from the application.
- Propagation: yes, `_meta`. This matches the belief stated in the task. CONFIRMED details:
  - Python, `.../instrumentation/mcp/__init__.py` lines 165-168 (server): `meta = request.params.get("_meta"); if meta: ctx = propagate.extract(meta); restore = context.attach(ctx)`. Lines 201-205 (client): `meta = request.params.setdefault("_meta", {}); propagate.get_global_textmap().inject(meta)`.
  - JS, `src/mcp.ts` lines 303-306 (inject): `if (!message.params._meta) { message.params._meta = {}; } propagation.inject(context.active(), message.params._meta);` and line 326 (extract): `propagation.extract(context.active(), message.params?._meta)`.
  - Uses the globally configured propagator, so `traceparent`, `tracestate`, `baggage` all flow if the W3C propagators are registered. Unlike openllmetry, it creates `_meta` when absent.
- The Python code structure (ObjectProxy stream wrappers, `ContextSavingStreamWriter`) is the same family as openllmetry's, and openinference's was evidently derived from it (HYPOTHESIS from code similarity; both use identical class patterns).
- Transports: wraps the `mcp` Python SDK transports and the JS SDK's `Client`/`Server`/transport modules (`InstrumentationNodeModuleDefinition` x6 in `mcp.ts`). Exact module list for Python not enumerated here.
- Span names/attributes: none from this package. Error representation: none (no spans).
- Tests: yes, Python `tests/test_instrumenter.py` plus fixtures; JS `test/mcp.test.ts`.
- Open issues relevant to tracing: #1477 (MCP python instrumentation proposal), #1478 (context span kind), #1543 ("MCP sampling propagation", updated 2026-10-25), #2123 (auto-instrumentation does not trace `tool.task` calls), #1666 (hard to represent MCP get-prompt with existing semconv). All open.
- Also relevant: `open-telemetry/opentelemetry-js-contrib` issue #3668 "Migrate OpenInference JS Instrumentations into opentelemetry-js-contrib" (open, updated 2026-10-30) means the JS MCP propagation package may move upstream into OTel contrib. HYPOTHESIS that the MCP package is in scope for that migration; I did not read the issue body.
- Contributing upstream: viable and the lowest-risk for propagation behavior. Active, Apache-2.0, small package. Its design (propagation only) is the cleanest proof that "just `_meta` inject/extract" is the shared contract.

### 2.6 open-telemetry contrib repos (go-contrib, python-contrib, js-contrib)

- CONFIRMED: no MCP instrumentation package exists in any of the three as of 2026-10-30. `gh search code "mcp"` returned only unrelated hits (host-metrics, openai-agents README, OpenAI test mocks in js-contrib; `instrumentation/host/go.mod` in go-contrib).
- go-contrib: issue https://github.com/open-telemetry/opentelemetry-go-contrib/issues/7616 "Proposal: Support for OTel Implementation in go-mcp" (open, last update 2025-09-12, 3 comments). No code.
- python-contrib: https://github.com/open-telemetry/opentelemetry-python-contrib/issues/4632 "opentelemetry-util-genai: add MCP invocation types and TelemetryHandler support" is CLOSED as completed 2026-06-16. HYPOTHESIS: this is util code for GenAI MCP handling, not a full MCP instrumentation. Also #4197 (OpenAI Agents MCP List Tools spans) closed 2026-10-22.
- js-contrib: only tracking issue #3668 (OpenInference JS migration), see 2.5.
- Spec side (CONFIRMED): the semconv moved from `open-telemetry/semantic-conventions` (deprecated MCP model under `model/mcp/deprecated/`) to `open-telemetry/semantic-conventions-genai` (`docs/gen-ai/mcp.md`, `model/mcp/spans.yaml`). Open issues there: #437 (align MCP semconv with protocol 2026-07-28 and expose peer server metadata), #509 (model MCP tool call spans as `execute_tool` refinement), #66 (how to record params and result), #589 (lineage reference on tool-call span), #95 (tool approval). All open, updated Sep-Oct 2026.
- Contributing upstream: the OTel Go/JS contrib repos are the natural home for a vendor-neutral Go MCP instrumentation. The go-contrib proposal has been idle for a year; a PR there would need a sponsor/maintainer and would likely be asked to wait for semconv stabilization (still "Development" status).

### 2.7 Other relevant repos found while searching

- AmirK-S/mcp-opentelemetry (https://github.com/AmirK-S/mcp-opentelemetry): OTel instrumentation for the MCP TypeScript SDK v2. 0 stars, 1 open issue, MIT, created 2026-09-05, last push 2026-09-06, single author (19 commits). CONFIRMED the most complete propagation implementation of the small libraries: `src/instrumentation.ts` lines 471-490 start a `SpanKind.CLIENT` span and `propagator.inject(ctx, ownMeta(message), metaSetter)` into `params._meta` for outgoing requests and notifications; lines 533-576 `propagator.extract(ROOT_CONTEXT, meta, metaGetter)` and start a `SpanKind.SERVER` span (with span links and baggage handling) for incoming ones; keys in `src/keys.ts` are `traceparent`, `tracestate`, `baggage`. Attributes in `src/semconv.ts`: `mcp.method.name`, `mcp.protocol.version`, `mcp.resource.uri`, `gen_ai.tool.name`, `gen_ai.operation.name`, `gen_ai.prompt.name`, `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`, `jsonrpc.request.id`, `jsonrpc.protocol.version`, `rpc.response.status_code`, `error.type`, `network.transport`; metrics `mcp.{client,server}.operation.duration` and `mcp.{client,server}.session.duration`. Its own `docs/END-OF-LIFE.md` (dated 2026-09-05) says it may be retired if the TS SDK ships instrumentation; it also records that the Python SDK ships OTel middleware by default. Tests exist (`test/requests.test.ts`, `notifications.test.ts`, `keys.test.ts` etc., including a check of keys against the SDK). Not a good upstream target (one-month-old solo project), but a good reference implementation.
- dylibso/mcp-otel (https://github.com/dylibso/mcp-otel): 15 stars, BSD-3-Clause, last push 2026-02-14. CONFIRMED custom mechanism: `_meta.__traceContext = {traceId, spanId, traceFlags, isRemote}` (README lines 44-72; `src/agent.ts` line 122-125). Its README says it "creatively reuses" `_meta`. This is the non-standard outlier.
- theharithsa/opentelemetry-instrumentation-mcp: 5 stars, last push 2026-06-18, ISC (`package.json`). CONFIRMED tiny: `src/index.ts` line 31 only does `tracer.startSpan("mcp.tool:" + name)` and has no propagation code (my grep for `_meta`/`traceparent`/`inject`/`extract` found nothing). A different, non-semconv span name.
- threadwire/mcp-telemetry (Python, 0 stars, pushed 2026-08-30, MIT): ADR-0003 says it continues a W3C `traceparent` received as an HTTP header (`examples/responder.py` line 81-84: `self.headers.get("traceparent")`). This is the HTTP-header-only variety. CONFIRMED from the ADR and example file; this is a hub-to-responder relay, not an MCP-SDK instrumentation, and does not use `_meta`.
- Official SDK built-in tracing (CONFIRMED by reading source): `modelcontextprotocol/python-sdk` (24,715 stars) has `src/mcp/shared/_otel.py` (`inject_trace_context(meta)`, `extract_trace_context`, tracer `mcp-python-sdk`) and `src/mcp/server/_otel.py` (server middleware, `SpanKind.SERVER`, `context=extract_trace_context(ctx.meta)`, attributes `jsonrpc.request.id`, `gen_ai.operation.name=execute_tool`, `gen_ai.tool.name`, `gen_ai.prompt.name`, `error.type`, `rpc.response.status_code`; tool `isError` sets `error.type="tool_error"` and ERROR status), plus docs `docs/run/opentelemetry.md` and tests `tests/server/test_otel.py`, `tests/shared/test_otel.py`. Latest releases v2.3.0 (2026-10-02), v2.2.0 (2026-09-07). The claim in AmirK-S's doc that this became default middleware in `mcp` 2.1.1 is HYPOTHESIS (I did not verify the version).
- `modelcontextprotocol/typescript-sdk`: open issues #1264 "OpenTelemetry integration out of the box" (updated 2026-10-18) and #1238 "Middleware support for McpServer" (2026-10-15). No built-in tracing yet (CONFIRMED by issue state).
- `modelcontextprotocol/go-sdk` (5,291 stars): no OTel/tracing files in the tree (CONFIRMED by tree listing for otel/telemetry/trac). The SEP-414 documentation issue #934 is closed (2026-05-05). The go-sdk middleware point is what mcp-otel-go uses.

### 2.8 Langfuse

- CONFIRMED: Langfuse has no dedicated MCP tracing instrumentation. It is an OTLP-accepting backend (repo https://github.com/langfuse/langfuse, 36,766 stars, 1,108 open issues, pushed 2026-10-30; license reported as NOASSERTION because of mixed MIT/enterprise directories).
- Langfuse's MCP features are (a) its own MCP server that lets agents query Langfuse (`web/src/features/mcp/*`, `web/src/pages/api/public/mcp/index.ts`) and a docs MCP (`langfuse-docs/content/docs/docs-mcp.mdx`), and (b) an integration page for the `mcp-use` agent framework (`content/integrations/frameworks/mcp-use.mdx`). mcp-use has 10,830 stars; I read only the page header, not the integration mechanics (HYPOTHESIS: it relies on an LLM-framework callback/OTel path rather than MCP wire propagation).
- Third-party: `avivsinai/langfuse-mcp` (112 stars, pushed 2026-10-29) is an MCP server for querying Langfuse traces; unrelated to instrumentation.
- Implication: Langfuse (like Phoenix, Jaeger, Tempo) will consume whatever the instrumentations emit. Contributing upstream there is not relevant for propagation.

### 2.9 Top repos from GitHub searches (stars sorted)

Queries used on 2026-10-30: "mcp opentelemetry", "mcp otel middleware" (0 results), "mcp tracing go" (0 results), "mcp-trace", "mcp telemetry", "opentel-mcp", "agentcat", plus description/name searches. Search results are dominated by unrelated repos; real MCP-instrumentation libraries are all tiny. Instrumentation-relevant list, sorted by stars:

| Repo | Stars | Last push | Language | One-line |
|---|---|---|---|---|
| traceloop/openllmetry (mcp package) | 7,513 | 2026-10-19 | Python | OpenLLMetry; includes `opentelemetry-instrumentation-mcp` |
| Arize-ai/openinference (mcp packages) | 1,296 | 2026-10-30 | Python/TS | Propagation-only MCP instrumentation, Py+JS |
| agentcathq/agentcat-typescript-sdk | 127 | 2026-10-30 | TypeScript | MCP product analytics, optional OTLP/Datadog/Sentry exporters |
| agentcathq/agentcat-python-sdk | 53 | 2026-10-24 | Python | same, Python |
| shinzo-labs/shinzo-ts | 68 | 2025-12-08 | TypeScript | Shinzo Labs telemetry SDK (mentioned in mcp-otel-go README as TS-only; not read in detail) |
| dylibso/mcp-otel | 15 | 2026-02-14 | TypeScript | Demo of distributed tracing across MCP via `_meta.__traceContext` |
| agentcathq/agentcat-go-sdk | 10 | 2026-10-24 | Go | MCP analytics, Go |
| Thirumalaiboobathi/opentel-mcp | 6 | 2026-10-26 | JavaScript | OTel + silent-failure detection for MCP servers |
| olgasafonova/mcp-otel-go | 5 | 2026-10-11 | Go | OTel middleware for go-sdk (server side) |
| theharithsa/opentelemetry-instrumentation-mcp | 5 | 2026-06-18 | TypeScript | Minimal tool-call spans, no propagation |
| AmirK-S/mcp-opentelemetry | 0 | 2026-09-06 | TypeScript | OTel for TS SDK v2 with full `_meta` inject+extract |
| threadwire/mcp-telemetry | 0 | 2026-08-30 | Python | Zero-config MCP telemetry, HTTP `traceparent` continuation |

Excluded as unrelated: SigNoz (backend), `traceloop/opentelemetry-mcp-server` and `pavolloffay/opentelemetry-mcp-server` (MCP servers that query traces), `mottibec/otelcol-mcp` (52 stars, OTel collector MCP server), `liatrio-labs/otel-instrumentation-mcp` (an MCP server serving OTel docs). `shinzo-labs/otel-mcp` returned 404, so I did not read it.

## 3. Propagation evidence (file paths and snippets)

All CONFIRMED by cloning on 2026-10-30.

| Library | File and lines | Snippet |
|---|---|---|
| mcp-otel-go | `mcpotel/middleware.go` 179; `mcpotel/propagation.go` | `ctx = r.propagator.Extract(ctx, carrierFor(req))` ; `type metaCarrier map[string]any` over `params.GetMeta()` |
| opentel-mcp | `packages/core/src/tracecontext/extract.js` | `if (typeof meta.traceparent !== 'string') return baseContext;` ... `spanContext.traceState = createTraceState(meta.tracestate)` |
| openllmetry client | `.../mcp/instrumentation.py` 249-254 | `TraceContextTextMapPropagator().inject(carrier)` / `meta.traceparent = carrier["traceparent"]` inside `if meta and len(args) > 0:` |
| openllmetry writer | same file 711-712 | `meta = request.params.setdefault("_meta", {})` / `propagate.get_global_textmap().inject(meta)` |
| openllmetry server | same file 654-656 | `meta = request.params.get("_meta")` / `ctx = propagate.extract(meta)` |
| openinference Py | `.../openinference/instrumentation/mcp/__init__.py` 165-168, 204-205 | `ctx = propagate.extract(meta)` ; `meta = request.params.setdefault("_meta", {})` / `propagate.get_global_textmap().inject(meta)` |
| openinference JS | `js/packages/openinference-instrumentation-mcp/src/mcp.ts` 303-306, 326 | `propagation.inject(context.active(), message.params._meta);` ; `propagation.extract(context.active(), message.params?._meta)` |
| AmirK-S | `src/instrumentation.ts` 474, 534; `src/keys.ts` | `state.propagator.inject(ctx, ownMeta(message), metaSetter)` ; `TRACEPARENT_META_KEY = 'traceparent'` |
| python-sdk | `src/mcp/shared/_otel.py` | `from opentelemetry.propagate import extract, inject`; `inject_trace_context(meta)` |
| dylibso/mcp-otel (outlier) | `README.md` 52-62, `src/agent.ts` 122-125 | `_meta: { __traceContext: { traceId, spanId, traceFlags, isRemote: true } }` |
| AgentCat (outlier) | `src/modules/exporters/trace-context.ts` | `createHash("sha256").update(sessionId).digest("hex").substring(0, 32)` |
| threadwire/mcp-telemetry (HTTP) | `examples/responder.py` 81-84 | `header = self.headers.get("traceparent")` |

Compatibility reading: libraries 1-8 in this table exchange W3C context through the same `_meta` keys, so a client instrumented with openinference or the AmirK-S library will link to a server instrumented with mcp-otel-go, opentel-mcp or the Python SDK. Not interoperable: dylibso (custom object under a `__traceContext` key), AgentCat (no wire propagation), threadwire (HTTP header only, which only helps for streamable-HTTP-style hops and not stdio).

Practical gaps that break "same field" interop:
1. Missing `tracestate`/`baggage`: openllmetry client writes only `traceparent`; opentel-mcp reads `traceparent`+`tracestate` but not `baggage` (extract.js).
2. No-op default propagator: mcp-otel-go documents that the OTel global propagator defaults to no-op, so servers that never call `otel.SetTextMapPropagator` extract nothing. The same applies to any library that relies on the global propagator (openinference, openllmetry server path). HYPOTHESIS for the Python/JS ones (standard OTel behavior, not tested here); CONFIRMED comment in mcp-otel-go.
3. Client injection skipped (openllmetry #4374) or never present (mcp-otel-go, opentel-mcp).
4. Notifications: only AmirK-S injects/extracts for notifications (`instrumentation.ts` lines 462-490 and `test/notifications.test.ts`); mcp-otel-go explicitly skips four notification methods. The OTel semconv says context should be injected "when creating a request or notification".

## 4. Semantic conventions follow-through

Spec (CONFIRMED, https://raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/main/docs/gen-ai/mcp.md, read 2026-10-30): span name `{mcp.method.name} {target}` or `{mcp.method.name}` (lines 143-146); required `mcp.method.name`; `error.type` set when the operation fails, with the JSON-RPC code as string, and `tool_error` when a tool result has `isError: true` (lines 180-196); `jsonrpc.request.id`; `rpc.response.status_code` for the JSON-RPC code; status of ERROR should carry `JSONRPCError.message`. Status of the convention: Development.

- mcp-otel-go: names and attributes align. Gaps vs spec: no `jsonrpc.request.id`, no `rpc.response.status_code`, no `mcp.protocol.version` (HYPOTHESIS that these are absent; not in `spans.go` constants).
- opentel-mcp: aligned on name format and core attributes, plus a large custom `mcp.tool.*` extension.
- AmirK-S: aligned and the closest to the full attribute list.
- Python SDK middleware: aligned on attributes; sets both `error.type` and `rpc.response.status_code`.
- openllmetry: NOT aligned on span names (`{x}.tool`, `{x}.mcp`) or kinds; shares only `mcp.method.name` and `error.type`; adds `traceloop.*` and non-spec `mcp.request.*`.
- openinference: n/a (no spans).
- AgentCat: not aligned (`mcp.session_id`, `mcp.event_type`, custom names).

## 5. Comparison table

| Library | Language | Propagation mechanism | Field used | Semconv followed | Transports | Tests | License | Active? |
|---|---|---|---|---|---|---|---|---|
| olgasafonova/mcp-otel-go | Go (go-sdk) | OTel propagator Extract only (server) | `_meta.traceparent` / `tracestate` / `baggage` (via propagator) | OTel MCP semconv (mostly) | Method-level, transport-agnostic (hyp.) | Yes (~1.7k lines, CI) | MIT | Yes (2026-10-11), 5 stars, solo |
| Thirumalaiboobathi/opentel-mcp | JS (TS SDK v1/v2) + Python | Hand-parsed W3C extract only (server, tools/call) | `_meta.traceparent`, `_meta.tracestate` | OTel MCP semconv + custom `mcp.tool.*` | Wraps server object (hyp. agnostic); stderr span output | Yes (extensive + conformance) | MIT (no root LICENSE detected) | Yes (2026-10-26), 6 stars, solo |
| AgentCat SDKs (TS/Py/Go) | TS, Python, Go | None; trace id = sha256(session id) | none (`_meta` only for clientInfo) | No (`mcp.*` custom) | Wraps server object; workerd supported | Yes | MIT | Yes (2026-10-30), 127/53/10 stars |
| traceloop/openllmetry MCP | Python | Inject (client) + extract (server) via propagators | `_meta.traceparent` (client path writes `traceparent` only) | Traceloop conventions, not OTel MCP | stdio, SSE, streamable HTTP | Yes (6 files) | Apache-2.0 | Yes (2026-10-19), 7.5k stars, but open MCP bugs #4374, #4569, #4413 |
| Arize openinference MCP (Py+JS) | Python, TypeScript | Inject + extract via global propagator, no spans | `_meta` (propagator keys: `traceparent`, `tracestate`, `baggage`) | N/A (no spans) | SDK transports | Yes | Apache-2.0 | Yes (2026-10-30) |
| OTel contrib (go/py/js) | n/a | n/a (no MCP package) | n/a | n/a | n/a | n/a | Apache-2.0 | Repos active; MCP: only proposal #7616 (idle since 2025-09) |
| Langfuse | TS | none (OTLP backend; own MCP server) | n/a | n/a | n/a | n/a | NOASSERTION (mixed MIT/ee) | Very active |
| AmirK-S/mcp-opentelemetry | TS (SDK v2) | Inject + extract (requests and notifications) | `_meta.traceparent`/`tracestate`/`baggage` | OTel MCP semconv | stdio and HTTP examples | Yes | MIT | New (2026-09-06); self-declared possible EOL |
| modelcontextprotocol/python-sdk built-in | Python | Inject + extract | `_meta.traceparent`/`tracestate` | OTel MCP semconv (attrs) | all SDK transports | Yes | MIT (HYPOTHESIS, not checked) | Yes (v2.3.0 2026-10-02) |
| dylibso/mcp-otel | TS | Custom object | `_meta.__traceContext` | No | n/a (demo) | none seen | BSD-3-Clause | Stale (2026-02-14) |
| theharithsa/opentelemetry-instrumentation-mcp | TS | none | none | No (`mcp.tool:{name}`) | n/a | none seen | ISC | Stale (2026-06-18) |
| threadwire/mcp-telemetry | Python | HTTP header continuation | HTTP `traceparent` header | Unknown | HTTP relay only | Yes (CI) | MIT | New (2026-08-30), 0 stars |

## 6. Decision-relevant implications

- The cross-library wire contract is `_meta.traceparent` (+ `tracestate`, `baggage`) and is now backed by the 2026-07-28 spec text. A new project should implement exactly this via a standard OTel propagator, not a custom key.
- Both a reading and a writing side are needed. Of the Go/JS server libraries, only AmirK-S and the official Python SDK cover both. There is no maintained Go client-side injector (HYPOTHESIS: none found in searches; the Go landscape is mcp-otel-go plus AgentCat Go).
- The OTel contrib Go MCP proposal (#7616) is idle, and the semconv still has Development status with open alignment issues (#437), so an upstream contribution there is possible but slow.
- The highest-value, lowest-risk fixes to existing projects: openllmetry #4374 (Python client injection), and a client-side `Inject` plus notification handling for mcp-otel-go.
- Upstream SDKs may absorb this: Python SDK already ships it; TS SDK has open issue #1264; go-sdk has none. Watch these before investing in a separate library.

## 7. Method notes and limits

- I cloned (depth 50) mcp-otel-go, opentel-mcp, three AgentCat SDKs, mcp-opentelemetry, theharithsa's repo, threadwire/mcp-telemetry, openllmetry and openinference, and read the files cited. I fetched other files via raw.githubusercontent.com.
- Not done: running any library's tests, building anything, verifying runtime behavior, reading Shinzo's or mcp-use's code, reading issue comment threads beyond #4374's body, checking whether openinference JS tests exercise real transports, and reading `.github` CI for coverage thresholds.
- "Last commit" for single repos means the cloned HEAD commit date; monorepo values are the last commit touching the package path.
