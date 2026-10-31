# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/), but nothing is stable before v0.1.

## [Unreleased]

## [0.0.1] - 2026-10-30

### Added
- Project scaffolding: docs, license, CI, issue and PR templates.
- `doctor` command: reads newline-delimited OTLP JSON, prints a text report or `-json`, exit codes for CI, `-strict`.
- eight rules: MCP-SEM-001/002, MCP-ERR-001/002/003, MCP-PROP-001/002, MCP-PRIV-001.
- Go SDK Streamable HTTP scenario harness (`examples/go-sdk-streamable-http`) with failure flags, and a collector config in `examples/collector`.
- Five fixtures in `testdata/` captured from the harness, pinned by `TestFixtures`.
- `Makefile` (`check`, `fixtures`), architecture doc, ADR-0005, engineering practices doc.
