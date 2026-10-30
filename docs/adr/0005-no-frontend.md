# 0005: No web frontend

Status: accepted, 2026-10-30

## Context
The kickoff document listed a Vue or React UI as a "potential later" item. The maintainer then asked about a stack of Vite, Tailwind, react-query and zod. The product is a trace linter. Its users already run an observability backend (Jaeger, Tempo, Langfuse, and similar) that renders traces. What they lack is a verdict on whether their MCP spans are correct, which fits a CLI and a CI gate.

## Decision
No web frontend. The interface is the `doctor` CLI, `-json` output, and exit codes for CI.

Revisit only as a static, self-contained HTML report (for example `doctor -html`) written to a file, if users ask for it. Never a server, SPA, or hosted service.

## Consequences
- No JS toolchain, no npm supply chain, no frontend CVE surface.
- CI integration works with exit codes and JSON.
- Less visual appeal for demos; the text report must carry the message.
- A static HTML report, if built, needs no new runtime and can use `html/template` from the stdlib.
- Anyone wanting a UI can consume `-json`.

## Alternatives
- Vite + React SPA with a local server: rejected; adds a server, a toolchain, and a stack the project does not need.
- Hosted dashboard: rejected by ADR-0002 and ADR-0004 (no backend, no SaaS).
- Plugin for an existing backend UI: deferred; needs demand and a stable finding schema first.
