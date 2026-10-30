# Security Policy

Status: pre-alpha. No supported releases yet; fixes land on `main`.

## Reporting a vulnerability

Use GitHub private vulnerability reporting: the repository's **Security** tab, then **Report a vulnerability** (GitHub security advisories). Do not open a public issue. Expect an acknowledgement within a best-effort few days; this is a single-maintainer project with no SLA.

## Scope

In scope: the `mcp-reliability` CLI (parsing of untrusted trace files, report output) and the code under `examples/`.

Out of scope: vulnerabilities in the MCP SDKs, OpenTelemetry Collector, or other third-party software; report those upstream.

## Telemetry may contain secrets

Traces can contain tool arguments, tool results, tokens, and personal data (`gen_ai.tool.call.arguments` and `.result` are opt-in because they are sensitive). Before attaching a trace to an issue or advisory, redact it. The CLI runs locally, makes no network calls, and redacts attribute values in its output by default, but trace files themselves are not sanitized by this project.
