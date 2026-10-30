package trace

import (
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

const line = `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"srv"}}]},"scopeSpans":[{"scope":{"name":"x"},"spans":[{"traceId":"0af7651916cd43dd8448eb211c80319c","spanId":"00f067aa0ba902b7","parentSpanId":"b7ad6b7169203331","name":"tools/call echo","kind":2,"startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[{"key":"mcp.method.name","value":{"stringValue":"tools/call"}}],"status":{"code":2}}]}]}]}`

func TestLoad(t *testing.T) {
	spans, err := Load(strings.NewReader(line + "\n\n" + line + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 2 {
		t.Fatalf("got %d spans", len(spans))
	}
	s := spans[1]
	if s.TraceID != "0af7651916cd43dd8448eb211c80319c" || s.SpanID != "00f067aa0ba902b7" || s.ParentID != "b7ad6b7169203331" {
		t.Fatalf("ids: %+v", s)
	}
	if s.Kind != ptrace.SpanKindServer || s.Status != ptrace.StatusCodeError || s.Service != "srv" || s.Line != 3 {
		t.Fatalf("fields: %+v", s)
	}
	if v, _ := s.Attr("mcp.method.name"); v != "tools/call" {
		t.Fatalf("attr: %q", v)
	}
	if _, err := Load(strings.NewReader("{not json")); err == nil {
		t.Fatal("expected error on bad line")
	}
}
