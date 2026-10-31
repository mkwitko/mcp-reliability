"""Plain OpenTelemetry SDK setup plus openllmetry's MCPInstrumentor.

Call setup() before importing anything from `mcp`: the instrumentor patches
`mcp.client.streamable_http.streamablehttp_client` (and friends) as module
attributes, so a name imported earlier would stay unpatched.

The exporter reads OTEL_EXPORTER_OTLP_ENDPOINT (default http://localhost:4318).
The default global propagator in opentelemetry-python is tracecontext,baggage.
"""

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.mcp import McpInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


def setup(service: str) -> TracerProvider:
    tp = TracerProvider(resource=Resource.create({"service.name": service}))
    tp.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(tp)
    McpInstrumentor().instrument()
    return tp
