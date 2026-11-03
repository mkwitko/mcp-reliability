# Compatibility report

Generated 2026-11-03 by `make report` from the fixtures in `testdata/`. Every row is
reproducible: the producer is in `examples/`, the capture script is next to it,
and the counts come from `mcp-reliability doctor -json`. Do not edit by hand.

Severity meaning: FAIL = spec MUST or semconv Required violated; WARN = SHOULD,
Conditionally Required, or sensitive data present; INFO = ambiguous evidence.
Rule texts and citations: `docs/06-architecture.md`, section 6.

| Fixture | Producer | Shows | FAIL | WARN | INFO | Rules fired |
|---|---|---|---|---|---|---|
| `bad-traceparent` | go-sdk harness | client sends malformed _meta.traceparent (needs -wire) | 6 | 6 | 0 | MCP-PROP-001 x6, MCP-PROP-003 x6 |
| `broken-error-type` | go-sdk harness | server omits error.type on isError | 0 | 1 | 0 | MCP-ERR-003 x1 |
| `broken-propagation-stdio` | go-sdk harness over stdio | client skips _meta inject | 0 | 4 | 1 | MCP-PROP-001 x4, MCP-PROP-004 x1 |
| `broken-propagation` | go-sdk harness | client skips _meta inject | 0 | 6 | 1 | MCP-PROP-001 x6, MCP-PROP-004 x1 |
| `cross-go-client-python-server` | go-sdk harness client -> mcp 2.3.0 server | cross-SDK propagation | 0 | 0 | 1 | MCP-MET-003 x1 |
| `cross-go-client-typescript-server` | go-sdk harness client -> @modelcontextprotocol/sdk 1.32.2 server | cross-SDK propagation | 0 | 0 | 1 | MCP-MET-003 x1 |
| `cross-python-client-go-server` | mcp 2.3.0 client -> go-sdk harness server | cross-SDK propagation | 0 | 12 | 4 | MCP-ERR-003 x2, MCP-ERR-005 x1, MCP-MET-003 x1, MCP-SEM-002 x3, MCP-SEM-003 x6, MCP-SEM-004 x3 |
| `cross-typescript-client-go-server` | @modelcontextprotocol/sdk 1.32.2 client -> go-sdk harness server | cross-SDK propagation | 0 | 0 | 2 | MCP-MET-003 x1, MCP-PROP-005 x1 |
| `healthy-stdio` | go-sdk v1.8.0 client spawning the server over stdio | baseline over stdio | 0 | 0 | 0 | none |
| `healthy` | go-sdk v1.8.0 client+server, hand-rolled middleware | baseline, no injected defect | 0 | 0 | 0 | none |
| `leaked-arguments` | go-sdk harness | server records gen_ai.tool.call.arguments | 0 | 3 | 0 | MCP-PRIV-001 x3 |
| `mcp-otel-go-client-inject` | mcp-otel-go server, client with inject middleware | proposed upstream fix | 4 | 1 | 4 | MCP-ERR-004 x4, MCP-ERR-006 x1, MCP-MET-003 x1, MCP-SEM-004 x3 |
| `mcp-otel-go-default` | mcp-otel-go server, plain go-sdk client | library defaults | 4 | 1 | 5 | MCP-ERR-004 x4, MCP-ERR-006 x1, MCP-MET-003 x1, MCP-PROP-005 x1, MCP-SEM-004 x3 |
| `missing-method-attr` | go-sdk harness | server omits mcp.method.name | 6 | 0 | 0 | MCP-MET-001 x1, MCP-SEM-001 x5 |
| `openllmetry-default` | opentelemetry-instrumentation-mcp (traceloop) client+server | library defaults | 13 | 2 | 17 | MCP-ERR-001 x2, MCP-ERR-004 x3, MCP-MET-003 x1, MCP-PRIV-002 x15, MCP-PROP-006 x1, MCP-SEM-001 x10 |
| `openllmetry-legacy-transport` | opentelemetry-instrumentation-mcp 0.62.4, legacy SSE transport | Python SDK mcp 1.30.0 | 18 | 2 | 17 | MCP-ERR-001 x2, MCP-ERR-004 x3, MCP-MET-003 x1, MCP-PRIV-002 x15, MCP-PROP-006 x1, MCP-SEM-001 x15 |
| `openllmetry-with-meta` | opentelemetry-instrumentation-mcp client with non-None meta | #4374 workaround | 13 | 2 | 17 | MCP-ERR-001 x2, MCP-ERR-004 x3, MCP-MET-003 x1, MCP-PRIV-002 x15, MCP-PROP-006 x1, MCP-SEM-001 x10 |
| `python-sdk-default` | mcp 2.3.0 (Python) client+server, built-in OTel | SDK defaults | 0 | 10 | 4 | MCP-ERR-003 x2, MCP-MET-003 x1, MCP-SEM-002 x3, MCP-SEM-003 x5, MCP-SEM-004 x3 |
| `typescript-sdk-default` | @modelcontextprotocol/sdk 1.32.2 (TypeScript) client+server, no MCP instrumentation | SDK defaults | 0 | 0 | 1 | MCP-COV-001 x1 |
| `typescript-sdk-pr3115` | typescript-sdk#3115 build (2.4.0 + built-in OTel) client+server | open PR, not released | 0 | 2 | 1 | MCP-ERR-003 x1, MCP-ERR-005 x1, MCP-MET-003 x1 |

Reading the table:

- Rows from the go-sdk harness with an injected defect are self-tests of the
  rules, not statements about the Go SDK.
- Rows named after a library or SDK describe that library's default output on
  the date above, with the version pinned in its `examples/` directory.
- A WARN against an official SDK is a conversation to have upstream, not a
  bug in the SDK by itself: most WARN rules enforce SHOULD-level text in a
  Development-status convention. Drafts live in `docs/upstream/`.
