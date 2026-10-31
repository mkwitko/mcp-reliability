# 11. Code tour

A guided read of the codebase for someone new to Go, OpenTelemetry, or MCP. It follows one `doctor` run from the command line to the report and names the Go idioms each file uses. Written 2026-10-31; keep it in sync with `docs/06-architecture.md`, which stays the source of truth for the rule catalogue.

## 1. Shape of the repo

```
cmd/mcp-reliability/main.go     entry point: flags, file loading, exit code
internal/trace/load.go          OTLP JSON (NDJSON) -> []Span
internal/wire/wire.go           {method,_meta} capture -> []Message
internal/rules/rules.go         the 16 span rules, Run(), Sort()
internal/rules/wire.go          the 2 wire rules (traceparent format)
internal/report/text.go         text and JSON output
internal/report/weaver.go       export for `weaver registry live-check`
examples/                       scenario harnesses that produce testdata/
testdata/*.jsonl                captured fixtures, pinned by tests
```

Go conventions visible here:

- `cmd/<name>` holds a `package main`; one binary per directory.
- `internal/` is enforced by the compiler: only code inside this module can import it. That is the whole public-API story; there is none.
- A package is a directory. File names do not matter to the compiler, only `package x` at the top.
- Dependencies between packages form a DAG: `main -> report -> rules -> trace`. `trace` imports only pdata. Nothing imports `main`.

## 2. One run, end to end

`mcp-reliability doctor -wire capture.jsonl testdata/broken-propagation.jsonl`

1. `main()` calls `run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)` and exits with its return value. `run` takes readers and writers as parameters so tests call it without touching the real process (`main_test.go`).
2. `run` builds a `flag.FlagSet` named `doctor`. Stdlib `flag` parses `-json`, `-strict`, `-wire`, `-weaver-json`. Remaining arguments are files; `-` means stdin.
3. Each file goes through `trace.LoadFile` and the spans are appended into one slice. Loading errors return exit code 2 at once.
4. `rules.Run(spans)` returns `[]Finding`, sorted FAIL first.
5. If `-wire` was given, `wire.LoadFile` and `rules.Wire` add the format findings and the list is sorted again.
6. `report.Text` writes the report and counts severities; the counts choose the exit code: 1 on any FAIL, or on WARN with `-strict`, else 0.

Exit codes are the CLI's contract for CI: 0 pass, 1 findings, 2 usage or parse error.

## 3. internal/trace: the loader

Input is the newline-delimited OTLP JSON that the Collector `fileexporter` writes: one `ExportTraceServiceRequest` per line. The loader does not hand-parse that JSON. It uses `ptrace.JSONUnmarshaler` from the Collector's `pdata` module, so hex ids, enums, and attribute types decode exactly as the Collector encodes them (ADR-0003).

`flatten` walks Resource -> Scope -> Span and copies the few fields the rules need into `trace.Span`: ids, name, kind, status, `service.name` from the resource, and all attributes rendered to strings. The rules never see pdata types. That keeps `rules` free of the large pdata API and makes test spans trivial to build as struct literals.

Idioms:

- `bufio.Scanner` with `sc.Buffer(..., 64<<20)`: the default 64 KB line limit is too small for exporter lines.
- `defer f.Close()` right after a successful `os.Open`.
- Errors wrap context: `fmt.Errorf("line %d: %w", line, err)`. `%w` keeps the original error reachable with `errors.Is`.
- Methods on value receivers (`func (s Span) Attr`) because `Span` is small and never mutated by the rules.

## 4. internal/rules: the checks

`Finding` is a plain struct: rule id, severity, message, evidence span ids, and the URL of the normative text. `Severity` is an `int` with constants from `iota` and a `String()` method; that is how Go spells an enum.

`Run` is a list of function calls, each returning findings for one concern: required attributes, error representation, client/server pair consistency, propagation, sensitive data, coverage. Adding a rule means adding a loop in the right function or a new function and one line in `Run`. No registry, no reflection, no plugin system; the file is the registry.

`looksMCP` decides which spans the rules consider. It accepts semconv attributes, the known MCP method names, and the openllmetry naming shape. Everything else is ignored, which is why a file full of HTTP spans produces no findings.

Each rule comment quotes the text it enforces. Severity follows the text: MUST or Required gives FAIL, SHOULD or Conditionally Required gives WARN, project heuristics give INFO. The test `severityOf` pins every severity so a change fails a test before it reaches a user.

Idioms:

- Maps keyed by small structs (`map[key][]trace.Span` with `type key struct{ method, a string }`) replace a nested map and are comparable for free.
- Comma-ok form everywhere: `v, ok := s.Attr("x")`.
- `sort.SliceStable` so equal severities keep rule order.
- Messages use `%q` for names and never include attribute values (privacy rule in `AGENTS.md`).

`wire.go` is the only place with a regular expression. SEP-414 says `_meta.traceparent` MUST be W3C format when present, which is the only MUST the wire checks can verify.

## 5. internal/report: output

`Text` writes to any `io.Writer` and returns three counts. `JSON` declares a local struct with `json:"..."` tags so field names in the output are stable and lower-case regardless of the Go names. `Weaver` reshapes spans into the sample JSON that `weaver registry live-check` reads.

## 6. Tests

- `rules_test.go` `TestFixtures`: a map from fixture name to expected `{rule: count}`. Every fixture in `testdata/` is listed, including the healthy ones with an empty map. This is the regression net; a rule that starts firing on a healthy fixture fails here.
- `TestSyntheticRules`: hand-built spans for rules no SDK can trigger.
- `main_test.go`: exit codes, stdin, the weaver export.
- `go test -race` runs the race detector; there are no goroutines in the CLI today, but the harnesses have them.

Table-driven tests with `t.Run(name, ...)` are the Go norm. Subtests show up individually in `go test -v` and can be selected with `-run TestFixtures/healthy`.

## 7. The Go SDK harness (`examples/go-sdk-streamable-http`)

One binary, `-role server` or `-role client`, with flags that inject exactly one defect each. The interesting parts for a Go learner:

- `mcp.Middleware` is `func(MethodHandler) MethodHandler`: a function that takes the next handler and returns a wrapped one. The same shape as `net/http` middleware.
- `metaCarrier` adapts `params._meta` (`map[string]any`) to OTel's `TextMapCarrier` interface by defining `Get`, `Set`, `Keys`. Go interfaces are satisfied implicitly; no `implements` keyword.
- `params()` guards against a typed-nil pointer: an interface holding a nil `*InitializedParams` is not `== nil`. The `reflect` check is the standard workaround.
- `toolCall` type-switches on `req.GetParams()` because the client sees `*CallToolParams` and the server sees `*CallToolParamsRaw`.
- `setupOTel` shows the three things every OTel Go program needs: an exporter, a `TracerProvider` with a batcher and a resource, and a global propagator. The propagator is a no-op by default; forgetting it is the most common propagation bug.

## 8. OpenTelemetry vocabulary used by the rules

- Trace: a set of spans sharing a trace id. Span: one timed operation with a parent id, a kind (CLIENT, SERVER, INTERNAL), attributes, and a status (Unset, Ok, Error).
- Propagation: the client injects `traceparent` into the outgoing message, the server extracts it and makes its span a child. For MCP the carrier is `params._meta`, not an HTTP header.
- Semantic conventions: named attributes with requirement levels. The MCP ones live in `semantic-conventions-genai/docs/gen-ai/mcp.md` and are Development stability.
- Collector: a separate process that receives OTLP over HTTP on 4318 and, here, writes NDJSON to a file. The doctor reads that file; it never talks to the network.

## 9. Where to make a change

| Change | Files |
|---|---|
| New rule | `internal/rules/rules.go` (or `wire.go`), fixture in `testdata/`, row in `rules_test.go`, catalogue in `docs/06-architecture.md`, README table, CHANGELOG |
| New input field | `internal/trace/load.go` `Span` and `flatten`; keep it a string if a rule only compares it |
| New output format | `internal/report/`, a flag in `main.go`, a case in `main_test.go` |
| New SDK scenario | `examples/<name>/` with `run.sh` that writes `testdata/<name>.jsonl`, README, research note, test row |

Checklist and commands: `AGENTS.md`.
