package rules

import (
	"path/filepath"
	"sort"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/trace"
	"github.com/mkwitko/mcp-reliability/internal/wire"
)

// Each fixture was produced by examples/run-scenarios.sh against the official
// Go SDK v1.8.0 over Streamable HTTP with one controlled defect. The test pins
// which rules fire so a rule regression or an over-eager rule is caught.
// severityOf pins each rule's severity. Changing a severity is a reviewed
// decision (see docs/06-architecture.md, rule contract), so it fails here first.
var severityOf = map[string]Severity{
	"MCP-SEM-001": Fail, "MCP-SEM-002": Warn, "MCP-SEM-003": Warn, "MCP-SEM-004": Info,
	"MCP-ERR-001": Warn, "MCP-ERR-002": Warn, "MCP-ERR-003": Warn, "MCP-ERR-004": Fail, "MCP-ERR-005": Warn,
	"MCP-PROP-001": Warn, "MCP-PROP-002": Info, "MCP-PROP-003": Fail, "MCP-PROP-004": Info,
	"MCP-PRIV-001": Warn,
}

// Each fixture was produced by examples/run-scenarios.sh against the official
// Go SDK v1.8.0 over Streamable HTTP with one controlled defect. The test pins
// which rules fire and how many times, so a regression or an over-eager rule
// is caught. Wire captures (testdata/<name>.wire.jsonl) are checked when present.
func TestFixtures(t *testing.T) {
	cases := map[string]map[string]int{
		"healthy":                       {},
		"broken-propagation":            {"MCP-PROP-001": 6, "MCP-PROP-004": 1},
		"broken-error-type":             {"MCP-ERR-003": 1},
		"leaked-arguments":              {"MCP-PRIV-001": 3},
		"missing-method-attr":           {"MCP-SEM-001": 5},
		"bad-traceparent":               {"MCP-PROP-001": 6, "MCP-PROP-003": 6},
		"python-sdk-default":            {"MCP-ERR-003": 2, "MCP-SEM-002": 3, "MCP-SEM-003": 5, "MCP-SEM-004": 3},
		"cross-go-client-python-server": {},
		"cross-python-client-go-server": {"MCP-SEM-003": 6, "MCP-SEM-002": 3, "MCP-SEM-004": 3, "MCP-ERR-003": 2, "MCP-ERR-005": 1},
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
			findings := Run(spans)
			if msgs, err := wire.LoadFile(filepath.Join("..", "..", "testdata", name+".wire.jsonl")); err == nil {
				findings = append(findings, Wire(msgs)...)
			}
			got := map[string]int{}
			for _, f := range findings {
				got[f.Rule]++
				if sev, ok := severityOf[f.Rule]; !ok || sev != f.Severity {
					t.Errorf("%s has severity %s, pinned %s", f.Rule, f.Severity, sev)
				}
				if f.Ref == "" || f.Message == "" {
					t.Errorf("%s lacks ref or message", f.Rule)
				}
			}
			if len(got) != len(want) {
				t.Fatalf("rules fired = %v, want %v", got, want)
			}
			for r, n := range want {
				if got[r] != n {
					t.Fatalf("rules fired = %v, want %v", got, want)
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
		}, []string{"MCP-SEM-002", "MCP-SEM-004"}},
		{"ERR-002 error.type with Unset status", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindClient, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "tools/call", "gen_ai.operation.name": "execute_tool", "gen_ai.tool.name": "x", "error.type": "tool_error"}),
		}, []string{"MCP-ERR-002"}},
		{"PROP-002 parent missing from file", []trace.Span{
			mcp("a", "zz", "ping", ptrace.SpanKindServer, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "ping"}),
		}, []string{"MCP-PROP-002"}},
		{"ERR-001 failed without error.type", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindServer, ptrace.StatusCodeError, map[string]string{"mcp.method.name": "tools/call", "gen_ai.operation.name": "execute_tool", "gen_ai.tool.name": "x"}),
		}, []string{"MCP-ERR-001"}},
		{"ERR-004 status Ok", []trace.Span{
			mcp("a", "", "ping", ptrace.SpanKindClient, ptrace.StatusCodeOk, map[string]string{"mcp.method.name": "ping"}),
		}, []string{"MCP-ERR-004"}},
		{"ERR-005 unknown tool marked as error", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindClient, ptrace.StatusCodeError, map[string]string{"mcp.method.name": "tools/call", "gen_ai.operation.name": "execute_tool", "gen_ai.tool.name": "x", "error.type": "-32602", "rpc.response.status_code": "-32602"}),
		}, []string{"MCP-ERR-005"}},
		{"unknown tool unset status is not a failure", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindClient, ptrace.StatusCodeUnset, map[string]string{"mcp.method.name": "tools/call", "gen_ai.operation.name": "execute_tool", "gen_ai.tool.name": "x", "rpc.response.status_code": "-32602"}),
		}, nil},
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
