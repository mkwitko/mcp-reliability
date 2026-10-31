"""MCP Python SDK v2 client: one "agent step" root span, three tool calls.

No MCP instrumentation is added here; whatever spans and `_meta` injection
appear come from the SDK itself (mcp/shared/jsonrpc_dispatcher.py).

    uv run client.py [--url http://127.0.0.1:8765/]
"""

import argparse
import asyncio

from mcp.client import Client
from mcp.shared.exceptions import MCPError
from opentelemetry import trace

import otel_setup

p = argparse.ArgumentParser()
p.add_argument("--url", default="http://127.0.0.1:8765/")
args = p.parse_args()


async def main() -> None:
    tracer = trace.get_tracer("example/mcp-client-py")
    async with Client(args.url) as c:
        # One root span per logical agent step so client spans are not themselves roots.
        with tracer.start_as_current_span("agent step"):
            for name in ("echo", "fail", "missing"):
                try:
                    res = await c.call_tool(name, {"text": "hello secret-token-123"})
                    print(f"{name}: {'tool error' if res.is_error else 'ok'}")
                except MCPError as e:
                    print(f"{name}: rpc error: {e}")


tp = otel_setup.setup("example-mcp-client-py")
try:
    asyncio.run(main())
finally:
    tp.shutdown()
