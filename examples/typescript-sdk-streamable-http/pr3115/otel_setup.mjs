// Plain OpenTelemetry NodeSDK setup shared by server.mjs and client.mjs.
// Nothing MCP-specific: the NodeSDK registers a context manager and the default
// tracecontext,baggage propagator. Spans go to the local collector over OTLP/HTTP.
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";

export function setup(service) {
  process.env.OTEL_SERVICE_NAME = service;
  const sdk = new NodeSDK({ traceExporter: new OTLPTraceExporter({ url: "http://127.0.0.1:4318/v1/traces" }) });
  sdk.start();
  return sdk;
}
