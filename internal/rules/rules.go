// Package rules holds the deterministic checks that mcp-reliability runs over
// a loaded trace. Every rule cites the normative text it enforces. A rule must
// never report a violation where the cited standard does not require the
// behavior; "SHOULD" level text produces WARN, "MUST"/"Required" produces FAIL.
package rules

import (
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/trace"
)

// Severity of a finding.
type Severity int

const (
	Info Severity = iota
	Warn
	Fail
)

func (s Severity) String() string {
	switch s {
	case Fail:
		return "FAIL"
	case Warn:
		return "WARN"
	default:
		return "INFO"
	}
}

// Finding is one detected problem with evidence pointing at concrete spans.
type Finding struct {
	Rule     string
	Severity Severity
	Message  string
	Spans    []string // span ids that are the evidence
	Ref      string   // URL of the normative text
}

const (
	refSemconvMCP = "https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md"
	refRecErrors  = "https://opentelemetry.io/docs/specs/semconv/general/recording-errors/"
)

// Well-known MCP method names from the semconv enum (subset is enough to
// recognise MCP spans that forgot mcp.method.name).
var knownMethods = []string{
	"initialize", "ping", "tools/call", "tools/list", "resources/read", "resources/list",
	"resources/subscribe", "resources/unsubscribe", "resources/templates/list",
	"prompts/get", "prompts/list", "completion/complete", "logging/setLevel",
	"sampling/createMessage", "elicitation/create", "roots/list", "notifications/",
}

// Sensitive opt-in attributes per semconv (payload-bearing).
var sensitiveAttrs = []string{
	"gen_ai.tool.call.arguments", "gen_ai.tool.call.result",
	"gen_ai.input.messages", "gen_ai.output.messages",
}

// looksMCP reports whether a span is (or was meant to be) an MCP span.
func looksMCP(s trace.Span) bool {
	for k := range s.Attrs {
		if strings.HasPrefix(k, "mcp.") || strings.HasPrefix(k, "gen_ai.tool.") || k == "jsonrpc.request.id" {
			return true
		}
	}
	for _, m := range knownMethods {
		if s.Name == m || strings.HasPrefix(s.Name, m+" ") || (strings.HasSuffix(m, "/") && strings.HasPrefix(s.Name, m)) {
			return true
		}
	}
	return false
}

// Run executes all rules and returns findings sorted by severity (highest first).
func Run(spans []trace.Span) []Finding {
	var f []Finding
	f = append(f, requiredAttributes(spans)...)
	f = append(f, errorRepresentation(spans)...)
	f = append(f, pairConsistency(spans)...)
	f = append(f, propagation(spans)...)
	f = append(f, sensitiveData(spans)...)
	sort.SliceStable(f, func(i, j int) bool { return f[i].Severity > f[j].Severity })
	return f
}

// MCP-SEM-001: mcp.method.name is Required on MCP spans.
// MCP-SEM-002: gen_ai.tool.name is Conditionally Required for tool operations.
func requiredAttributes(spans []trace.Span) []Finding {
	var out []Finding
	for _, s := range spans {
		if !looksMCP(s) {
			continue
		}
		method, has := s.Attr("mcp.method.name")
		if !has {
			out = append(out, Finding{
				Rule: "MCP-SEM-001", Severity: Fail, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q looks like an MCP span but lacks mcp.method.name (Required)", s.Name),
			})
			continue
		}
		if method == "tools/call" {
			if _, ok := s.Attr("gen_ai.tool.name"); !ok {
				out = append(out, Finding{
					Rule: "MCP-SEM-002", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
					Message: fmt.Sprintf("tools/call span %q lacks gen_ai.tool.name (Conditionally Required when the operation relates to a tool)", s.Name),
				})
			}
		}
	}
	return out
}

// MCP-ERR-001: error.type is Conditionally Required if and only if the operation fails.
// MCP-ERR-002: a failed operation whose span status is Unset (recording-errors guidance).
func errorRepresentation(spans []trace.Span) []Finding {
	var out []Finding
	for _, s := range spans {
		if !looksMCP(s) {
			continue
		}
		_, hasErrType := s.Attr("error.type")
		code, hasCode := s.Attr("rpc.response.status_code")
		failed := s.Status == ptrace.StatusCodeError || (hasCode && code != "OK" && code != "")
		if failed && !hasErrType {
			out = append(out, Finding{
				Rule: "MCP-ERR-001", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q failed (status=%s, rpc.response.status_code present=%t) but lacks error.type (Conditionally Required if the operation fails)", s.Name, s.Status, hasCode),
			})
		}
		if hasErrType && s.Status == ptrace.StatusCodeUnset {
			out = append(out, Finding{
				Rule: "MCP-ERR-002", Severity: Warn, Spans: []string{s.SpanID}, Ref: refRecErrors,
				Message: fmt.Sprintf("span %q carries error.type but its status is Unset; backends will not show it as failed", s.Name),
			})
		}
	}
	return out
}

// MCP-PROP-001: a SERVER span for an MCP call whose trace differs from the CLIENT
// span of the same call => context was not propagated across the MCP boundary.
// The semconv says instrumentation SHOULD inject/extract via params._meta, and
// SEP-414 reserves the keys; propagation itself is not a MUST, so this is WARN.
// MCP-PROP-002: parent span id set but parent absent from the file (orphan). INFO
// because the parent may simply be unsampled or exported elsewhere.
func propagation(spans []trace.Span) []Finding {
	var out []Finding
	byID := map[string]trace.Span{}
	for _, s := range spans {
		byID[s.SpanID] = s
	}
	type key struct{ method, target string }
	clients := map[key][]trace.Span{}
	for _, s := range spans {
		if s.Kind != ptrace.SpanKindClient || !looksMCP(s) {
			continue
		}
		m, _ := s.Attr("mcp.method.name")
		t, _ := s.Attr("gen_ai.tool.name")
		clients[key{m, t}] = append(clients[key{m, t}], s)
	}
	for _, s := range spans {
		if !looksMCP(s) {
			continue
		}
		if s.ParentID != "" {
			if _, ok := byID[s.ParentID]; !ok {
				out = append(out, Finding{
					Rule: "MCP-PROP-002", Severity: Info, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
					Message: fmt.Sprintf("span %q references parent %s which is not in this file (unsampled, dropped, or exported elsewhere?)", s.Name, s.ParentID),
				})
			}
		}
		if s.Kind != ptrace.SpanKindServer || s.ParentID != "" {
			continue
		}
		m, _ := s.Attr("mcp.method.name")
		t, _ := s.Attr("gen_ai.tool.name")
		cands := clients[key{m, t}]
		if len(cands) == 0 {
			continue
		}
		sameTrace := false
		for _, c := range cands {
			if c.TraceID == s.TraceID {
				sameTrace = true
				break
			}
		}
		if !sameTrace {
			out = append(out, Finding{
				Rule: "MCP-PROP-001", Severity: Warn, Spans: []string{s.SpanID, cands[0].SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("SERVER span %q (service %q, trace %s) is a root span while CLIENT span for the same call exists in trace %s: trace context was not propagated via params._meta", s.Name, s.Service, s.TraceID, cands[0].TraceID),
			})
		}
	}
	return out
}

// MCP-PRIV-001: opt-in payload attributes are present. Not a violation; the
// semconv marks them Opt-In and sensitive, so surface them for review.
func sensitiveData(spans []trace.Span) []Finding {
	var out []Finding
	for _, s := range spans {
		for _, a := range sensitiveAttrs {
			if _, ok := s.Attr(a); ok {
				out = append(out, Finding{
					Rule: "MCP-PRIV-001", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
					Message: fmt.Sprintf("span %q carries opt-in payload attribute %s; confirm this is intended and redacted", s.Name, a),
				})
			}
		}
	}
	return out
}

// MCP-ERR-003: a SERVER span whose parent is a CLIENT span of the same MCP
// call disagrees with it about failure. One side recorded error.type, the
// other did not. Propagation worked, but one instrumentation dropped the error.
func pairConsistency(spans []trace.Span) []Finding {
	var out []Finding
	byID := map[string]trace.Span{}
	for _, s := range spans {
		byID[s.SpanID] = s
	}
	for _, srv := range spans {
		if srv.Kind != ptrace.SpanKindServer || !looksMCP(srv) || srv.ParentID == "" {
			continue
		}
		cli, ok := byID[srv.ParentID]
		if !ok || cli.Kind != ptrace.SpanKindClient {
			continue
		}
		sm, _ := srv.Attr("mcp.method.name")
		cm, _ := cli.Attr("mcp.method.name")
		if sm != cm {
			continue
		}
		_, srvErr := srv.Attr("error.type")
		_, cliErr := cli.Attr("error.type")
		if srvErr != cliErr {
			side := "server"
			if srvErr {
				side = "client"
			}
			out = append(out, Finding{
				Rule: "MCP-ERR-003", Severity: Warn, Spans: []string{cli.SpanID, srv.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("client and server spans for %q disagree on failure: the %s span lacks error.type", srv.Name, side),
			})
		}
	}
	return out
}
