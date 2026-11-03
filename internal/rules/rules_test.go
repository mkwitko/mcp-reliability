package rules

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/metrics"
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
	"MCP-ERR-001": Warn, "MCP-ERR-002": Warn, "MCP-ERR-003": Warn, "MCP-ERR-004": Fail, "MCP-ERR-005": Warn, "MCP-ERR-006": Warn,
	"MCP-PROP-001": Warn, "MCP-PROP-002": Info, "MCP-PROP-003": Fail, "MCP-PROP-004": Info, "MCP-PROP-005": Info,
	"MCP-MET-001": Fail, "MCP-MET-002": Warn, "MCP-MET-003": Info,
	"MCP-PRIV-001": Warn, "MCP-PRIV-002": Info, "MCP-PROP-006": Info, "MCP-COV-001": Info,
}

// Each fixture was produced by examples/run-scenarios.sh against the official
// Go SDK v1.8.0 over Streamable HTTP with one controlled defect. The test pins
// which rules fire and how many times, so a regression or an over-eager rule
// is caught. Wire captures (testdata/<name>.wire.jsonl) are checked when present.
func TestFixtures(t *testing.T) {
	cases := map[string]map[string]int{
		"healthy":                           {},
		"healthy-stdio":                     {},
		"broken-propagation-stdio":          {"MCP-PROP-001": 4, "MCP-PROP-004": 1},
		"broken-propagation":                {"MCP-PROP-001": 6, "MCP-PROP-004": 1},
		"broken-error-type":                 {"MCP-ERR-003": 1},
		"leaked-arguments":                  {"MCP-PRIV-001": 3},
		"missing-method-attr":               {"MCP-MET-001": 1, "MCP-SEM-001": 5},
		"bad-traceparent":                   {"MCP-PROP-001": 6, "MCP-PROP-003": 6},
		"python-sdk-default":                {"MCP-MET-003": 1, "MCP-ERR-003": 2, "MCP-SEM-002": 3, "MCP-SEM-003": 5, "MCP-SEM-004": 3},
		"cross-go-client-python-server":     {"MCP-MET-003": 1},
		"openllmetry-default":               {"MCP-MET-003": 1, "MCP-ERR-001": 2, "MCP-ERR-004": 3, "MCP-PRIV-002": 15, "MCP-PROP-006": 1, "MCP-SEM-001": 10},
		"openllmetry-with-meta":             {"MCP-MET-003": 1, "MCP-ERR-001": 2, "MCP-ERR-004": 3, "MCP-PRIV-002": 15, "MCP-PROP-006": 1, "MCP-SEM-001": 10},
		"openllmetry-legacy-transport":      {"MCP-MET-003": 1, "MCP-ERR-001": 2, "MCP-ERR-004": 3, "MCP-PRIV-002": 15, "MCP-PROP-006": 1, "MCP-SEM-001": 15},
		"mcp-otel-go-default":               {"MCP-MET-003": 1, "MCP-ERR-004": 4, "MCP-ERR-006": 1, "MCP-PROP-005": 1, "MCP-SEM-004": 3},
		"mcp-otel-go-client-inject":         {"MCP-MET-003": 1, "MCP-ERR-004": 4, "MCP-ERR-006": 1, "MCP-SEM-004": 3},
		"cross-python-client-go-server":     {"MCP-MET-003": 1, "MCP-SEM-003": 6, "MCP-SEM-002": 3, "MCP-SEM-004": 3, "MCP-ERR-003": 2, "MCP-ERR-005": 1},
		"typescript-sdk-default":            {"MCP-COV-001": 1},
		"cross-go-client-typescript-server": {"MCP-MET-003": 1},
		"cross-typescript-client-go-server": {"MCP-MET-003": 1, "MCP-PROP-005": 1},
		"typescript-sdk-pr3115":             {"MCP-MET-003": 1, "MCP-ERR-003": 1, "MCP-ERR-005": 1},
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
			points, err := metrics.LoadFile(filepath.Join("..", "..", "testdata", name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			findings = append(findings, Metrics(points, spans)...)
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
		{"ERR-006 Go type name in error.type", []trace.Span{
			mcp("a", "", "tools/call x", ptrace.SpanKindServer, ptrace.StatusCodeError, map[string]string{"mcp.method.name": "tools/call", "gen_ai.operation.name": "execute_tool", "gen_ai.tool.name": "x", "error.type": "*jsonrpc2.WireError"}),
		}, []string{"MCP-ERR-006"}},
		{"PROP-005 root servers, other service, no client spans", []trace.Span{
			{TraceID: "t1", SpanID: "a", Name: "agent step", Kind: ptrace.SpanKindInternal, Service: "app", Attrs: map[string]string{}},
			{TraceID: "t2", SpanID: "b", Name: "ping", Kind: ptrace.SpanKindServer, Service: "srv", Attrs: map[string]string{"mcp.method.name": "ping"}},
		}, []string{"MCP-PROP-005"}},
		{"PROP-005 not raised for a single-service file", []trace.Span{
			{TraceID: "t2", SpanID: "b", Name: "ping", Kind: ptrace.SpanKindServer, Service: "srv", Attrs: map[string]string{"mcp.method.name": "ping"}},
		}, nil},
		{"non-MCP span named like a method prefix is ignored", []trace.Span{
			{TraceID: "t", SpanID: "a", Name: "ping-db", Kind: ptrace.SpanKindClient, Status: ptrace.StatusCodeError, Attrs: map[string]string{}},
		}, []string{"MCP-COV-001"}},
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

func TestMetricRules(t *testing.T) {
	pt := func(name, unit string, attrs map[string]string) metrics.Point {
		return metrics.Point{Name: name, Unit: unit, Kind: "histogram", Attrs: attrs, Line: 1}
	}
	span := []trace.Span{{SpanID: "a", Name: "ping", Attrs: map[string]string{"mcp.method.name": "ping"}}}
	cases := []struct {
		name   string
		points []metrics.Point
		spans  []trace.Span
		want   []string
	}{
		{"MET-001 operation metric without mcp.method.name", []metrics.Point{
			pt("mcp.server.operation.duration", "s", map[string]string{}),
		}, nil, []string{"MCP-MET-001"}},
		{"session metric needs no mcp.method.name", []metrics.Point{
			pt("mcp.client.session.duration", "s", map[string]string{}),
		}, nil, nil},
		{"MET-002 wrong unit", []metrics.Point{
			pt("mcp.client.operation.duration", "ms", map[string]string{"mcp.method.name": "ping"}),
		}, nil, []string{"MCP-MET-002"}},
		{"MET-003 spans but no MCP metric", []metrics.Point{
			pt("http.server.request.duration", "s", nil),
		}, span, []string{"MCP-MET-003"}},
		{"no spans, no metrics: silent", nil, nil, nil},
		{"conforming point", []metrics.Point{
			pt("mcp.client.operation.duration", "s", map[string]string{"mcp.method.name": "ping"}),
		}, span, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(Metrics(c.points, c.spans))
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
