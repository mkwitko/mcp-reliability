// MCP TypeScript SDK 1.32.2 server over stateless Streamable HTTP, no MCP
// instrumentation. Tools mirror the Python example: `echo` and `fail` (isError).
//   node server.mjs [--port 8765]
import http from "node:http";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import * as z from "zod";
import { setup } from "./otel_setup.mjs";

const i = process.argv.indexOf("--port");
const port = i > 0 ? Number(process.argv[i + 1]) : 8765;
const sdk = setup("example-mcp-server-ts");

function build() {
  const s = new McpServer({ name: "example-server-ts", version: "1.0.0" });
  s.registerTool("echo", { description: "echo text", inputSchema: { text: z.string() } }, async ({ text }) => ({
    content: [{ type: "text", text }],
  }));
  s.registerTool("fail", { description: "always fails as a tool error", inputSchema: { text: z.string() } }, async () => ({
    content: [{ type: "text", text: "boom" }],
    isError: true,
  }));
  return s;
}

// Any path is served so the Go example client (Endpoint "http://host:port") can connect too.
const srv = http.createServer(async (req, res) => {
  const chunks = [];
  for await (const c of req) chunks.push(c);
  const raw = Buffer.concat(chunks).toString();
  const mcp = build();
  const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined });
  res.on("close", () => { transport.close(); mcp.close(); });
  await mcp.connect(transport);
  await transport.handleRequest(req, res, raw ? JSON.parse(raw) : undefined);
});
srv.listen(port, "127.0.0.1", () => console.error(`listening on http://127.0.0.1:${port}/`));

for (const sig of ["SIGINT", "SIGTERM"]) {
  process.on(sig, async () => { srv.close(); await sdk.shutdown(); process.exit(0); });
}
