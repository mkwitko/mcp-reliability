"""MCP Python SDK v2 server over Streamable HTTP with the SDK's default OTel.

Tools mirror examples/go-sdk-streamable-http: `echo` and `fail` (isError).
No MCP instrumentation is added here; the SDK's OpenTelemetryMiddleware is
installed by default on every lowlevel Server (mcp/server/lowlevel/server.py).

    uv run server.py [--port 8765]
"""

import argparse

from mcp.server.mcpserver import MCPServer
from mcp_types import CallToolResult, TextContent

import otel_setup

p = argparse.ArgumentParser()
p.add_argument("--host", default="127.0.0.1")
p.add_argument("--port", type=int, default=8765)
args = p.parse_args()

tp = otel_setup.setup("example-mcp-server-py")
mcp = MCPServer("example-server-py")


@mcp.tool(description="echo text")
def echo(text: str) -> str:
    return text


@mcp.tool(description="always fails as a tool error")
def fail(text: str) -> CallToolResult:
    return CallToolResult(content=[TextContent(type="text", text="boom")], is_error=True)


try:
    # Path "/" so the Go example client (Endpoint "http://host:port") can connect too.
    mcp.run("streamable-http", host=args.host, port=args.port, streamable_http_path="/")
finally:
    tp.shutdown()
