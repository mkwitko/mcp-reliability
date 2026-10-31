# Technical feasibility

Date: 2026-10-30. Sourced claims carry a URL from `docs/research/*.md` (verified 2026-10-30). Items
marked **spike** come from the project's own spike run on 2026-10-30; they are not in the research
files and carry no external URL. HYPOTHESIS marks untested inference.

## Smallest experiment

Build `mcp-reliability doctor <trace.jsonl>`:

1. Read the file line by line. Each line is one OTLP `ExportTraceServiceRequest` batch (fileexporter
   default, "one line per batch":
   https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter).
2. Parse each line with `ptrace.JSONUnmarshaler.UnmarshalTraces` from
   `go.opentelemetry.io/collector/pdata`
   (https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/ptrace/json.go).
3. Merge spans across lines into per-trace sets, since "a line is a batch, and spans of one trace
   are spread across lines and files" (`platforms-and-tooling.md` B.1).
4. Run rule functions over the merged set; print findings; exit non-zero on violations.
5. Generate input from `examples/`: go-sdk v1.8.0 client and server over Streamable HTTP,
   hand-rolled OTel middleware on `mcp.Middleware`, `Params.GetMeta/SetMeta`, with fault flags (no
   propagation, missing `error.type`, leaked arguments), exported through an OTel Collector
   `fileexporter`.

Success: each fault flag produces exactly the expected finding and the clean run produces none.

## Spike results

- **spike**: `ptrace.JSONUnmarshaler` parsed fileexporter newline-delimited output correctly,
  confirmed 2026-10-30. Hex trace and span ids decoded. This matches the research expectation that
  pdata handles the OTLP hex deviation internally, a reason to prefer it over stock `protojson`
  (`platforms-and-tooling.md` B.4, HYPOTHESIS there, now supported by the spike).
- **spike**: the pdata dependency tree is 26 modules.
- Not yet spiked: rules over real go-sdk traces, `tracestate` / `baggage` format validators,
  behaviour on rotated or compressed files.

## Rule inputs and where each comes from

| Rule family | Input in the file | Spec source (2026-10-30) |
|---|---|---|
| `_meta` W3C format | `_meta.traceparent/tracestate/baggage` recorded as span attributes or events by the instrumentation; HYPOTHESIS: only present if the instrumentation records them | https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2026-07-28/basic/index.mdx |
| Semconv attributes | span attributes, kind, status | https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md |
| Same trace id, client and server | `traceId` of CLIENT and SERVER spans for the same `mcp.method.name` + `jsonrpc.request.id` | same |
| Parent present / orphan | `parentSpanId` resolves to a span in the file | OTLP structure |
| Failed call with UNSET status | `error.type`, `rpc.response.status_code`, `tool_error`, span status code | semconv; https://github.com/modelcontextprotocol/python-sdk/issues/3174 |
| Sensitive opt-in present | `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result` | semconv |

Design note: the `_meta` rule only works if the trace file contains the `_meta` values. Spans do not
carry `_meta` by default, so for rule 1 the harness middleware must record them. For third-party
traces, rule 1 can only be inferred from trace-id linkage (HYPOTHESIS). This limits what `doctor`
can say about real-world files and must be stated in the CLI output.

## Risks

1. **pdata dependency weight.** 26 modules (**spike**). pdata is a standalone module and does not
   pull the full Collector (`platforms-and-tooling.md` B.4), but it is heavy for a small CLI and
   tracks Collector release cadence. Mitigation: pin; keep parsing behind one function; see fallback
   A.
2. **fileexporter is alpha.** The README says "no guarantee that exact field names will remain
   stable" (URL above). Mitigation: a small fixture corpus per Collector version; lenient parsing
   (ints or strings for nanos, `kind: 2` or `SPAN_KIND_SERVER`, empty `parentSpanId` as root).
3. **Semconv is Development.** Every MCP span, metric and attribute is Development; the repo moved
   once already, and #437 says it lags the 2026-07-28 protocol
   (https://github.com/open-telemetry/semantic-conventions-genai/issues/437). Mitigation: rules cite
   the semconv commit they were written against, carry a stability tag, and can be disabled by id; a
   `--semconv` profile switch if names break.
4. **go-sdk `GetID`.** `Request.GetID()` is on main (issue #1264 closed 2026-10-28:
   https://github.com/modelcontextprotocol/go-sdk/issues/1264). HYPOTHESIS: not in v1.8.0. Without
   it, harness middleware cannot read the JSON-RPC id, so `jsonrpc.request.id` cannot be set and
   client/server pairing by id fails. Mitigation: verify with `git tag --contains`; until then pair
   by trace id plus method and tool name plus time overlap (weaker; HYPOTHESIS), or build the
   harness against main.
5. **Protocol mode drift.** 2026-07-28 removed sessions and `initialize`; the semconv still
   references `mcp.session.id` and session metrics (`mcp-ecosystem.md` 1.1 and 4.4). Rules must not
   require session attributes.
6. **Double spans and SDK-native tracing.** Where an SDK already traces (Python v2), the semconv
   says to add attributes to the existing span, not create another (`mcp-ecosystem.md` 4.5). Rules
   must tolerate a single span with CLIENT/SERVER kind plus `execute_tool` attributes.
7. **Demand risk.** Moderate, from maintainers only (`00-project-vision.md`). The experiment is
   deliberately small so this risk is cheap to test.
8. **Large files.** One line per batch, files rotate at 100 MB by default (fileexporter README).
   In-memory merging may not scale. HYPOTHESIS: acceptable for test traces; streaming per-trace is a
   later concern.
9. **Compressed input.** zstd output without `nativeCompression` is per-batch and not readable by
   `zstd -d` (fileexporter README). v0 reads uncompressed JSON only and says so.

## Fallback plans

- **A. Replace pdata.** Hand-roll decoding with `encoding/json` into small structs (the research
  recommends this for Jaeger and Zipkin anyway). Cost: write hex, int-or-string, kind-enum handling
  ourselves. Trigger: pdata dependency becomes a packaging problem.
- **B. `go.opentelemetry.io/proto/otlp` + protojson.** Lighter, but stock protojson expects base64
  for `bytes`; OTLP uses hex, so a pre-pass is needed (`platforms-and-tooling.md` B.4, HYPOTHESIS
  untested).
- **C. Defer to weaver for attributes.** Emit weaver sample JSON or replay through a Collector into
  weaver's gRPC listener. Needs a Collector binary, which defeats the single-binary story
  (`platforms-and-tooling.md` B.2, HYPOTHESIS). Use only if attribute rules prove costly to
  maintain.
- **D. Shrink to the harness.** If no one wants `doctor`, publish the scenarios and fixture traces
  as test material for SDK maintainers, and keep the upstream PRs (conformance scenario, mcp-otel-go
  inject, openllmetry #4374).
- **E. Stop.** If Go SDK v2, TS PR #3115 and others converge and the failures disappear, archive
  with a note. Upstream PRs retain value.

## Licence check

Apache-2.0 chosen. Reasons: explicit patent grant, the same licence as OpenTelemetry and weaver
(https://github.com/open-telemetry/weaver, Apache-2.0) and the MCP SDKs' direction (conformance repo
carries an MIT to Apache-2.0 transition notice:
https://github.com/modelcontextprotocol/conformance); friendlier to corporate contributors; MIT is
shorter but has no patent grant. All known dependencies (pdata and `go.opentelemetry.io/proto/otlp`
Apache-2.0, go-sdk, mcp-otel-go MIT) are compatible. The licence of pdata itself is not stated in
the research files (CONFIRMED 2026-10-30: the pdata module LICENSE file in the Go module cache is Apache-2.0). Re-check with `go-licenses` before
release.

## PoC results (2026-10-30)

Implemented and executed locally (Go 1.27.3, go-sdk v1.8.0, pdata v1.70.0,
otel/opentelemetry-collector-contrib 0.146.0 via Docker):

| Fixture (`testdata/`)      | Defect injected                              | Rules fired                | Expected |
|----------------------------|----------------------------------------------|----------------------------|----------|
| healthy.jsonl              | none                                         | none                       | yes      |
| broken-propagation.jsonl   | client skips `_meta` inject                  | MCP-PROP-001 x6            | yes      |
| broken-error-type.jsonl    | server omits `error.type`, status Unset      | MCP-ERR-001, MCP-ERR-003   | yes      |
| leaked-arguments.jsonl     | server records `gen_ai.tool.call.arguments`  | MCP-PRIV-001 x3            | yes      |
| missing-method-attr.jsonl  | server omits `mcp.method.name`               | MCP-SEM-001 x5 (FAIL)      | yes      |

Each fixture holds 13 spans (initialize, notifications/initialized,
server/discover, three tools/call pairs, one client root span). Checks run:
gofmt, go vet, go test -race, govulncheck (no reachable vulnerabilities),
staticcheck (clean).

Lessons that changed the code:

- The Go SDK hands the server `*mcp.CallToolParamsRaw`, not `*mcp.CallToolParams` (our hand-rolled harness missed this; mcp-otel-go handles both, verified 2026-10-30 in `mcpotel/spans.go:66-74`).
  Middleware that type-asserts only the client type silently loses the tool name.
  The doctor caught this in the harness itself (MCP-SEM-002 on the healthy run).
- A tool `isError` result with no `error.type` and status Unset is invisible from
  the server span alone. Pairing the server span with its client parent
  (MCP-ERR-003) recovers it when propagation works. When propagation is also
  broken, the defect cannot be detected from spans; a wire capture would be needed.
- `go run` does not forward SIGINT, and the Collector fileexporter keeps the output
  file open, so fixture generation restarts the collector per scenario.
- `leaked-arguments.jsonl` intentionally contains the string `secret-token-123`
  as a demo payload. Nothing in fixtures is a real credential.
