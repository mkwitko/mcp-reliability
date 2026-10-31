# Compatibility report

Generated 2026-10-30 by `make report` from the fixtures in `testdata/`. Every row is
reproducible: the producer is in `examples/`, the capture script is next to it,
and the counts come from `mcp-reliability doctor -json`. Do not edit by hand.

Severity meaning: FAIL = spec MUST or semconv Required violated; WARN = SHOULD,
Conditionally Required, or sensitive data present; INFO = ambiguous evidence.
Rule texts and citations: `docs/06-architecture.md`, section 6.

| Fixture | Producer | Shows | FAIL | WARN | INFO | Rules fired |
|---|---|---|---|---|---|---|
| `bad-traceparent` | go-sdk harness | client sends malformed _meta.traceparent (needs -wire) | 6 | 6 | 0 | MCP-PROP-001 x6, MCP-PROP-003 x6 |
| `broken-error-type` | go-sdk harness | server omits error.type on isError | 0 | 1 | 0 | MCP-ERR-003 x1 |
| `broken-propagation` | go-sdk harness | client skips _meta inject | 0 | 6 | 1 | MCP-PROP-001 x6, MCP-PROP-004 x1 |
| `cross-go-client-python-server` | go-sdk harness client -> mcp 2.3.0 server | cross-SDK propagation | 0 | 0 | 0 | none |
| `cross-python-client-go-server` | mcp 2.3.0 client -> go-sdk harness server | cross-SDK propagation | 0 | 12 | 3 | MCP-ERR-003 x2, MCP-ERR-005 x1, MCP-SEM-002 x3, MCP-SEM-003 x6, MCP-SEM-004 x3 |
| `healthy` | go-sdk v1.8.0 client+server, hand-rolled middleware | baseline, no injected defect | 0 | 0 | 0 | none |
| `leaked-arguments` | go-sdk harness | server records gen_ai.tool.call.arguments | 0 | 3 | 0 | MCP-PRIV-001 x3 |
| `missing-method-attr` | go-sdk harness | server omits mcp.method.name | 5 | 0 | 0 | MCP-SEM-001 x5 |
| `python-sdk-default` | mcp 2.3.0 (Python) client+server, built-in OTel | SDK defaults | 0 | 10 | 3 | MCP-ERR-003 x2, MCP-SEM-002 x3, MCP-SEM-003 x5, MCP-SEM-004 x3 |

Reading the table:

- Rows from the go-sdk harness with an injected defect are self-tests of the
  rules, not statements about the Go SDK.
- Rows named after a library or SDK describe that library's default output on
  the date above, with the version pinned in its `examples/` directory.
- A WARN against an official SDK is a conversation to have upstream, not a
  bug in the SDK by itself: most WARN rules enforce SHOULD-level text in a
  Development-status convention. Drafts live in `docs/upstream/`.
