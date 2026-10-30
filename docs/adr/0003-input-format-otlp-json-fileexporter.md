# 0003: Input is OTLP JSON from the Collector fileexporter

Status: accepted, 2026-10-30

## Context
We need a vendor-neutral, reproducible trace input. The Collector `fileexporter` writes newline-delimited `ExportTraceServiceRequest` JSON. `go.opentelemetry.io/collector/pdata/ptrace.JSONUnmarshaler` parses it. Weaver cannot read this format directly (`docs/research/platforms-and-tooling.md`, B.2).

## Decision
`doctor` accepts only newline-delimited OTLP JSON traces, parsed with `ptrace.JSONUnmarshaler`, one request per line. `examples/collector` ships a Collector config for producing it.

## Consequences
- Works with any SDK that exports OTLP; no custom format.
- Needs a Collector (or any OTLP file writer) to capture traces.
- pdata is a heavy dependency for a small tool; accepted for parsing correctness.
- Other inputs (OTLP/HTTP listener, Jaeger/Tempo export) are deferred.

## Alternatives
- `protojson` with `go.opentelemetry.io/proto/otlp`: lighter, viable fallback.
- Weaver's own JSON sample format: not OTLP; would need conversion.
- Live OTLP receiver: adds networking and lifecycle; conflicts with local-only posture.
