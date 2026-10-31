"""MCP Python SDK 1.x client, instrumented only by openllmetry's MCPInstrumentor.

One "agent step" root span, then tools/call for echo, fail, missing.

    uv run client.py [--url URL] [--meta] [--legacy-transport]

--meta              pass meta={} to call_tool, so params._meta is non-None
                    (the case openllmetry's send_request patch handles).
--legacy-transport  use the deprecated streamablehttp_client, the only
                    Streamable HTTP client factory openllmetry 0.62.4 wraps.
"""

import argparse
import asyncio

import otel_setup

tp = otel_setup.setup("example-mcp-client-openllmetry")

from mcp import ClientSession  # noqa: E402  (after instrument())
from mcp.client import streamable_http  # noqa: E402
from mcp.shared.exceptions import McpError  # noqa: E402
from opentelemetry import trace  # noqa: E402

p = argparse.ArgumentParser()
p.add_argument("--url", default="http://127.0.0.1:8766/")
p.add_argument("--meta", action="store_true")
p.add_argument("--legacy-transport", action="store_true")
args = p.parse_args()


async def main() -> None:
    tracer = trace.get_tracer("example/mcp-client-openllmetry")
    # Attribute lookup at call time so the instrumentor's wrapper is used.
    connect = streamable_http.streamablehttp_client if args.legacy_transport else streamable_http.streamable_http_client
    async with connect(args.url) as (read, write, _), ClientSession(read, write) as s:
        await s.initialize()
        with tracer.start_as_current_span("agent step"):
            for name in ("echo", "fail", "missing"):
                kw = {"meta": {}} if args.meta else {}
                try:
                    res = await s.call_tool(name, {"text": "hello secret-token-123"}, **kw)
                    print(f"{name}: {'tool error' if res.isError else 'ok'}")
                except McpError as e:
                    print(f"{name}: rpc error: {e}")


try:
    asyncio.run(main())
finally:
    tp.shutdown()
