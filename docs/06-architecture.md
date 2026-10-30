# 06. Architecture

Status: describes the code as of 2026-10-30 (M0 done, pre-alpha). Written for AI agents and humans. If the code and this file disagree, the code wins; fix this file.

## 1. Purpose and non-goals

Purpose: read an OpenTelemetry trace file from a system that uses MCP (Model Context Protocol) and report observability defects against published normative text: missing required attributes, lost trace context across the MCP boundary, inconsistent error representation, opt-in payload attributes left on.

Non-goals (see ADR-0002, ADR-0004, ADR-0005):
- Not a tracing backend, dashboard, or web UI.
- No LLM in the core; no network I/O in `doctor`.
- No proprietary trace format; input is OTLP JSON only.
- Does not capture traffic. It analyzes spans somebody already exported.
- Does not judge MCP server correctness, only how it is observed.

## 2. Data flow

```
trace.jsonl --trace.LoadFile/Load--> []trace.Span --rules.Run--> []rules.Finding --report.Text / report.JSON--> stdout
                                                                                  \--> exit code (main.go)
```

1. `cmd/mcp-reliability/main.go` parses `doctor [-json] [-strict] <file>`.
2. `trace.LoadFile` reads newline-delimited OTLP JSON and flattens every span.
3. `rules.Run` runs every rule over the whole span slice and returns findings sorted by severity, highest first (stable sort, so equal severities keep rule order).
4. `report.JSON` (only with `-json`) writes a JSON array to stdout. `report.Text` always runs to count severities; with `-json` its output goes to `io.Discard`.
5. Exit code is decided in `main.go` from the counts.

All state is in function arguments and return values. No globals except compiled regexps and constant tables.

## 3. Package map and exported API

| Path | Responsibility |
|------|----------------|
| `cmd/mcp-reliability` | CLI only: flags, exit codes. No analysis logic. |
| `internal/trace` | OTLP JSON to flat spans. Knows nothing about MCP. |
| `internal/rules` | All checks. Knows MCP and semconv. No I/O. |
| `internal/report` | Rendering findings. No analysis. |
| `examples/go-sdk-streamable-http` | Scenario harness (separate `main`), produces fixtures. Not imported by anything. |
| `examples/collector` | Collector config and compose file used to capture traces. |
| `testdata` | Fixtures: captured traces, treated as golden inputs. |

Exported API today (everything is under `internal/`, so none of it is public API):

```go
// internal/trace
type Span struct {
    TraceID, SpanID string
    ParentID        string            // empty for root spans
    Name            string
    Kind            ptrace.SpanKind
    Status          ptrace.StatusCode
    StatusMsg       string
    Service         string            // resource attribute service.name
    Attrs           map[string]string // span attributes, scalar values rendered via AsString()
    Line            int               // 1-based input line
}
func (s Span) Attr(key string) (string, bool)
func LoadFile(path string) ([]Span, error)
func Load(r io.Reader) ([]Span, error)

// internal/rules
type Severity int // Info=0, Warn=1, Fail=2; String() gives "INFO"/"WARN"/"FAIL"
type Finding struct {
    Rule     string   // MCP-<AREA>-NNN
    Severity Severity
    Message  string
    Spans    []string // span ids (evidence)
    Ref      string   // URL of normative text
}
func Run(spans []trace.Span) []Finding

// internal/report
func Text(w io.Writer, file string, nSpans int, findings []rules.Finding) (fails, warns, infos int)
func JSON(w io.Writer, findings []rules.Finding) error // keys: rule, severity, message, spans, ref
```

Resource attributes other than `service.name` and scope information are dropped by the loader. `Span.Line` is loaded but no rule uses it yet.

## 4. Rule contract

- **ID**: `MCP-<AREA>-NNN`. Areas in use: SEM (semantic conventions), ERR (error representation), PROP (context propagation), PRIV (privacy). IDs are never reused.
- **Severity** follows the cited text, not our opinion:
  - FAIL: spec MUST, or semconv attribute marked Required.
  - WARN: SHOULD, Conditionally Required, or sensitive data present.
  - INFO: ambiguous evidence (the trace may simply be incomplete).
- Every rule cites a `Ref` URL for the text it enforces and says so in the code comment above the function.
- A rule must never report a violation the standard does not require.
- **Evidence** is a list of span ids. A finding with no span evidence is a bug.
- **Privacy**: messages must not echo attribute values. Allowed: rule id, span name, attribute names, trace/span ids, service name. See section 12 for known deviations.
- A rule is a function `func([]trace.Span) []Finding` called from `Run`. Rules are deterministic and side-effect free.

## 5. How MCP spans are recognised

`looksMCP(s)` in `internal/rules/rules.go` returns true if any of:
- an attribute key starts with `mcp.` or `gen_ai.tool.`, or equals `jsonrpc.request.id`;
- the span name starts with a known MCP method (`initialize`, `ping`, `tools/call`, `tools/list`, `resources/*`, `prompts/*`, `completion/complete`, `logging/setLevel`, `sampling/createMessage`, `elicitation/create`, `roots/list`, `notifications/`).

Limits:
- Name matching is a prefix match. A non-MCP span named `ping-db` matches `ping`.
- A span with neither attributes nor a known name is invisible, so a fully uninstrumented MCP hop is not reported.
- The method list is a subset of the semconv enum and is hand-maintained. A method not in it (for example `server/discover`, seen in the fixtures) is only recognised via attributes.
- MCP-PRIV-001 does not use `looksMCP`; it checks every span.

## 6. Rule catalogue

Source URLs: semconv = `https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md`; recording-errors = `https://opentelemetry.io/docs/specs/semconv/general/recording-errors/`; SEP-414 = `https://modelcontextprotocol.io/specification/draft/basic/index#meta`. Semconv for GenAI/MCP is at Development stability.

| ID | Condition | Sev | Normative text (as cited in code) | False-positive risks |
|----|-----------|-----|-----------------------------------|----------------------|
| MCP-SEM-001 | `looksMCP` span without `mcp.method.name` | FAIL | `mcp.method.name` is Required | Heuristic misclassifies a non-MCP span (name prefix) |
| MCP-SEM-002 | `mcp.method.name=tools/call` without `gen_ai.tool.name` | WARN | Conditionally Required when the operation relates to a tool | Harness bug hid this once (CallToolParamsRaw); low otherwise |
| MCP-ERR-001 | span status Error, or `rpc.response.status_code` set and not `OK`, but no `error.type` | WARN | `error.type` Conditionally Required if and only if the operation fails | Any non-`OK` status code counts as failure; SDKs that use other success codes would be flagged |
| MCP-ERR-002 | `error.type` present but status Unset | WARN | recording-errors guidance: failed operations get Error status | Some backends/SDKs set status in a processor after export |
| MCP-ERR-003 | SERVER span whose parent is a CLIENT span with the same `mcp.method.name`; exactly one side has `error.type` | WARN | Same `error.type` text, applied to both sides | Client legitimately sees a transport error the server never saw |
| MCP-PROP-001 | Root SERVER MCP span; a CLIENT span with same method and tool name exists in another trace, none in the same trace | WARN | semconv: instrumentation SHOULD inject/extract context via `params._meta`; SEP-414 reserves the keys | Same-name calls from unrelated clients in the file; matching is by (method, tool) only |
| MCP-PROP-002 | MCP span with a parent id not present in the file | INFO | semconv span model | Parent unsampled, dropped, or exported elsewhere (this is why it is INFO) |
| MCP-PRIV-001 | Any span carries `gen_ai.tool.call.arguments`, `gen_ai.tool.call.result`, `gen_ai.input.messages`, or `gen_ai.output.messages` | WARN | semconv marks these Opt-In and sensitive | Intended in dev/test environments |

Fixture coverage: SEM-001, ERR-001, ERR-003, PROP-001, PRIV-001 have a live-captured fixture. SEM-002, ERR-002, PROP-002 are covered by synthetic spans in `TestSyntheticRules`. A `_meta.traceparent` format check (SEP-414 MUST) is not implemented: spans do not carry the raw `_meta` value under any semconv attribute, so it needs a wire capture (see limitations).

## 7. Input format

Newline-delimited OTLP JSON as written by the Collector `fileexporter` (ADR-0003): one `ExportTraceServiceRequest` per line.

- Parsed with `ptrace.JSONUnmarshaler` from `go.opentelemetry.io/collector/pdata`, not `protojson`. pdata decodes the collector's own encoding exactly (hex trace/span ids, enum names). The cost is a heavy dependency; `protojson` with `go.opentelemetry.io/proto/otlp` is the documented fallback.
- One JSON document per line. Pretty-printed or concatenated JSON is not supported. Blank lines are skipped.
- Scanner buffer: 1 MiB initial, 64 MiB maximum per line. A longer line is a load error.
- A parse error aborts the whole load with `line N: ...` (exit 2). There is no partial result.
- Attribute values are flattened with `AsString()`: non-string scalars become their string form, maps and slices become JSON text.
- Metrics and logs files are not supported.

## 8. Scenario harness

`examples/go-sdk-streamable-http/main.go` is one binary with `-role server|client`, built on the official Go SDK (v1.8.0) over Streamable HTTP, exporting OTLP/HTTP to a local collector. Instrumentation is hand-rolled middleware so no third-party MCP instrumentation is a dependency.

Shapes:
- Client: `AddSendingMiddleware`. Starts a CLIENT span, injects trace context into `params._meta`.
- Server: `AddReceivingMiddleware`. Extracts context from `params._meta`, starts a SERVER span.
- `metaCarrier` adapts `map[string]any` (`_meta`) to the OTel `TextMapCarrier`; non-string values read as empty.
- The global propagator is a no-op until `otel.SetTextMapPropagator(...)` is called. The harness sets it; forgetting it is a common real misconfiguration.

Pitfalls:
- The client sees `*mcp.CallToolParams`, the server sees `*mcp.CallToolParamsRaw`. Asserting only one type loses the tool name on the other side. `toolCall()` handles both.
- go-sdk v1.8.0 exposes no request id getter (`GetID`), so `jsonrpc.request.id` is not recorded.

Failure flags (each introduces one defect):
- `-break-propagation` (client): do not inject `_meta`.
- `-break-error-type` (server): omit `error.type`, leave status Unset.
- `-leak-arguments` (server): record raw arguments as `gen_ai.tool.call.arguments`.
- `-drop-method-attr` (server): omit `mcp.method.name`.

The client calls tools `echo` (ok), `fail` (tool `isError`), and `missing` (JSON-RPC error) under one root span `agent step`.

`examples/run-scenarios.sh` (`make fixtures`, needs Docker):
- Builds the example once and runs the binary. `go run` does not forward SIGINT, so the server would not shut down and flush.
- Restarts the collector per scenario. The fileexporter holds the output file open, so deleting the file underneath it loses data.
- Sleeps for the batch timeout and file flush before copying `examples/collector/out/traces.jsonl` to `testdata/<scenario>.jsonl`.

## 9. Fixtures as contract tests

`internal/rules/rules_test.go` `TestFixtures` loads each `testdata/*.jsonl` and asserts the exact set of rule ids that fire. This catches both a rule that stops firing and one that starts firing too eagerly. Fixtures are captured from real runs, not hand-written, with one exception allowed: a minimal hand-built case for a rule no SDK can trigger.

Current fixtures (13 spans each): `healthy`, `broken-propagation`, `broken-error-type`, `leaked-arguments`, `missing-method-attr`.

## 10. Exit codes

| Code | Meaning |
|------|---------|
| 0 | No FAIL (and no WARN when `-strict`) |
| 1 | At least one FAIL, or a WARN with `-strict` |
| 2 | Usage error, unreadable file, or parse error |

INFO never affects the exit code. With `-json`, stdout is the JSON array only.

## 11. Privacy posture

- `doctor` reads one local file and writes to stdout/stderr. No network.
- Findings carry ids and attribute names, not attribute values (with the exceptions in section 12).
- MCP-PRIV-001 exists because tool arguments and results are the most likely place for secrets.
- `testdata/leaked-arguments.jsonl` contains the demo string `secret-token-123`. It is not a credential.
- Trace files may contain sensitive data. Do not attach unredacted traces to issues.

## 12. Known limitations

- No wire capture. A tool `isError` result with no `error.type`, Unset status, and broken propagation cannot be detected from spans; MCP-ERR-003 only recovers it when client and server are linked.
- go-sdk v1.8.0 has no request id getter; no request-id correlation.
- Semconv for GenAI/MCP is Development stability. Attribute names and requirement levels can change; every rule must be re-verified on a semconv bump.
- `looksMCP` is a heuristic (section 5).
- MCP-PROP-001 matches on (method, tool name) only, so parallel calls to one tool can mask or fake a link.
- Messages echo trace ids, span ids, span names and service names only; no attribute values. Span names may contain a tool name, which is low-cardinality metadata, not payload.
- Only the Go SDK has a scenario. Python and TypeScript are M2.

## 13. Extension points

New rule:
1. One function in `internal/rules/rules.go` (or a new file in the package), called from `Run`.
2. Comment above it: ID, condition, quoted normative text, URL.
3. A fixture in `testdata/` that triggers it (and the healthy fixture must still produce nothing).
4. A row in the `cases` map in `TestFixtures` (or a unit test when no SDK can trigger it).
5. Update the catalogue in this file and the README table; add a CHANGELOG entry.

New SDK scenario: a new directory `examples/<sdk>/` that produces `testdata/<sdk>-<scenario>.jsonl` through the same collector config, wired into `examples/run-scenarios.sh`, plus rows in `TestFixtures`.

## 14. Invariants for AI agents

1. Never report FAIL unless the cited text is a MUST or a Required attribute. Quote the text.
2. Every finding has a `Ref` URL and at least one span id.
3. Findings never contain attribute values beyond what section 12 lists; do not add new echoes.
4. No network calls in `doctor` or any package it imports.
5. No LLM, no telemetry, no background goroutines in core.
6. `trace` knows no MCP; `rules` does no I/O; `report` does no analysis.
7. Output is deterministic: same file, same bytes. Do not iterate over maps into output without sorting.
8. Every new rule ships with a fixture and a test row in the same change.
9. Never put real secrets or customer data in `testdata/`.
10. Do not edit `docs/research/` retroactively; add dated findings.
11. Dependencies are added only with a written reason in the PR.
12. Keep `go.mod` Go version, `Makefile` tool versions, and CI in sync.
