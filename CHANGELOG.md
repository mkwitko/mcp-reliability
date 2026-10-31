# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/), but nothing is stable before v0.1.

## [Unreleased]

- Rule MCP-COV-001 (INFO): file has spans but none looks like MCP, so "no findings" is not a pass.
- docs/11-code-tour.md: file-by-file walk of one `doctor` run for newcomers.

### Added
- `doctor -wire <capture.jsonl>`: wire-format checks MCP-PROP-003 (SEP-414 `_meta.traceparent` MUST be W3C formatted, FAIL) and MCP-PROP-004 (no traceparent on the wire, INFO). Harness flag `-capture` writes the capture; `-bad-traceparent` client flag and `bad-traceparent` fixture.
- `doctor` accepts several trace files and `-` for stdin.
- `make report` renders `docs/compatibility-report.md` from the fixtures (M2 exit criterion).
- `doctor -weaver-json out.json` exports spans in OTel Weaver live-check sample format; `examples/weaver/` has the verified command and matchers (M4).
- Upstream drafts for python-sdk (`docs/upstream/python-sdk.md`), mcp-otel-go, and the conformance suite.
- Rules MCP-ERR-004 (status `Ok` on success, FAIL per Recording Errors MUST) and MCP-ERR-005 (caller-error JSON-RPC codes recorded as failures, WARN per mcp.md footnote 5). Both found real cases: our own harness marked unknown tools as errors, and so does the Python SDK 2.3.0 client.
- Rules MCP-PROP-006 (no pairable CLIENT/SERVER spans, INFO) and MCP-PRIV-002 (vendor payload attributes, INFO, project heuristic). `looksMCP` recognises openllmetry's `{tool}.tool` / `{method}.mcp` spans.
- openllmetry scenario (`examples/openllmetry-python`, `opentelemetry-instrumentation-mcp` 0.62.4 with mcp 1.30.0) with three fixtures: defaults (propagation lost on every request, #4374 reproduced), `meta={}` workaround, deprecated `streamablehttp_client` transport (propagates). Report: `docs/research/openllmetry-validation.md`.
- Rules MCP-ERR-006 (Go type name in `error.type`, WARN) and MCP-PROP-005 (root server spans with no MCP client span in a multi-service file, INFO), both from the mcp-otel-go v0.2.1 validation.
- mcp-otel-go scenario (`examples/mcp-otel-go`, standalone module) with two fixtures: library defaults (propagation lost: six root server spans in six traces, status Ok on success) and the proposed client-inject fix (linked). Report: `docs/research/mcp-otel-go-validation.md`.
- Rules MCP-SEM-003 (span name format, WARN) and MCP-SEM-004 (missing `gen_ai.operation.name`, INFO), both true positives against the official Python SDK 2.3.0 client.
- Python SDK v2 scenario (`examples/python-sdk-streamable-http`, uv) and three fixtures: Python default, Go client to Python server, Python client to Go server. Report in `docs/research/python-sdk-v2-validation.md`.
- TypeScript SDK scenario (`examples/typescript-sdk-streamable-http`, npm): baseline `@modelcontextprotocol/sdk` 1.32.2 (no MCP spans, no `_meta.traceparent`) and the open typescript-sdk#3115 build (built-in spans, propagates), four fixtures incl. two cross-SDK. Report in `docs/research/typescript-sdk-validation.md`.

### Changed
- MCP-PROP-001 pairs client and server spans by `jsonrpc.request.id` when present, falling back to tool name. The Python client sets no tool name, which hid a broken-propagation case.

### Fixed
- Go harness marked `-32602` (unknown tool) as a span failure; semconv says caller-error codes SHOULD NOT be errors. MCP-ERR-001 no longer treats those codes as failures either.
- Go harness panicked on a typed-nil params pointer for `notifications/initialized` sent without params (go-sdk v1.8.0 behaviour).
- CLI tests (`cmd/mcp-reliability/main_test.go`); fixture tests now pin severity and finding counts per rule.
- goreleaser config and tag-triggered release workflow (draft releases with checksums and SBOM).
- GitHub labels, Discussions enabled.

## [0.0.1] - 2026-10-30

### Added
- Project scaffolding: docs, license, CI, issue and PR templates.
- `doctor` command: reads newline-delimited OTLP JSON, prints a text report or `-json`, exit codes for CI, `-strict`.
- eight rules: MCP-SEM-001/002, MCP-ERR-001/002/003, MCP-PROP-001/002, MCP-PRIV-001.
- Go SDK Streamable HTTP scenario harness (`examples/go-sdk-streamable-http`) with failure flags, and a collector config in `examples/collector`.
- Five fixtures in `testdata/` captured from the harness, pinned by `TestFixtures`.
- `Makefile` (`check`, `fixtures`), architecture doc, ADR-0005, engineering practices doc.
