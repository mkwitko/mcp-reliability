package rules

import (
	"fmt"
	"sort"

	"github.com/mkwitko/mcp-reliability/internal/metrics"
	"github.com/mkwitko/mcp-reliability/internal/trace"
)

// Source for all metric rules: refSemconvMCP, section "Metrics", verified
// 2026-11-03 (extract in docs/research/mcp-semconv-metrics.md). Four metrics
// are defined, all Histograms with unit `s`, requirement level Recommended:
// mcp.client.operation.duration, mcp.server.operation.duration,
// mcp.client.session.duration, mcp.server.session.duration.
var mcpMetricUnit = map[string]string{
	"mcp.client.operation.duration": "s", "mcp.server.operation.duration": "s",
	"mcp.client.session.duration": "s", "mcp.server.session.duration": "s",
}

// Only the operation metrics list mcp.method.name (`Required`); the session
// metrics have no Required attribute.
var mcpOperationMetric = map[string]bool{
	"mcp.client.operation.duration": true, "mcp.server.operation.duration": true,
}

// Metrics runs the rules over metric data points. spans is needed only for
// MCP-MET-003.
//
// MCP-MET-001 (FAIL): the attribute table of both operation-duration metrics
// lists `mcp.method.name` with requirement level `Required`.
// MCP-MET-002 (WARN): the metric tables give the unit (UCUM) as `s`. The text
// is a table cell, not a MUST, so a different unit is WARN.
// MCP-MET-003 (INFO, project heuristic): spans show MCP operations but the
// file holds no MCP metric. The semconv requirement level of the metrics is
// Recommended, so absence is never a violation; the finding explains what
// was not checked. False positive: metrics exported by a different pipeline
// into another file.
func Metrics(points []metrics.Point, spans []trace.Span) []Finding {
	var out []Finding
	missing := map[string][]string{} // metric -> evidence lines
	units := map[string]map[string]bool{}
	seen := false
	for _, p := range points {
		want, ok := mcpMetricUnit[p.Name]
		if !ok {
			continue
		}
		seen = true
		ev := fmt.Sprintf("metrics line %d", p.Line)
		if mcpOperationMetric[p.Name] {
			if _, has := p.Attrs["mcp.method.name"]; !has && !contains(missing[p.Name], ev) {
				missing[p.Name] = append(missing[p.Name], ev)
			}
		}
		if p.Unit != want {
			if units[p.Name] == nil {
				units[p.Name] = map[string]bool{}
			}
			units[p.Name][p.Unit] = true
		}
	}
	for _, name := range sortedKeys(missing) {
		out = append(out, Finding{Rule: "MCP-MET-001", Severity: Fail, Spans: missing[name], Ref: refSemconvMCP,
			Message: fmt.Sprintf("metric %s has data points without mcp.method.name (Required)", name)})
	}
	for _, name := range sortedKeys(units) {
		out = append(out, Finding{Rule: "MCP-MET-002", Severity: Warn, Ref: refSemconvMCP,
			Message: fmt.Sprintf("metric %s has unit %q, the semconv says %q", name, sortedKeys(units[name])[0], mcpMetricUnit[name])})
	}
	if !seen {
		n := 0
		for _, s := range spans {
			if looksMCP(s) {
				n++
			}
		}
		if n > 0 {
			out = append(out, Finding{Rule: "MCP-MET-003", Severity: Info, Ref: refProjectRules,
				Message: fmt.Sprintf("%d MCP-looking spans but no MCP metric (mcp.{client,server}.{operation,session}.duration) in the file; metric rules did not run", n)})
		}
	}
	return out
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
