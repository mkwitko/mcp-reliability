# 07. Roadmap

Status: plan, not commitment. Pre-alpha. Dates are targets set 2026-10-30.

## M0: PoC (done 2026-10-30)
Scope: loader, 3-5 rules, one broken fixture, `doctor` prints findings.
Shipped: `doctor` with `-json` and `-strict`; loader for Collector fileexporter OTLP JSON; 9 rules (MCP-SEM-001/002, MCP-ERR-001/002/003, MCP-PROP-001/002/003, MCP-PRIV-001); go-sdk Streamable HTTP scenario harness with four failure flags; collector config; 5 fixtures pinned by tests; CI (gofmt, vet, test -race, govulncheck, staticcheck).
Exit criteria, as met:
- `go run ./cmd/mcp-reliability doctor testdata/broken-propagation.jsonl` runs and reports MCP-PROP-001 (6 findings). Severity is WARN, not FAIL: the semconv says instrumentation SHOULD propagate via `params._meta`, so a FAIL would overstate the requirement.
- `healthy.jsonl` produces zero findings.
- Each rule cites its source URL.

## M1: v0.1
Scope: `doctor` with the full initial rule set, go-sdk scenario (`examples/go-sdk-streamable-http` with the three failure flags), `examples/collector`, fixtures, CI.
Exit criteria:
- Every rule has a passing and a failing fixture; `go test -race ./...` green.
- CI green: gofmt, vet, test -race, govulncheck, staticcheck.
- Scenario regenerates the fixtures from the documented command.
- Tagged v0.1.0 with changelog and README quick start that actually works.

## M2: Python and TS scenarios, compatibility report
Scope: Python and TS SDK scenarios (candidates from research: openllmetry, openinference, official Python SDK), compatibility matrix of libraries vs. rules.
Exit criteria:
- At least 2 non-Go scenarios with fixtures.
- Published compatibility report with library versions, commands to reproduce, and doctor output.
- Every claim in the report reproducible from the repo.

## M3: Upstream PRs
Candidate PRs from research (each only if the doctor evidence supports it):
1. mcp-otel-go: client-side inject (server extract only today); span status set to `Ok` on success (semconv MUST leave it unset); `error.type` holding Go type names instead of `tool_error`/JSON-RPC codes; caller-error codes marked as failures. Drafts in `docs/upstream/mcp-otel-go.md`.
2. traceloop/openllmetry #4374: client injection skipped when `params.meta` is None.
3. MCP conformance suite: a `_meta.traceparent` scenario (suite has no telemetry tests today).
4. opentelemetry-go-contrib #7616: MCP instrumentation proposal (open, idle since 2025-09).
Exit criteria: all four PRs or comments opened with reproduction; at least one merged or receives maintainer feedback. A rejection is an acceptable result.

## M4: Weaver integration
Scope: validate attribute names, enum values, and requirement levels via `weaver registry live-check` against the semconv-genai registry instead of rebuilding that logic.
Exit criteria:
- Documented, tested path from a fileexporter file to weaver (research notes weaver cannot read it directly; a Collector replay is the hypothesized route, unverified).
- Hand-written attribute rules removed or reduced where weaver covers them.
- If the path needs more than a documented script, decide in an ADR whether to proceed.

## M5: Optional AI layer
Gated on evidence (see `docs/09-validation-plan.md`): users asking for explanations or triage that deterministic output does not give.
Exit criteria:
- Written evidence from at least 3 external users.
- Design keeps core deterministic, opt-in, BYOK, no trace upload by default.
- Otherwise: not built.
