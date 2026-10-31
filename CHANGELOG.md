# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/), but nothing is stable before v0.1.

## [Unreleased]

### Added
- `doctor -wire <capture.jsonl>`: wire-format checks MCP-PROP-003 (SEP-414 `_meta.traceparent` MUST be W3C formatted, FAIL) and MCP-PROP-004 (no traceparent on the wire, INFO). Harness flag `-capture` writes the capture; `-bad-traceparent` client flag and `bad-traceparent` fixture.
- `doctor` accepts several trace files and `-` for stdin.
- `doctor -weaver-json out.json` exports spans in OTel Weaver live-check sample format; `examples/weaver/` has the verified command and matchers (M4).
- Upstream drafts for python-sdk (`docs/upstream/python-sdk.md`), mcp-otel-go, and the conformance suite.
- Rules MCP-ERR-004 (status `Ok` on success, FAIL per Recording Errors MUST) and MCP-ERR-005 (caller-error JSON-RPC codes recorded as failures, WARN per mcp.md footnote 5). Both found real cases: our own harness marked unknown tools as errors, and so does the Python SDK 2.3.0 client.
- Rules MCP-SEM-003 (span name format, WARN) and MCP-SEM-004 (missing `gen_ai.operation.name`, INFO), both true positives against the official Python SDK 2.3.0 client.
- Python SDK v2 scenario (`examples/python-sdk-streamable-http`, uv) and three fixtures: Python default, Go client to Python server, Python client to Go server. Report in `docs/research/python-sdk-v2-validation.md`.

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
