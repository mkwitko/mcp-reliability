// Server on the typescript-sdk#3115 build (v2 API). No MCP instrumentation here:
// registering an OTel SDK is all the PR asks for (docs/advanced/opentelemetry.md).
//   node server.mjs [--port 8765]
import { serve } from "@hono/node-server";
import { createMcpHandler, McpServer } from "@modelcontextprotocol/server";
import * as z from "zod/v4";
import { setup } from "./otel_setup.mjs";

const i = process.argv.indexOf("--port");
const port = i > 0 ? Number(process.argv[i + 1]) : 8765;
const sdk = setup("example-mcp-server-ts");

function build() {
  const s = new McpServer({ name: "example-server-ts", version: "1.0.0" });
  s.registerTool("echo", { description: "echo text", inputSchema: z.object({ text: z.string() }) }, async ({ text }) => ({
    content: [{ type: "text", text }],
  }));
  s.registerTool("fail", { description: "always fails as a tool error", inputSchema: z.object({ text: z.string() }) }, async () => ({
    content: [{ type: "text", text: "boom" }],
    isError: true,
  }));
  return s;
}

const handler = createMcpHandler(build);
// Any path is served so the Go example client (Endpoint "http://host:port") can connect too.
const srv = serve({ fetch: (req) => handler.fetch(req), port, hostname: "127.0.0.1" }, () =>
  console.error(`listening on http://127.0.0.1:${port}/`));

for (const sig of ["SIGINT", "SIGTERM"]) {
  process.on(sig, async () => { srv.close(); await sdk.shutdown(); process.exit(0); });
}
