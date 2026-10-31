"""MCP Python SDK 1.x FastMCP server over Streamable HTTP, instrumented only
by openllmetry's MCPInstrumentor (mcp 1.x has no built-in OTel).

Tools: `echo` and `fail` (isError), same surface as the other examples.

    uv run server.py [--port 8766]
"""

import argparse

import otel_setup

tp = otel_setup.setup("example-mcp-server-openllmetry")

from mcp.server.fastmcp import FastMCP  # noqa: E402  (after instrument())
from mcp.types import CallToolResult, TextContent  # noqa: E402

p = argparse.ArgumentParser()
p.add_argument("--host", default="127.0.0.1")
p.add_argument("--port", type=int, default=8766)
args = p.parse_args()

mcp = FastMCP("example-server-openllmetry", host=args.host, port=args.port, streamable_http_path="/")


@mcp.tool(description="echo text")
def echo(text: str) -> str:
    return text


@mcp.tool(description="always fails as a tool error")
def fail(text: str) -> CallToolResult:
    return CallToolResult(content=[TextContent(type="text", text="boom")], isError=True)


try:
    mcp.run("streamable-http")
finally:
    tp.shutdown()
