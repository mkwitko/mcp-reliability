package rules

import (
	"path/filepath"
	"sort"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/trace"
)

// Each fixture was produced by examples/run-scenarios.sh against the official
// Go SDK v1.8.0 over Streamable HTTP with one controlled defect. The test pins
// which rules fire so a rule regression or an over-eager rule is caught.
func TestFixtures(t *testing.T) {
	cases := map[string][]string{
		"healthy":             {},
		"broken-propagation":  {"MCP-PROP-001"},
		"broken-error-type":   {"MCP-ERR-001", "MCP-ERR-003"},
		"leaked-arguments":    {"MCP-PRIV-001"},
		"missing-method-attr": {"MCP-SEM-001"},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			spans, err := trace.LoadFile(filepath.Join("..", "..", "testdata", name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if len(spans) == 0 {
				t.Fatal("no spans loaded")
			}
			got := map[string]bool{}
			for _, f := range Run(spans) {
				got[f.Rule] = true
			}
			var gotList []string
			for r := range got {
				gotList = append(gotList, r)
			}
			sort.Strings(gotList)
			if len(gotList) != len(want) {
				t.Fatalf("rules fired = %v, want %v", gotList, want)
			}
			for i := range want {
				if gotList[i] != want[i] {
					t.Fatalf("rules fired = %v, want %v", gotList, want)
				}
			}
		})
	}
}

func ids(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	sort.Strings(out)
	return out
}

func TestSyntheticRules(t *testing.T) {
	mcp := func(id, parent, name string, kind ptrace.SpanKind, status ptrace.StatusCode, attrs map[string]string) trace.Span {
		return trace.Span{TraceID: "t", SpanID: id, ParentID: parent, Name: name, Kind: kind, Status: status, Attrs: attrs}
	}
	cases := []struct {
		name  string
		spans []trace.Span
		want  []string
	}{
		{"SEM-002 tools/call without tool name", []trace.Span{
			mcp("a", "", "tools/call", ptrace.SpanKindClient, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "tools/call"}),
		}, []string{"MCP-SEM-002"}},
		{"ERR-002 error.type with Unset status", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindClient, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "tools/call", "gen_ai.tool.name": "x", "error.type": "tool_error"}),
		}, []string{"MCP-ERR-002"}},
		{"PROP-002 parent missing from file", []trace.Span{
			mcp("a", "zz", "ping", ptrace.SpanKindServer, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "ping"}),
		}, []string{"MCP-PROP-002"}},
		{"non-MCP span named like a method prefix is ignored", []trace.Span{
			{TraceID: "t", SpanID: "a", Name: "ping-db", Kind: ptrace.SpanKindClient, Status: ptrace.StatusCodeError, Attrs: map[string]string{}},
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(Run(c.spans))
			if len(got) != len(c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v want %v", got, c.want)
				}
			}
		})
	}
}
