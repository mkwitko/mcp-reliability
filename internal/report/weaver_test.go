package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/trace"
)

func TestWeaver(t *testing.T) {
	var buf bytes.Buffer
	err := Weaver(&buf, []trace.Span{{
		TraceID: "t", SpanID: "s", ParentID: "p", Name: "tools/call x",
		Kind: ptrace.SpanKindServer, Status: ptrace.StatusCodeError, StatusMsg: "boom",
		Attrs: map[string]string{"mcp.method.name": "tools/call"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sp := got[0]["span"]
	if sp["kind"] != "server" || sp["parent_span_id"] != "p" || sp["status"].(map[string]any)["code"] != "error" {
		t.Fatalf("unexpected: %v", sp)
	}
	if a := sp["attributes"].([]any)[0].(map[string]any); a["name"] != "mcp.method.name" {
		t.Fatalf("attrs: %v", a)
	}
}
