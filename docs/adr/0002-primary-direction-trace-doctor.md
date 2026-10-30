# 0002: Primary direction is a trace doctor

Status: accepted, 2026-10-30 (provisional until H1-H3 in `docs/09-validation-plan.md`)

## Context
Research (`docs/research/`) found: libraries already exchange W3C context through `_meta.traceparent`, but fail in different ways (one-sided injection, no-op default propagator, openllmetry#4374). The official conformance suite does not test telemetry. `weaver registry live-check` validates attributes but does not read fileexporter files. No existing tool checks MCP trace propagation end to end. Dashboards and backends are crowded.

## Decision
Build a local CLI, `mcp-reliability doctor`, that checks recorded traces against the spec and semconv, plus example scenarios and fixtures. Go first.

## Consequences
- Small surface, testable offline, useful to SDK maintainers via reports.
- Depends on a Development-status semconv that may change; rules must cite versions.
- Does not give users live monitoring.

## Alternatives
- Another instrumentation library: crowded, and the real gap is verification.
- Dashboard/backend/SaaS: out of scope, crowded, high cost.
- Conformance scenario only: planned as an upstream PR (M3), but needs this tooling to prove value.
