# AGENTS.md

Operating manual for AI coding agents in this repo. Humans: see `CONTRIBUTING.md`.

Project: `mcp-reliability`, a Go CLI (`mcp-reliability doctor`) that checks OpenTelemetry traces of MCP systems against published normative text. Pre-alpha; the PoC works. Do not describe it as more mature than that.

## 1. Read order

1. `README.md`
2. `docs/06-architecture.md` (data flow, API, rule contract, invariants)
3. `docs/11-code-tour.md` (file-by-file walk of one run; names the Go idioms)
4. `docs/adr/` (decisions already made; do not relitigate without a new ADR)
5. `internal/rules/rules.go` (the rules and their cited text)
6. `docs/07-roadmap.md` for scope; `docs/research/` only for sources.

## 2. Commands

```
make check      # fmt vet test lint vuln; this is the definition of green
make test       # go test -race -count=1 ./...
make fixtures   # regenerate the go-sdk testdata/*.jsonl; needs Docker, restarts the collector per scenario
examples/python-sdk-streamable-http/run.sh   # Python SDK 2.3.0 fixtures (uv)
examples/typescript-sdk-streamable-http/run.sh   # TypeScript SDK 1.32.2 baseline fixtures (npm); pr3115 scenario needs pr3115/build-tarballs.sh first
examples/openllmetry-python/run.sh           # openllmetry fixtures (uv), own collector on 4319
examples/mcp-otel-go/run.sh                  # mcp-otel-go fixtures, own collector on 4320
make report     # regenerate docs/compatibility-report.md from testdata/
# Captures sharing a collector port must run one at a time; the compose file
# takes COLLECTOR_PORT / COLLECTOR_OUT and a -p project name for isolation.
go run ./cmd/mcp-reliability doctor testdata/broken-propagation.jsonl
```

On the maintainer's machine Go may live outside PATH:
`export PATH=$PATH:$HOME/.local/go-sdk/go/bin`

`make lint` and `make vuln` download pinned tool versions (Makefile variables), so they need network once. `doctor` itself never uses the network.

## 3. Definition of done

- `gofmt -l .` empty, `go vet ./...`, `go test -race ./...`, staticcheck, govulncheck all green (`make check`). Show the output; do not claim it.
- Every new rule has a fixture and a test row (see section 4).
- Docs updated: `docs/06-architecture.md` catalogue, README rule table, any ADR affected.
- `CHANGELOG.md` has an entry under Unreleased.
- Diff reviewed by you before reporting.

## 4. Rule-authoring checklist

- [ ] Find the normative text (MCP spec or OTel GenAI MCP semconv). Quote it in the code comment with URL and verification date.
- [ ] Severity from the text: FAIL only for MUST / Required. WARN for SHOULD, Conditionally Required, or sensitive data present. INFO for ambiguous evidence.
- [ ] Never report a violation the standard does not require. If you cannot quote it, it is not a FAIL. Mark unverified claims HYPOTHESIS.
- [ ] ID `MCP-<AREA>-NNN`, never reused.
- [ ] Evidence is span ids. The message names attributes, never echoes their values.
- [ ] `Ref` is set.
- [ ] Fixture that triggers it (captured from a real run, or a minimal unit test if no SDK can trigger it).
- [ ] The healthy fixture still produces zero findings.
- [ ] Think through false positives and write them in the architecture catalogue.

## 5. Spec-driven workflow

Do these in order, briefly. Skip none for a new rule or feature.

1. Problem: who is hurt, how.
2. Evidence: a real trace, a spec quote, an issue. Dated source.
3. Requirements: what must be true.
4. Acceptance criteria: observable, testable (fixture + expected rule ids + exit code).
5. Design: smallest change that fits the existing packages.
6. Security and privacy: what could leak, what input could crash the loader.
7. Plan: small ordered steps.
8. Implement.
9. Test.
10. Review your own diff.
11. Docs and changelog.

## 6. Commits

- Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`, `ci:`). Scope optional, for example `feat(rules): add MCP-ERR-004`.
- One logical change per commit: rule + fixture + test row together; docs may be separate.
- Do not commit, push, tag, or open a PR without explicit authorization from the maintainer, each time.
- When Claude co-authors, end the message with:
  `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`
- No sign-off or CLA required.

## 7. Never

- No dashboards, web UI, servers, backends, or SaaS (ADR-0005).
- No LLM or AI calls in core (ADR-0004).
- No network access in `doctor` or its imports.
- No proprietary trace format; input is OTLP JSON (ADR-0003).
- No new dependency without a written justification in the PR. Stdlib first; no interface with one implementation.
- No secrets, tokens, hostnames, or customer data in fixtures. Review fixture payloads before adding.
- No attribute values in finding messages.
- No edits to `docs/research/`; add new dated findings instead.
- No marketing language or claims of maturity in docs.
- Do not hand-edit generated fixtures; regenerate them.

## 8. Citing sources

Every external claim in docs or code comments needs: URL, what it says, and the date you verified it (`verified 2026-10-30`). Quote normative text verbatim and keep it short. If a page changed or you could not fetch it, say so instead of paraphrasing from memory.

## 9. Gotchas

- The collector writes traces and metrics to the same file; `trace.Load` and `metrics.Load` each skip the other's lines. A new signal needs the same skip in both loaders.
- Go harness metrics are exported every 1 s; the harness must call the provider `Shutdown` (it does, via `setupOTel`'s returned func) or the last interval is lost. In stdio mode the server flushes when the client closes stdin.
- Go SDK server side sees `*mcp.CallToolParamsRaw`, client side `*mcp.CallToolParams`.
- `go run` does not forward SIGINT; fixture generation uses a built binary.
- The Collector fileexporter keeps its file open; restart it between scenarios.
- Semconv for GenAI/MCP is Development stability. Re-verify rules when it changes.
