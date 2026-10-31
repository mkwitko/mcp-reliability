"""Plain OpenTelemetry SDK setup shared by server.py and client.py.

Nothing MCP-specific lives here: spans come from the MCP Python SDK's
built-in instrumentation (tracer "mcp-python-sdk") plus the client's own
"agent step" root span. The default global propagator in opentelemetry-python
is already tracecontext,baggage (OTEL_PROPAGATORS), so no propagator setup.
"""

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


def setup(service: str) -> TracerProvider:
    tp = TracerProvider(resource=Resource.create({"service.name": service}))
    tp.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint="http://localhost:4318/v1/traces")))
    trace.set_tracer_provider(tp)
    return tp
