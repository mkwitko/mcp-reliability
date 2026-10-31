// MCP TypeScript SDK 1.32.2 client: one "agent step" root span, three tool calls.
// No MCP instrumentation here.   node client.mjs [--url http://127.0.0.1:8765/]
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { trace } from "@opentelemetry/api";
import { setup } from "./otel_setup.mjs";

const i = process.argv.indexOf("--url");
const url = i > 0 ? process.argv[i + 1] : "http://127.0.0.1:8765/";
const sdk = setup("example-mcp-client-ts");

const c = new Client({ name: "example-client-ts", version: "1.0.0" });
await c.connect(new StreamableHTTPClientTransport(new URL(url)));
// One root span per logical agent step so client spans are not themselves roots.
await trace.getTracer("example/mcp-client-ts").startActiveSpan("agent step", async (span) => {
  for (const name of ["echo", "fail", "missing"]) {
    try {
      const res = await c.callTool({ name, arguments: { text: "hello secret-token-123" } });
      console.log(`${name}: ${res.isError ? "tool error" : "ok"}`);
    } catch (e) {
      console.log(`${name}: rpc error: ${e.message}`);
    }
  }
  span.end();
});
await c.close();
await sdk.shutdown();
