// Client on the typescript-sdk#3115 build (v2 API): one "agent step" root span,
// three tool calls. No MCP instrumentation here.
//   node client.mjs [--url http://127.0.0.1:8765/] [--legacy]
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";
import { trace } from "@opentelemetry/api";
import { setup } from "./otel_setup.mjs";

const arg = (k, d) => { const i = process.argv.indexOf(k); return i > 0 ? process.argv[i + 1] : d; };
const url = arg("--url", "http://127.0.0.1:8765/");
const legacy = process.argv.includes("--legacy");
const sdk = setup("example-mcp-client-ts");

const c = new Client({ name: "example-client-ts", version: "1.0.0" }, { versionNegotiation: { mode: legacy ? "legacy" : "auto" } });
await c.connect(new StreamableHTTPClientTransport(new URL(url)));
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
