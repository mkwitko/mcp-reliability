# MCP ecosystem research: protocol, SDKs, OpenTelemetry

Verified: 2026-10-30 (all "verified" dates below are this date unless stated). Method: `gh api` and raw.githubusercontent.com reads of spec text, SDK source, schema YAML and issue threads. CONFIRMED means I read the spec text, source file, or issue/PR thread itself. HYPOTHESIS means inference not backed by something I read.

## 1. MCP specification

### 1.1 Versions and dates (CONFIRMED)

Source: https://github.com/modelcontextprotocol/modelcontextprotocol/releases (via `gh api .../releases`) and the `docs/specification/` directory listing.

| Version | Release date | Notes |
|---|---|---|
| 2024-11-05 | earlier | original |
| 2025-03-26 | 2025-03-26 | introduced Streamable HTTP, deprecated HTTP+SSE |
| 2025-06-18 | 2025-06-18 | |
| 2025-11-25 | 2025-11-25 (RC 2025-11-15) | last stateful release; has `initialize` handshake and `Mcp-Session-Id` |
| 2026-07-28 | 2026-07-28 (RC 2026-05-29) | latest; major redesign, see below |
| draft | n/a | tracks next revision |

Changelog for 2026-07-28: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/changelog.mdx (verified 2026-10-30). Items that matter for an observability tool:

- Protocol-level sessions and the `Mcp-Session-Id` header are removed (SEP-2567). The `initialize` / `notifications/initialized` handshake is removed (SEP-2575). Each request carries `io.modelcontextprotocol/protocolVersion` and `io.modelcontextprotocol/clientCapabilities` in `_meta`, plus optionally `clientInfo`. Servers SHOULD return `io.modelcontextprotocol/serverInfo` in each result's `_meta`. New required RPC `server/discover`.
- `ping`, `logging/setLevel`, `notifications/roots/list_changed` removed. Roots, Sampling and Logging are deprecated (SEP-2577). The changelog suggests "log to stderr (stdio) or use OpenTelemetry instead of Logging".
- Tasks moved to an official extension `io.modelcontextprotocol/tasks` (SEP-2663). Server-initiated requests replaced by Multi Round-Trip Requests (`InputRequiredResult`, `resultType: "input_required"`; SEP-2322). All results carry required `resultType`.
- SSE stream resumability (`Last-Event-ID`) removed from Streamable HTTP. A broken response stream loses the in-flight request and the client re-issues it with a new request id (SEP-2575).
- Streamable HTTP POST must carry `Mcp-Method` and `Mcp-Name` headers (SEP-2243). Go SDK constants confirm these headers (`mcp/streamable_headers.go`).
- Results from list endpoints carry `ttlMs` and `cacheScope` (SEP-2549).
- OTel trace context in `_meta` is documented (SEP-414); see 1.3.

HYPOTHESIS: because there are no sessions and no handshake in 2026-07-28, an observer cannot rely on `initialize` to learn a connection's version or server identity. It must read `_meta` on every request. A proxy needs to handle both modes while 2025-11-25 and earlier servers remain deployed.

### 1.2 JSON-RPC error codes (CONFIRMED)

Source: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx, section "Error Codes" (verified 2026-10-30).

- Standard JSON-RPC: `-32700` parse error, `-32600` invalid request, `-32601` method not found, `-32602` invalid params, `-32603` internal error.
- Allocation policy (new in 2026-07-28): `-32000` to `-32019` is "legacy", implementation-defined, existing SDK use grandfathered. `-32020` to `-32099` is reserved for the MCP spec.
- MCP-defined: `-32020` `HeaderMismatch`, `-32021` `MissingRequiredClientCapability`, `-32022` `UnsupportedProtocolVersion`.
- Historic: `-32002` resource not found (2025-11-25 and earlier), replaced by `-32602` in 2026-07-28; receivers should still accept `-32002` from older peers. `-32042` URL elicitation required (2025-11-25 only). Note: the numbers `-32001/-32003/-32004` appeared in a draft and were renumbered.
- Tool-execution failures are not JSON-RPC errors. They are a normal result with `isError: true`. The OTel convention maps this to `error.type = tool_error` (see section 4). Open spec issue modelcontextprotocol#3003 (open, 2026-07-02) notes a gap on schema-governed error signalling for tool errors.

Implication (CONFIRMED by reading both): any error classification must look at both the JSON-RPC `error.code` and `result.isError`.

### 1.3 `_meta` semantics (CONFIRMED)

Source: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx, section "`_meta`" (verified 2026-10-30).

- `_meta` is a property bag on requests, notifications and results. Key format: optional prefix (dot-separated labels ending in `/`) plus name. Reverse-DNS prefixes recommended. Any prefix whose second label is `modelcontextprotocol` or `mcp` is reserved for MCP. `com.example.mcp/` is not reserved.
- Reserved keys table: `progressToken`, `io.modelcontextprotocol/protocolVersion`, `.../clientInfo`, `.../clientCapabilities`, `.../logLevel`, `.../subscriptionId`, and unprefixed `traceparent`, `tracestate`, `baggage`.
- `progressToken` (progress.mdx): string or integer, unique across active requests; including it opts the request into `notifications/progress`.
- Trace context exception: "the keys `traceparent`, `tracestate`, and `baggage` are reserved for OpenTelemetry trace context propagation. When present, their values MUST follow W3C Trace Context and W3C Baggage formats." The spec gives a non-normative `tools/call` example with `_meta.traceparent`. It explicitly cites the OTel semconv for MCP as the reason for the unprefixed exception.
- Peers MUST NOT make assumptions about values at reserved keys. HYPOTHESIS: a proxy that injects `traceparent` where the client did not send one is permitted by this text, but the text says nothing about proxies. Not tested.

### 1.4 Does the spec say anything about tracing / traceparent / OTel? YES, but only recently

CONFIRMED. Search of the spec tree: `traceparent` appears in `docs/specification/draft` and `docs/specification/2026-07-28` and NOT in `2025-11-25` (code search `traceparent repo:modelcontextprotocol/modelcontextprotocol path:docs` returned only `draft/basic/index.mdx`, `2026-07-28/basic/index.mdx`, `2026-07-28/changelog.mdx`, `docs/seps/414-request-meta.mdx`). So the convention is normative-ish only from 2026-07-28; before that it was a de-facto convention.

Chronology (all CONFIRMED by reading the threads):

- Issue #246 "Include OpenTelemetry Trace identifiers as part of the MCP client -> server protocol" (https://github.com/modelcontextprotocol/modelcontextprotocol/issues/246). Opened 2025-03-27 by an OTel-community contributor. Maintainer position (jspahrsummers, 2025-04-23): "All MCP object types are open in the spec, so if the ecosystem can agree on a convention for this, it's already possible to inject." No new protocol field added. Closed 2025-12-05 by a TS SDK maintainer as moved to the TS v2 wishlist. State: closed.
- Discussion #269 "Adding OpenTelemetry Trace Support to MCP" (closed).
- PR #414 "SEP-414: Document OpenTelemetry Trace Context Propagation Conventions" (https://github.com/modelcontextprotocol/modelcontextprotocol/pull/414). Author Adrian Cole, sponsor Marcelo Trylesinski (Python SDK maintainer). Merged 2026-02-26 by pja-ant. SEP file `seps/414-request-meta.md` has Status: Final. It is a documentation SEP: it adds the reserved-key text and example, and "documents an exception to the DNS prefixing convention". It also notes that namespacing like `io.modelcontextprotocol.traceparent` "will break traces".
- Tracking issue #2868 "(implementation): SEP-414" (https://github.com/modelcontextprotocol/modelcontextprotocol/issues/2868) lists per-SDK implementation issues (TS, C#, Go, Java, PHP, Python, Ruby, Rust ticked; Kotlin and Swift unticked). Closed 2026-08-23 by localden: "tracking will be split for Tier 1 SDKs".
- Related SEPs:
  - SEP-1788 (issue #1788) "Clarify `_meta` Property Reserved Keys": closed 2026-06-26. The SEP-414 text says SEP-1788 "should be updated" with the trace keys; the spec table now contains them.
  - SEP-2028 (PR #2028, https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2028) "Automatic `_meta` to HTTP header forwarding for distributed tracing": OPEN, labelled proposal/SEP, no sponsor as of 2026-03-24 (author asked how to find one), automation bot reminder on 2026-10-19 after 95 days of inactivity. A TS SDK PR (#666) doing the same was closed by a maintainer in June saying it must be at protocol level first. VS Code shipped a `_meta.traceparent` fix in 1.119.0 (per author comment; not independently verified).
  - SEP-1763 (issue #1763) "Interceptors for Model Context Protocol": status Draft in the file, issue closed 2026-04-22. Would add an interceptor extension point for validate/mutate/observe. Not adopted in 2026-07-28 changelog.
  - Issue #2638 "Data lineage for tool calls" closed by a maintainer as AI-generated, no concrete mapping (2026-04-23). Shows low appetite for vague observability proposals.
- No SEP exists that defines MCP-native telemetry export, metrics, or a telemetry capability. The spec position is: propagate context through `_meta`; semantics live in OTel semconv; logging to stderr or OTel (Logging feature deprecated).

SEP process (CONFIRMED, https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/community/sep-guidelines.mdx): SEP files live in `seps/`; types Standards Track, Informational, Process, Extensions Track; needs a sponsor (awaiting sponsor up to 6 months, else Dormant); statuses Draft, In-Review, Accepted/Rejected, Final; core maintainers decide. An Extensions Track SEP is the route for something that is not core protocol (HYPOTHESIS that an observability extension would go that route; nothing proposes one).

### 1.5 Transports, lifecycle, cancellation, timeouts (CONFIRMED)

Sources: `docs/specification/2026-07-28/basic/transports/{index,stdio,streamable-http}.mdx`, `basic/patterns/cancellation.mdx`, `basic/patterns/progress.mdx`, changelog (all verified 2026-10-30).

- Transports: stdio and Streamable HTTP. HTTP+SSE (from 2024-11-05) is Deprecated under the feature lifecycle policy (SEP-2596; originally deprecated 2025-03-26).
- Lifecycle in 2026-07-28: no initialize; version/capabilities per request in `_meta`; `server/discover` for capability probing; `UnsupportedProtocolVersionError` (-32022) on mismatch. Servers must not rely on prior requests on the connection; stdio processes are not a lifetime boundary for a task/conversation.
- Cancellation: Streamable HTTP: closing the response stream IS the cancel signal, server MUST treat disconnect as cancellation, no `notifications/cancelled` expected. stdio: client MUST send `notifications/cancelled` with `requestId` (and optional `reason`). Servers send `notifications/cancelled` only to tear down a `subscriptions/listen` stream.
- Timeouts: senders SHOULD set timeouts on all requests; on expiry SHOULD cancel (close stream / send cancelled). SDKs SHOULD make timeouts configurable per request. MAY reset the clock on progress notifications; SHOULD enforce a maximum regardless. The spec defines no numeric timeout and no error code for timeout; there is no wire-level timeout error. HYPOTHESIS: timeouts are therefore only observable client-side (a local error, or a stdio `notifications/cancelled`), and a server-side observer sees a cancel/disconnect, not a timeout.
- Progress and request-scoped notifications flow on the response stream of the originating request.

## 2. Official Go SDK (modelcontextprotocol/go-sdk)

### 2.1 Version, stability, module path (CONFIRMED)

- Module path: `github.com/modelcontextprotocol/go-sdk` (`go.mod` on main; requires Go 1.25.0). Packages: `mcp`, `jsonrpc`, `auth`, `oauthex`. Source: https://github.com/modelcontextprotocol/go-sdk.
- Latest release v1.8.0, 2026-09-14 (v1.7.0 on 2026-07-28; 1.8.0-pre.1/2 on 2026-09-04). v1 line, semver, stable by the project's own versioning (HYPOTHESIS on "stable" meaning: I did not read a formal stability policy; the README has a version-compat table).
- README compat table (https://github.com/modelcontextprotocol/go-sdk/blob/main/README.md): v1.7.0+ targets 2026-07-28 and also supports 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05. `mcp/shared.go` has `latestProtocolVersion = protocolVersion20260728`.
- ~5.3k stars; pushed 2026-10-30 (active).

### 2.2 Middleware (CONFIRMED by reading source)

Source: https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/shared.go and `mcp/server.go`, `mcp/client.go`.

```go
type MethodHandler func(ctx context.Context, method string, req Request) (result Result, err error)
type Middleware func(MethodHandler) MethodHandler
func (s *Server) AddSendingMiddleware(middleware ...Middleware)     // server.go:1936
func (s *Server) AddReceivingMiddleware(middleware ...Middleware)   // server.go:1954
func (c *Client) AddSendingMiddleware(middleware ...Middleware)     // client.go:1179
func (c *Client) AddReceivingMiddleware(middleware ...Middleware)   // client.go:1197
```

Middleware wraps every method (requests and notifications) on that peer. `Request` exposes `GetSession()`, `GetParams()`, `GetExtra()` (server side: bearer `TokenInfo`, HTTP `Header`) and now `GetID() jsonrpc.ID`.

Caveat on `GetID`: it is newly added. Issue go-sdk#1264 "expose the JSON-RPC request id to receiving middleware and handlers" (https://github.com/modelcontextprotocol/go-sdk/issues/1264) was closed 2026-10-28; I found `GetID` in main's `mcp/shared.go`. I did not check which release contains it, so HYPOTHESIS: it is not in v1.8.0 (2026-09-14) and ships in the next release. Verify with `git tag --contains` before relying on it. Until then middleware cannot read the JSON-RPC id (the issue text says exactly that), which blocks the OTel `jsonrpc.request.id` attribute and request/response correlation by id.

### 2.3 `_meta` in Go (CONFIRMED)

`mcp/shared.go`: `type Meta map[string]any`, with `GetMeta()` and `SetMeta()`. It is embedded as `Meta `json:"_meta,omitempty"`` in every params and result struct (`mcp/protocol.go`, e.g. `CallToolParams`). So in a middleware, `req.GetParams().GetMeta()["traceparent"]` reads the inbound value, and a sending middleware can call `SetMeta` to inject. `getProgressToken` reads `Meta["progressToken"]`. There is no typed helper for trace keys and no automatic inject/extract in the SDK.

### 2.4 OpenTelemetry in the Go SDK (CONFIRMED)

- No built-in OTel. `go.mod` has no `go.opentelemetry.io` dependency. Code search for `opentelemetry` in the repo hits only `docs/server.md` and `mcp/client.go`, both of which are just the deprecation note telling servers to move from MCP logging to stderr or OpenTelemetry.
- Issue go-sdk#59 "OpenTelemetry support" (https://github.com/modelcontextprotocol/go-sdk/issues/59), opened 2025-06-27, closed `not_planned` 2025-07-19. Maintainer findleyr: "we shouldn't include explicit OpenTelemetry support as part of the SDK itself", while extensibility hooks are welcome. jba suggested a separate `mcp-go-sdk-otel` repo; the proposer (anuraaga) closed it after the org showed no interest in hosting one. His prototype `anuraaga/mcp-go-sdk-otel` last pushed 2025-06-27, i.e. stale (still not archived).
- Issue go-sdk#934 "Implement SEP-414" closed completed 2026-05-05: "No SDK work needed, filing for completeness". So the Go SDK treats `_meta` propagation as user code.
- Issue go-sdk#1264 (above) shows a real user instrumenting via receiving middleware, citing the MCP semconv in `semantic-conventions-genai`, and a maintainer (guglielmo-san, 2026-10-20) approving the accessor. This confirms the intended pattern: instrumentation lives outside the SDK, built on middleware.
- opentelemetry-go-contrib issue #7616 "Proposal: Support for OTel Implementation in go-mcp" is open (2025-07-24); no instrumentation package for MCP exists in go-contrib (tree search for "mcp" returned nothing). A community maintainer said in 2025-09 that his own version is private/incubating because both MCP and the semconv are unstable.

### 2.5 Streamable HTTP APIs (CONFIRMED, `mcp/streamable_server.go`, `mcp/streamable_client.go`, `mcp/streamable_headers.go`)

- Server: `mcp.NewStreamableHTTPHandler(getServer func(*http.Request) *Server, opts *StreamableHTTPOptions) *StreamableHTTPHandler` is an `http.Handler`. Options include `Stateless`, `JSONResponse` (plain JSON instead of SSE), and an `EventStore` for resumability (stream events `Open/Append/After/SessionClosed`). Stateless mode ignores `Mcp-Session-Id`.
- Lower level: `StreamableServerTransport`, `StreamableClientTransport`.
- Client: `StreamableClientTransport` has `HTTPClient` (custom `*http.Client`, so an `otelhttp` transport can be injected) and `MaxRetries` (default 5). Issue #926 added Headers/MaxResponseBytes.
- Constants in `streamable_headers.go`: `Mcp-Protocol-Version`, `Mcp-Session-Id`, `Last-Event-ID`, `Mcp-Method`, `Mcp-Name`, `Mcp-Param-` prefix.
- SSE transport (`sse.go`) is marked deprecated.
- stdio: `StdioTransport`, `CommandTransport`; `Transport` interface in `mcp/transport.go`. Open issue go-sdk#1209: a malformed JSON frame terminates the stdio session (relevant for reliability tooling).
- go-sdk#1150 (closed 2026-08-06): best-effort cancellation notification blocked the caller past its deadline. go-sdk#513 (closed): context propagation broken in StreamableClientTransport.

HYPOTHESIS: because Streamable HTTP stream close means cancel (spec 1.5) and the Go client sends best-effort cancellation, a Go-based proxy sitting between client and server should treat an upstream connection drop as cancellation and propagate it, not as an error. Not tested.

## 3. Python and TypeScript SDKs

### 3.1 Python (modelcontextprotocol/python-sdk)

- Releases: v2.3.0 (2026-10-02), v2.2.0 (2026-09-07), v1.30.0 (2026-09-07) (`gh api releases`). The `main` branch is v2 (imports `mcp_types`).
- OTel is built in on v2 (CONFIRMED by reading source): `src/mcp/shared/_otel.py` (tracer `mcp-python-sdk`, `inject_trace_context(meta)` and `extract_trace_context(meta)` using the global OTel propagators on the `_meta` dict) and `src/mcp/server/_otel.py` (`OpenTelemetryMiddleware`, `SpanKind.SERVER`, span name `"{method} {name}"`, attributes `mcp.method.name`, `mcp.protocol.version`, `jsonrpc.request.id`, and for `tools/call` `gen_ai.operation.name=execute_tool` + `gen_ai.tool.name`, for `prompts/get` `gen_ai.prompt.name`, error attrs `error.type`, `rpc.response.status_code`, and `error.type=tool_error` when `isError`). `opentelemetry-api>=1.28.0` is a hard dependency in `pyproject.toml`. Both files exist at tag v2.3.0 and are 404 at v1.30.0, so v1.x has no built-in OTel.
- PR python-sdk#2995 (closed) made the OTel middleware the single default: `Server.__init__` seeds `Server.middleware` with `OpenTelemetryMiddleware()`, a no-op until an exporter is installed; remove it from the list to opt out. Docs page `docs/advanced/opentelemetry.md`. Earlier PR #1693 "pluggable instrumentation interface" was closed unmerged (but cited as reference implementation in SEP-414). Issue #2387 "richer OTel MCP span attributes" is open (session id, resource uri). #3629 (closed) records JSON-RPC error responses on the client span. #3715 (open, 2026-10-15) asks for an outbound request seam so observers see a request before it is written.
- python-sdk#2790 (SEP-414 implementation) closed.
- Note: the Python implementation uses `mcp.method.name`, `jsonrpc.request.id` etc. which are exactly the semconv attribute names (see 4); it does not emit the `mcp.session.id`, `network.transport`, or metrics (CONFIRMED from the middleware source shown; I did not read the client side beyond `inject_trace_context`).

### 3.2 TypeScript (modelcontextprotocol/typescript-sdk)

- Releases: `@modelcontextprotocol/server@2.4.0` / client etc. 2026-10-23; `1.32.2` (v1 line) 2026-10-23.
- SEP-414 reserved key constants (`TRACEPARENT_META_KEY`, `TRACESTATE_META_KEY`, `BAGGAGE_META_KEY`) exist in `packages/core/src/constants.ts` (CONFIRMED); merged via PR #2270 (2026-06-15). Constants and docs only, no spans.
- Built-in OTel tracing is NOT in a release: PR typescript-sdk#3115 "feat: built-in OpenTelemetry tracing and metrics" is OPEN (opened 2026-10-27). It says Client/Server/McpServer will emit spans and duration metrics per MCP semconv and carry context in `params._meta`, on by default, `openTelemetry: false` to disable. A maintainer review (2026-10-28) suggests an optional override for propagator/provider. It implements issue #1264 "OpenTelemetry integration out of the box" (open since 2025-12-08). I read the PR description and a review comment only, not the diff: HYPOTHESIS about final shape and release timing.
- Community note (issue #3115 comment, 2026-10-18): on 1.32.1 and 2.3.1, manual inject on client and extract on server through `_meta` over stdio works.
- Third-party: Arize OpenInference MCP instrumentations for Python and JS (cited in SEP-414 as reference implementations; their code I did not read), Logfire's MCP integration, Envoy AI Gateway, ToolHive (all cited by SEP-414; not verified by me).

## 4. OpenTelemetry semantic conventions for MCP

### 4.1 Where it lives (CONFIRMED)

- Canonical doc (as of 2026-10-30): https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md. Attribute registry: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/registry/attributes/mcp.md. Models: `model/mcp/{common,metrics,registry,spans}.yaml` in that repo. Last commit touching `mcp.md`: 2026-10-17.
- The old location `open-telemetry/semantic-conventions/docs/gen-ai/mcp.md` is now a stub saying "GenAI semantic conventions have moved" to `semantic-conventions-genai`. `model/mcp/deprecated/` and `model/gen-ai/deprecated/` remain in the old repo. Several older links in SEP-414 and the TS PR point at the old path; use the new one.
- Overall status: **Development** (document-level `Status: Development`, every MCP span/metric/attribute badge `development`). Nothing MCP-specific is Stable. Stable items it reuses: `error.type`, `network.*`, `server.address`, `server.port`. `rpc.response.status_code` is Release Candidate. `jsonrpc.*` attributes are Development.
- The doc references the general semconv v1.44.0 release.
- HYPOTHESIS: names may still change (breaking) before stability. The relocation itself shows churn.

### 4.2 Context propagation rules (CONFIRMED, mcp.md "Context propagation")

- Instrumentations SHOULD inject context with the configured OTel propagators into `params._meta` when creating a request or notification; the receiver extracts from `params._meta` and uses it as the remote parent. Keys are unprefixed `traceparent`, `tracestate`, `baggage` (SEP-414 exception).
- Server instrumentation SHOULD by default use the extracted context as parent of the server span and SHOULD link the ambient (transport) context if present. MCP context and HTTP context are independent (one MCP request can span several HTTP requests; one HTTP request can carry several MCP messages); span links can record the transport context.

### 4.3 Spans (CONFIRMED)

Both span types: stability Development, overall requirement level Recommended.

| | MCP client span | MCP server span |
|---|---|---|
| Kind | SHOULD be `CLIENT` | SHOULD be `SERVER` |
| Name | `{mcp.method.name} {target}`; target = `gen_ai.tool.name` or `gen_ai.prompt.name`; else just `{mcp.method.name}`. `mcp.resource.uri` MAY be opted in as target, SHOULD NOT by default (cardinality). | same |
| Emitted by | the side that initiates a request/notification | the side that receives it |

Attributes (key: level), from the tables I read:

- Required: `mcp.method.name` (string).
- Conditionally Required: `error.type` (if and only if the operation fails); `gen_ai.tool.name` (when about a specific tool); `gen_ai.prompt.name` (when about a specific prompt); `jsonrpc.request.id` (when the peer executes a request; not captured for null/omitted id); `mcp.resource.uri` (for `resources/read`, `resources/subscribe`, `resources/unsubscribe`, `notifications/resources/updated`; Conditionally Required on the sender/server spans table, Opt-In in the metrics table); `rpc.response.status_code` (if the response carries an error code).
- Recommended: `gen_ai.operation.name` (SHOULD be `execute_tool` for tool calls, SHOULD NOT be set otherwise); `mcp.protocol.version` (e.g. `2025-06-18`); `mcp.session.id` (when part of a session); `jsonrpc.protocol.version` (only when not `2.0`); `network.transport` (`pipe` for stdio; `tcp` or `quic` for HTTP); `network.protocol.name`, `network.protocol.version`; client span also `server.address`, `server.port`; server span also `client.address`, `client.port`.
- Opt-In: `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result` (may hold sensitive data; structured object, JSON-string allowed on spans if structured unsupported), `gen_ai.prompt.variable.<arg>`.
- Error rules: when the JSON-RPC response carries an error code, `error.type` and `rpc.response.status_code` are both the string form of that code and all JSON-RPC error codes are errors. When the result has `isError: true`, `error.type` SHOULD be `tool_error`. Span status description SHOULD match `JSONRPCError.message`. Timeouts and transport errors: `error.type` per the general guidance (e.g. `timeout`).
- Registry attribute names in the MCP namespace: `mcp.method.name`, `mcp.protocol.version`, `mcp.resource.uri`, `mcp.session.id`. The `mcp.method.name` enum lists methods (e.g. `tools/call`, `tools/list`, `prompts/get`, `resources/read`, `initialize`, `ping`, `notifications/*`), all Development. I did not see `server/discover`, `subscriptions/listen`, or tasks methods in the table I read (HYPOTHESIS: the semconv lags the 2026-07-28 spec; custom values are allowed by the "MAY use a custom value" rule).
- "Transport recording" table in the doc: stdio gives `network.transport=pipe`; Streamable HTTP gives `tcp`/`quic` with `network.protocol.name=http`; `mcp.protocol.version` of `2025-06-18` or newer distinguishes Streamable HTTP from SSE.

### 4.4 Metrics (CONFIRMED)

All Histogram, unit `s`, stability Development, requirement level Recommended: `mcp.client.operation.duration`, `mcp.server.operation.duration`, `mcp.client.session.duration`, `mcp.server.session.duration`. Operation histograms SHOULD use explicit bucket boundaries `[0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300]`. Operation metrics reuse `mcp.method.name` (Required), `error.type`, `gen_ai.tool.name`, `gen_ai.prompt.name`, `rpc.response.status_code`, `gen_ai.operation.name`, `mcp.protocol.version`, `network.*`; session metrics have `error.type` (if the session ends with error), `mcp.protocol.version`, `network.*`, `server.address`, `server.port`. HYPOTHESIS: session duration metrics are less meaningful under the 2026-07-28 stateless protocol (no sessions); the semconv still references sessions.

### 4.5 GenAI `execute_tool` (CONFIRMED)

Source: https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md, section "Execute tool span".

- Status Development. `gen_ai.operation.name` SHOULD be `execute_tool`; span name SHOULD be `execute_tool {gen_ai.tool.name}`; kind SHOULD be `INTERNAL`; requirement level Recommended.
- Attributes: Required `gen_ai.operation.name`, `gen_ai.tool.name`; Conditionally Required `error.type`, `gen_ai.agent.name` (when applicable), `gen_ai.conversation.id` (if available), `gen_ai.tool.call.id`; Recommended `gen_ai.tool.description`, `gen_ai.tool.type`; Opt-In `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`. The `gen_ai.operation.name` well-known-values list includes `execute_tool` (Development).
- Relationship with MCP: the MCP client doc says "MCP tool call execution spans are compatible with GenAI execute_tool spans." If instrumentation can reliably detect that an outer GenAI instrumentation already traces the tool execution it SHOULD NOT create a separate span and SHOULD instead add MCP attributes to that existing span (configurable). So an MCP `tools/call` span has the MCP name (`tools/call get_weather`) and kind `CLIENT`/`SERVER` rather than the `execute_tool` name/`INTERNAL` kind, but carries `gen_ai.operation.name=execute_tool` and `gen_ai.tool.name`. The Python SDK middleware does exactly this (3.1).

### 4.6 Contrib instrumentation packages (CONFIRMED by tree and issue searches, 2026-10-30)

- opentelemetry-go-contrib: no MCP package (tree grep), proposal #7616 open and unaddressed.
- opentelemetry-python-contrib: no MCP instrumentation (tree grep for "mcp" under instrumentation dirs returned only genai openai/util). The one MCP-mentioning PR (#5203, closed) is the OpenAI-agents processor handling `MCPListToolsSpanData`.
- opentelemetry-js-contrib: PR #3175 and #3186 `instrumentation-mcp` both closed without merge (2025-10). Open tracking issue #3668 (2026-08-12) to migrate OpenInference JS instrumentations into js-contrib.
- opentelemetry-java-instrumentation: open PRs #20062 (MCP Java SDK client instrumentation) and #20100 (server tool call instrumentation), both opened Sept 2026; not merged.
- HYPOTHESIS: no OTel-org-maintained MCP instrumentation exists for Go; any Go tool will be first-party code or a third-party library.

## 5. MCP Registry (CONFIRMED)

modelcontextprotocol/registry (https://github.com/modelcontextprotocol/registry, description "A community driven registry service for Model Context Protocol (MCP) servers", latest release v1.8.7 on 2026-10-30) is a metadata catalogue and REST API that hosts `server.json` records (name, description, version, repository, packages, remotes) so clients can discover servers; it does not proxy or run servers. I read the generated `docs/reference/server-json/draft/server.schema.json` (from `docs/reference/api/openapi.yaml`): the schema has no telemetry, OpenTelemetry or tracing field (grep count 0). The only extension point is `_meta`, which publishers must place under the key `io.modelcontextprotocol.registry/publisher-provided` when publishing to the official registry (`docs/reference/server-json/generic-server-json.md`). The registry's own `internal/telemetry/` package is about the registry service's own metrics, not server metadata. Whether servers declare OTel endpoints is therefore not standardised anywhere; a publisher could put it in `_meta` but that is an unspecified convention.

## Key facts for decision-making

1. Latest protocol is 2026-07-28 (released 2026-07-28), and it is stateless: no `initialize`, no `Mcp-Session-Id`, per-request `_meta` carrying version and capabilities. Earlier versions (2025-11-25 and older) remain in the field, so any observer must support both modes. (spec changelog)
2. Trace context in MCP is settled: unprefixed `_meta.traceparent`, `tracestate`, `baggage`, W3C formats, normative reserved keys in 2026-07-28 via SEP-414 (merged 2026-02-26, Final). Do not invent a different carrier or prefix. (spec basic/index.mdx)
3. The spec defines no telemetry export, metrics, or telemetry capability, and no SEP proposes one. HTTP-header forwarding of `_meta` (SEP-2028) is open and unsponsored, so do not depend on it.
4. Timeouts are client-side only: spec says SHOULD cancel on timeout (close stream on HTTP, `notifications/cancelled` on stdio); there is no timeout error code. A server-side proxy sees cancels and disconnects, not timeouts. (1.5; the inference is HYPOTHESIS)
5. Two error channels must both be watched: JSON-RPC `error.code` (standard codes, `-32020..-32022` MCP-reserved, `-32000..-32019` legacy) and `result.isError` on `tools/call`, which OTel maps to `error.type=tool_error`.
6. Go SDK v1.8.0 (module `github.com/modelcontextprotocol/go-sdk`) has `Add{Sending,Receiving}Middleware` on both Server and Client, `_meta` as `map[string]any` embedded in every params struct, and no OTel dependency; maintainers explicitly decline built-in OTel (go-sdk#59 `not_planned`, #934 "no SDK work needed"). Middleware-based instrumentation is the sanctioned route and is what a Go observability library should build on.
7. The JSON-RPC id accessor (`Request.GetID`) is on go-sdk main but I could not confirm it is in v1.8.0; verify before depending on it (`jsonrpc.request.id` is a Conditionally Required semconv attribute).
8. OTel MCP semconv is Development (not stable) and now lives in `open-telemetry/semantic-conventions-genai` (old repo has a stub). Required: `mcp.method.name` only; tool calls SHOULD also set `gen_ai.operation.name=execute_tool` + `gen_ai.tool.name`; spans named `{method} {target}`, kind SERVER/CLIENT; four Development histograms (`mcp.{client,server}.{operation,session}.duration`). Arguments/results are Opt-In and flagged sensitive.
9. Competitive landscape: Python SDK v2 already ships default-on OTel server spans using these attribute names; TypeScript SDK has an open PR (#3115, 2026-10-27) to do the same; Java instrumentation PRs open; no OTel-org Go/Python-contrib/JS-contrib MCP package is merged. An out-of-process Go tool/proxy that reads `_meta` and emits semconv-compliant spans would be compatible with, not duplicate, SDK-native instrumentation, but must avoid double-spanning when an SDK already traces (semconv says to add attributes to the existing span if detectable).
10. The MCP Registry `server.json` has no telemetry fields; only the free-form `_meta` extension point exists. Nothing to integrate with there today.

## Unresolved questions

1. Which go-sdk release first contains `Request.GetID()` (merged around 2026-10-28)? Needs `git tag --contains` on the commit.
2. How do the Go SDK's streamable server and client behave when a request `_meta` already holds `traceparent` and an HTTP `traceparent` header is also present? I did not read the code path for header/`_meta` precedence; semconv says they are independent contexts and recommends links.
3. Is a proxy allowed to inject or rewrite `traceparent` in `_meta`? The spec says only that receivers MUST NOT assume values at reserved keys; there is no statement about intermediaries.
4. Will typescript-sdk PR #3115 land and in which form (default-on vs opt-in, attribute set)? Not merged at the verification date.
5. Does the semconv-genai MCP doc plan to cover 2026-07-28 methods (`server/discover`, `subscriptions/listen`, task methods) and the removal of sessions (affects `mcp.session.id` and session duration metrics)? The method table I read still references `initialize`, `ping`, `logging/setLevel`.
6. What exactly does Python SDK v2 emit on the client side (span kind, attributes, metrics)? I read only the shared helpers and the server middleware.
7. Stability policy and long-term support timeline of Go SDK v1.x relative to spec revisions: not read.
8. Whether the SEP-1763 interceptor framework (Draft) will progress; it would change the extension story for observers. Issue closed 2026-04-22 but the SEP file status is Draft; maintainer position not read.
9. Are there other third-party MCP OTel libraries for Go (beyond `anuraaga/mcp-go-sdk-otel`, stale since 2025-06-27) with traction? Not searched beyond the OTel org and the Go SDK issue tracker.
