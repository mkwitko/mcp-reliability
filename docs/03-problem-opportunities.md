# Problem opportunities

Scores are the author's judgement on 2026-10-30, based on `docs/research/*.md`. They are not
measured. Scale 1-5, higher is better for us on every axis:

- **Demand**: evidence that someone asked for or hit this.
- **Feasibility**: can one person build it soon.
- **Diff.** (differentiation): how little existing tools do this.
- **Comp.** (competition): 5 means no competitor, 1 means crowded or solved.
- **Maint.** (maintenance cost): 5 means cheap to keep alive.
- **Comm.** (community relevance): matters to the MCP/OTel community.
- **Upstream**: can the result land in an existing project.

## Table

| # | Candidate | Demand | Feas. | Diff. | Comp. | Maint. | Comm. | Upstream | Total /35 | Verdict |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | `doctor` trace-file linter (MCP contracts + cross-span) | 3 | 4 | 5 | 5 | 3 | 4 | 3 | 27 | Pursue (primary) |
| 2 | Interop scenario harness (go-sdk client+server, fault flags) | 3 | 4 | 4 | 4 | 3 | 4 | 3 | 25 | Pursue, folded into 1 |
| 3 | Go client-side `Inject` for mcp-otel-go | 3 | 5 | 2 | 3 | 5 | 3 | 4 | 25 | Upstream PR |
| 4 | Conformance scenario for `_meta.traceparent` format | 3 | 3 | 5 | 5 | 4 | 5 | 5 | 30 | Upstream PR |
| 5 | Fix openllmetry #4374 and add `tracestate`/`baggage` | 4 | 4 | 2 | 3 | 5 | 3 | 5 | 26 | Upstream PR |
| 6 | New Go MCP OTel library | 2 | 4 | 1 | 2 | 2 | 3 | 2 | 16 | Do not build |
| 7 | Server-side extract in Go | 1 | 5 | 1 | 1 | 5 | 2 | 1 | 16 | Already adequately solved |
| 8 | Python SDK built-in spans | 1 | 5 | 1 | 1 | 5 | 2 | 1 | 16 | Already adequately solved (server) |
| 9 | go-contrib MCP instrumentation (#7616) | 2 | 3 | 2 | 4 | 3 | 4 | 5 | 23 | Opportunistic upstream |
| 10 | AI diagnosis layer over `doctor` output | 1 | 3 | 3 | 3 | 2 | 2 | 1 | 15 | Deferred |
| 11 | Stdio/HTTP proxy that injects or records traces | 2 | 2 | 3 | 3 | 1 | 3 | 1 | 15 | Not now |

Rows 7 and 8 are scored as "problem exists, someone already solved it", so low differentiation and
competition scores are the point.

## Rationale

**1. `doctor` linter.** No tool checks client/server trace sharing, parent presence, orphan spans or
failed-call status on stored traces. Weaver does attribute-level checks but reads only live gRPC and
has no cross-span checks (https://github.com/open-telemetry/weaver, 2026-10-30). Demand is moderate:
SDK maintainers' bugs (python-sdk #3174, #3715; typescript-sdk #1231; php-sdk #583) fit the rules,
but no end-user request for a linter exists (`platforms-and-tooling.md` C.5). Maintenance risk:
fileexporter is alpha and the semconv is Development.

**2. Scenario harness.** Gives `doctor` ground truth: known-good and known-bad traces from go-sdk
v1.8.0 over Streamable HTTP, with flags for no propagation, missing `error.type`, leaked arguments.
The Go SDK exposes `Middleware` and `GetMeta/SetMeta`, so hand-rolled middleware is small
(https://github.com/modelcontextprotocol/go-sdk, 2026-10-30). Also serves as the compatibility
report. Cost grows with every added SDK.

**3. Go client inject.** mcp-otel-go uses only `Extract`; grep for `Inject` found nothing
(https://github.com/olgasafonova/mcp-otel-go, 2026-10-30). Small, useful PR. Risk: single
maintainer, no issue history to judge responsiveness.

**4. Conformance scenario.** The suite has no telemetry coverage; the spec rule (W3C format in
`_meta`) is normative. HYPOTHESIS: a format-only check fits the existing wire-capture design, while
end-to-end propagation would need an OTLP side channel the harness lacks
(https://github.com/modelcontextprotocol/conformance, 2026-10-30). Highest upstream potential.
Maintainers are busy with 2026-07-28 work, so acceptance is uncertain.

**5. openllmetry #4374.** Confirmed in code: injection sits inside `if meta and len(args) > 0`,
skipped when `params.meta` is None (https://github.com/traceloop/openllmetry/issues/4374). Active
maintainers, Apache-2.0. Caveat: 820 open issues repo-wide and its own `traceloop.*` conventions.

**6. New Go library.** Rejected. Existing libraries are small and fragmented; the Go SDK maintainers
declined built-in OTel (go-sdk#59 `not_planned`) but welcome hooks
(https://github.com/modelcontextprotocol/go-sdk/issues/59). Another library adds maintenance and a
seventh incompatible shape. Contributing to #3 and #9 is cheaper.

**7. Server-side extract in Go: already adequately solved.** mcp-otel-go extracts via propagator
before starting a SERVER span, handles both error channels, ships ~1.7k lines of tests
(https://github.com/olgasafonova/mcp-otel-go, 2026-10-11 push). Gaps are `jsonrpc.request.id`,
`rpc.response.status_code`, `mcp.protocol.version` (HYPOTHESIS: absent) and notifications.

**8. Python built-in spans: already adequately solved on the server.** v2 ships default-on
`OpenTelemetryMiddleware` with the semconv attributes and both error channels
(https://github.com/modelcontextprotocol/python-sdk, v2.3.0 2026-10-02). Client-side output was not
fully read; #3174 and #3715 show gaps. Those are inputs for `doctor`, not a reason for a library.

**9. go-contrib #7616.** Idle since 2025-09; a maintainer said his own version stays private because
MCP and the semconv are unstable
(https://github.com/open-telemetry/opentelemetry-go-contrib/issues/7616). A PR would likely be asked
to wait for stability. Worth a comment, not an investment.

**10. AI layer.** No demand evidence, and deterministic rules do not exist yet. If built: BYOK,
structured output, evidence refs to span ids, off by default. Revisit after rules ship and users
ask.

**11. Proxy.** Spec is silent on intermediaries injecting `traceparent` (HYPOTHESIS that it is
permitted; `docs/research/mcp-ecosystem.md` section 1.3). Must handle both stateful and stateless
protocol modes and avoid double spans. Small existing tools (td-02/mcp-observer, 4 stars) show
interest but no traction (https://github.com/td-02/mcp-observer). High maintenance, so no.

## Combined pick

Rows 1+2 as the product; 3, 4, 5 as upstream work that raises the quality of the traces `doctor`
reads; 9 opportunistically; 6, 10, 11 not now; 7, 8 left alone.
