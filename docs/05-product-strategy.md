# Product strategy

Date: 2026-10-30. External claims cite URLs verified in `docs/research/*.md` on 2026-10-30.
HYPOTHESIS marks untested inference.

## What it is

`mcp-reliability` is an open-source (Apache-2.0), local-first Go CLI. `doctor <trace.jsonl>` reads
OTLP JSON from an OTel Collector fileexporter and checks MCP observability contracts:

1. SEP-414 `_meta.traceparent/tracestate/baggage` W3C format.
2. OTel GenAI MCP semconv attribute requirements.
3. Cross-span rules: client and server spans share a trace id; parent span present in the file; no
   orphaned server spans; failed tool calls do not leave status UNSET; sensitive opt-in attributes
   flagged.

It ships with `examples/`, a scenario harness whose `doctor` output doubles as an SDK compatibility
report. Deterministic, no LLM, no network.

## Who benefits

| User | What they get | Evidence of the need (2026-10-30) |
|---|---|---|
| SDK maintainers | A reproducible check for their own instrumentation and a fixture they can attach to bug reports | python-sdk #3174, #3715; typescript-sdk #1231; php-sdk #583 (https://github.com/modelcontextprotocol/python-sdk/issues/3174, https://github.com/modelcontextprotocol/typescript-sdk/issues/1231, https://github.com/modelcontextprotocol/php-sdk/issues/583) |
| MCP server authors | A way to confirm that their traces connect to the caller's and do not leak arguments | Blog posts describe traces stopping at the MCP boundary (SEARCH-SUMMARY, https://dev.to/focused_dot_io/agent-traces-need-to-cross-the-mcp-boundary-focused-labs-5f2l); no issue from an end user found |
| Platform vendors | A neutral definition of "correct MCP telemetry" to test ingest against | HYPOTHESIS. No vendor request found; Sentry, Datadog and Grafana ship their own MCP views (`platforms-and-tooling.md` A.8, A.6) |

The primary audience is the first row. Be explicit about this: the research found no end-user
request for a linter (`platforms-and-tooling.md` C.5).

## Non-goals

- No backend, UI, storage or dashboards.
- No new instrumentation library; contribute upstream instead.
- No server conformance or security scanning (the conformance suite and checkmcp cover those:
  https://github.com/modelcontextprotocol/conformance, https://github.com/gregkozakiewicz/checkmcp).
- No reimplementation of semconv attribute validation if weaver can do it.
- No proxy or traffic rewriting.
- No AI layer for now. Deferred until deterministic rules exist and demand shows; if ever, BYOK,
  structured output, evidence refs.
- No promise of stability for rule ids or output format before the semconv leaves Development.

## Differentiation

- **File input.** Weaver live-check takes an OTLP gRPC listener, text, or its own sample JSON;
  OTLP/HTTP is open issue #1563 (https://github.com/open-telemetry/weaver, 2026-10-30). We read the
  fileexporter format directly.
- **Cross-span checks.** No tool found checks trace topology on MCP calls. Weaver span-type support
  is still being built (#1851, #1859).
- **MCP-specific rules.** `_meta` format, `isError` against span status, both error channels,
  stateless-protocol awareness.
- **Spec-cited, deterministic findings** usable in CI.
- **Compatibility report from real SDKs**, produced by the harness rather than asserted.

Differentiation is narrow and could close: weaver may add span checks, SDKs may fix their bugs, the
semconv may change. The project is small enough to be dropped (see `04-technical-feasibility.md`,
fallback E).

## How it complements existing work

- **Weaver.** Same registry, different input and scope. We may call weaver for attribute validation
  later (fallback C in `04-technical-feasibility.md`). We would also file issues upstream for
  OTLP/HTTP and file input rather than fork the idea.
- **Conformance suite** (https://github.com/modelcontextprotocol/conformance). It checks protocol
  wire traffic; we check telemetry. We propose a scenario for `_meta.traceparent` format that fits
  its wire-capture design (HYPOTHESIS on fit). Our end-to-end propagation rules stay in `doctor`,
  since the harness lacks an OTLP side channel.
- **Backends** (Langfuse, Phoenix, Tempo, Jaeger, Grafana, Sentry, Datadog). They display and store
  traces; we verify the file before or after it reaches them. No competition in intent.
- **Instrumentation libraries.** We send fixes: client-side `Inject` for mcp-otel-go
  (https://github.com/olgasafonova/mcp-otel-go), openllmetry #4374
  (https://github.com/traceloop/openllmetry/issues/4374), and a comment or PR on go-contrib #7616
  (https://github.com/open-telemetry/opentelemetry-go-contrib/issues/7616). `doctor` findings give
  those PRs concrete evidence.
- **SDKs.** Python v2 and TS PR #3115 become scenarios in the harness once stable.

## Sequencing

1. `doctor` v0 with five cross-span rules over go-sdk harness traces.
2. Compatibility report for go-sdk; publish fixtures.
3. Upstream PRs: conformance scenario, mcp-otel-go inject, openllmetry #4374.
4. Python v2 scenario, then TS once #3115 releases.
5. Revisit the AI layer and weaver integration only after items 1-4 and observed demand.

## Success and exit criteria

Success signals: an SDK maintainer cites `doctor` in an issue or CI; a conformance scenario merges;
three SDKs in the compatibility report.

Exit signals: no external use after the first upstream PRs land; weaver ships file input plus span
checks covering the same rules; semconv rewrite invalidates most rules. In any of these cases, fold
into the upstream projects and archive.
