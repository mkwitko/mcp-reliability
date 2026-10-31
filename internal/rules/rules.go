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
	refSemconvMCP   = "https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/mcp.md"
	refRecErrors    = "https://opentelemetry.io/docs/specs/semconv/general/recording-errors/"
	refProjectRules = "https://github.com/mkwitko/mcp-reliability/blob/main/docs/06-architecture.md#6-rule-catalogue (project heuristic, not a standard)"
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

// Vendor attribute names known to carry request or response payloads. These
// are not semconv attributes; the list is a project heuristic (MCP-PRIV-002)
// and must stay short and documented.
var vendorPayloadAttrs = []string{
	"traceloop.entity.input", "traceloop.entity.output", // openllmetry
	"mcp.response.value", "mcp.request.argument", // openllmetry server/client writers
}

// looksMCP reports whether a span is (or was meant to be) an MCP span.
// Besides semconv attributes and method names it recognises the openllmetry
// shape (`{tool}.tool` / `{method}.mcp` names with `traceloop.*` attributes),
// a documented heuristic so that non-conforming instrumentation is checked
// rather than silently skipped.
func looksMCP(s trace.Span) bool {
	for k := range s.Attrs {
		if strings.HasPrefix(k, "mcp.") || strings.HasPrefix(k, "gen_ai.tool.") || k == "jsonrpc.request.id" {
			return true
		}
		if strings.HasPrefix(k, "traceloop.") && (strings.HasSuffix(s.Name, ".mcp") || strings.HasSuffix(s.Name, ".tool")) {
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
	f = append(f, coverage(spans)...)
	return Sort(f)
}

// Sort orders findings by severity, highest first, keeping rule order otherwise.
func Sort(f []Finding) []Finding {
	sort.SliceStable(f, func(i, j int) bool { return f[i].Severity > f[j].Severity })
	return f
}

// MCP-SEM-001: mcp.method.name is Required on MCP spans.
// MCP-SEM-002: gen_ai.tool.name is Conditionally Required for tool operations.
// MCP-SEM-003: span name SHOULD be `{mcp.method.name} {target}` or `{mcp.method.name}`.
// MCP-SEM-004: gen_ai.operation.name=execute_tool is Recommended on tool calls.
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
		if s.Name != method && !strings.HasPrefix(s.Name, method+" ") {
			out = append(out, Finding{
				Rule: "MCP-SEM-003", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q is named neither %q nor %q (span name SHOULD follow the format {mcp.method.name} {target})", s.Name, method, method+" {target}"),
			})
		}
		if method == "tools/call" {
			if _, ok := s.Attr("gen_ai.operation.name"); !ok {
				out = append(out, Finding{
					Rule: "MCP-SEM-004", Severity: Info, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
					Message: fmt.Sprintf("tools/call span %q lacks gen_ai.operation.name=execute_tool (Recommended; lets backends treat MCP tool calls like other tool calls)", s.Name),
				})
			}
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

// JSON-RPC codes the semconv says SHOULD NOT be considered errors (caller
// sent a request the receiver could not serve).
var nonErrorCodes = map[string]bool{"-32700": true, "-32600": true, "-32601": true, "-32602": true, "-32002": true}

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
		failed := s.Status == ptrace.StatusCodeError || (hasCode && code != "OK" && code != "" && !nonErrorCodes[code])
		if failed && !hasErrType {
			out = append(out, Finding{
				Rule: "MCP-ERR-001", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q failed (status=%s, rpc.response.status_code present=%t) but lacks error.type (Conditionally Required if the operation fails)", s.Name, s.Status, hasCode),
			})
		}
		// MCP-ERR-004: status Ok on an MCP span. Recording Errors: "Span Status
		// Code MUST be left unset if the instrumented operation has ended without
		// any errors."
		if s.Status == ptrace.StatusCodeOk {
			out = append(out, Finding{
				Rule: "MCP-ERR-004", Severity: Fail, Spans: []string{s.SpanID}, Ref: refRecErrors,
				Message: fmt.Sprintf("span %q has status Ok; status MUST be left Unset when the operation ends without errors", s.Name),
			})
		}
		// MCP-ERR-005: a caller-error JSON-RPC code recorded as a failure. The
		// MCP semconv lists -32700, -32600, -32601, -32602 and -32002 as codes
		// that SHOULD NOT be considered errors.
		if hasCode && nonErrorCodes[code] && (s.Status == ptrace.StatusCodeError || hasErrType) {
			out = append(out, Finding{
				Rule: "MCP-ERR-005", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q is marked failed (status=%s, error.type present=%t) for a JSON-RPC code the semconv says SHOULD NOT be considered an error (caller-side codes -32700/-32600/-32601/-32602/-32002)", s.Name, s.Status, hasErrType),
			})
		}
		// MCP-ERR-006: error.type carrying a Go pointer type name. mcp.md
		// footnote [1]: use `tool_error` for tool results, the JSON-RPC error
		// code for protocol errors, and only otherwise an exception class name.
		if et, ok := s.Attr("error.type"); ok && strings.HasPrefix(et, "*") {
			out = append(out, Finding{
				Rule: "MCP-ERR-006", Severity: Warn, Spans: []string{s.SpanID}, Ref: refSemconvMCP,
				Message: fmt.Sprintf("span %q has error.type set to a Go pointer type name; semconv asks for tool_error or the JSON-RPC error code (low-cardinality values)", s.Name),
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
// Pairing uses (method, jsonrpc.request.id) when both sides carry the id, and
// falls back to (method, gen_ai.tool.name) otherwise.
// MCP-PROP-002: parent span id set but parent absent from the file (orphan). INFO
// because the parent may simply be unsampled or exported elsewhere.
func propagation(spans []trace.Span) []Finding {
	var out []Finding
	byID := map[string]trace.Span{}
	for _, s := range spans {
		byID[s.SpanID] = s
	}
	type key struct{ method, a string }
	byReqID := map[key][]trace.Span{}
	byTool := map[key][]trace.Span{}
	for _, s := range spans {
		if s.Kind != ptrace.SpanKindClient || !looksMCP(s) {
			continue
		}
		m, _ := s.Attr("mcp.method.name")
		if id, ok := s.Attr("jsonrpc.request.id"); ok {
			byReqID[key{m, id}] = append(byReqID[key{m, id}], s)
		}
		t, _ := s.Attr("gen_ai.tool.name")
		byTool[key{m, t}] = append(byTool[key{m, t}], s)
	}
	// MCP-PROP-005: every MCP SERVER span is a trace root and the file holds
	// no MCP CLIENT span at all, although spans from another service are
	// present. Nothing can be paired, so this is INFO: the caller is either
	// uninstrumented for MCP or does not inject into params._meta.
	var rootServers []string
	services := map[string]bool{}
	for _, s := range spans {
		services[s.Service] = true
		if looksMCP(s) && s.Kind == ptrace.SpanKindServer && s.ParentID == "" {
			rootServers = append(rootServers, s.SpanID)
		}
	}
	if len(rootServers) > 0 && len(byReqID)+len(byTool) == 0 && len(services) > 1 {
		out = append(out, Finding{
			Rule: "MCP-PROP-005", Severity: Info, Spans: rootServers, Ref: refSemconvMCP,
			Message: fmt.Sprintf("%d MCP SERVER spans are trace roots and the file has spans from %d services but no MCP CLIENT span: the client side is not instrumented for MCP or does not inject trace context into params._meta", len(rootServers), len(services)),
		})
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
		var cands []trace.Span
		if id, ok := s.Attr("jsonrpc.request.id"); ok {
			cands = byReqID[key{m, id}]
		}
		if len(cands) == 0 {
			t, _ := s.Attr("gen_ai.tool.name")
			cands = byTool[key{m, t}]
		}
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

// MCP-PRIV-002: vendor attributes known to hold payloads. Project heuristic,
// not a semconv requirement, hence INFO.
// MCP-PROP-006: MCP-looking spans exist but none is a CLIENT or SERVER span
// with mcp.method.name, so the propagation and pairing rules had nothing to
// work on. INFO so the absence of PROP findings is not read as a pass.
func coverage(spans []trace.Span) []Finding {
	var out []Finding
	mcpSpans, pairable := 0, 0
	for _, s := range spans {
		for _, a := range vendorPayloadAttrs {
			if _, ok := s.Attr(a); ok {
				out = append(out, Finding{
					Rule: "MCP-PRIV-002", Severity: Info, Spans: []string{s.SpanID}, Ref: refProjectRules,
					Message: fmt.Sprintf("span %q carries vendor attribute %s, which is known to hold request or response payloads", s.Name, a),
				})
			}
		}
		if !looksMCP(s) {
			continue
		}
		mcpSpans++
		if _, ok := s.Attr("mcp.method.name"); ok && (s.Kind == ptrace.SpanKindClient || s.Kind == ptrace.SpanKindServer) {
			pairable++
		}
	}
	// MCP-COV-001: the file has spans but none looks like MCP. Without this the
	// report prints "[PASS] no findings" for an SDK that emits no MCP spans at
	// all (typescript-sdk 1.32.2), which reads as a pass but means "nothing
	// was checked".
	if len(spans) > 0 && mcpSpans == 0 {
		out = append(out, Finding{
			Rule: "MCP-COV-001", Severity: Info, Ref: refProjectRules,
			Message: fmt.Sprintf("%d spans loaded but none looks like an MCP span (no mcp.* or gen_ai.tool.* attribute, no known MCP method name); no MCP rule ran", len(spans)),
		})
	}
	if mcpSpans > 0 && pairable == 0 {
		out = append(out, Finding{
			Rule: "MCP-PROP-006", Severity: Info, Ref: refSemconvMCP,
			Message: fmt.Sprintf("%d MCP-looking spans but none is a CLIENT or SERVER span with mcp.method.name; propagation and client/server pairing checks (MCP-PROP-001, MCP-ERR-003) could not run", mcpSpans),
		})
	}
	return out
}
