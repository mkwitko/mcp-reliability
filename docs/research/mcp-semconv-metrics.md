# MCP semconv metrics (extract)

Source: https://raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/main/docs/gen-ai/mcp.md, section "Metrics". Fetched and verified 2026-11-03 (`main` branch, no pinned commit; the page cites semantic-conventions v1.45.0 for shared attributes). Status of everything below: Development.

## Metrics defined

Four Histograms, unit (UCUM) `s`, metric requirement level "Recommended":

| Name | Description (verbatim) |
|---|---|
| `mcp.client.operation.duration` | "The duration of the MCP request or notification as observed on the sender from the time it was sent until the response or ack is received." |
| `mcp.server.operation.duration` | "MCP request or notification duration as observed on the receiver from the time it was received until the result or ack is sent." |
| `mcp.client.session.duration` | "The duration of the MCP session as observed on the MCP client." |
| `mcp.server.session.duration` | "The duration of the MCP session as observed on the MCP server." |

Each: "This metric SHOULD be specified with `ExplicitBucketBoundaries` of `[ 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300 ]`."

## Attribute requirement levels (operation metrics, identical on client and server)

| Key | Level |
|---|---|
| `mcp.method.name` | `Required` |
| `error.type` | `Conditionally Required` If and only if the operation fails. |
| `gen_ai.prompt.name` | `Conditionally Required` When operation is related to a specific prompt. |
| `gen_ai.tool.name` | `Conditionally Required` When operation is related to a specific tool. |
| `rpc.response.status_code` | `Conditionally Required` If response contains an error code. Marked Deprecated, "Replaced by `rpc.status_code`". |
| `gen_ai.operation.name`, `mcp.protocol.version`, `network.transport`, ... | `Recommended` |
| `gen_ai.prompt.variable`, `mcp.resource.uri` | `Opt-In` |

Session metrics: `error.type` is `Conditionally Required` "If and only if session ends with an error."; everything else `Recommended`. They have no `Required` attribute.

## Surprises

- Four metrics, not two. The session metrics have no Required attribute, so MCP-MET-001 covers only the operation metrics.
- `rpc.response.status_code` is deprecated in the metric tables in favour of `rpc.status_code`, while the span tables used by the other rules still use the old key. The doctor does not check either key on metrics.
- The metric requirement level is Recommended, so an absent metric is never a violation (MCP-MET-003 is INFO).
