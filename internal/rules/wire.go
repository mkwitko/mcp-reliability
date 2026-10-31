package rules

import (
	"fmt"
	"regexp"

	"github.com/mkwitko/mcp-reliability/internal/wire"
)

const refSEP414 = "https://modelcontextprotocol.io/specification/draft/basic/index#meta (SEP-414)"

// W3C Trace Context traceparent: version-traceid-parentid-flags, lowercase hex,
// trace id not all zeros, parent id not all zeros.
var traceparentRe = regexp.MustCompile(`^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)

// Wire runs the checks that need the raw JSON-RPC messages.
//
// MCP-PROP-003: SEP-414 says the reserved keys traceparent, tracestate and
// baggage, when present, MUST follow the W3C formats. Only traceparent has a
// format strict enough to validate deterministically; tracestate and baggage
// are checked for being strings.
// MCP-PROP-004: a request carries no traceparent at all. The spec does not
// require propagation, so this is INFO: it explains disconnected traces.
func Wire(msgs []wire.Message) []Finding {
	var out []Finding
	missing := 0
	for _, m := range msgs {
		ev := fmt.Sprintf("wire line %d", m.Line)
		tp, has := m.Meta["traceparent"]
		if !has {
			missing++
			continue
		}
		s, isStr := tp.(string)
		switch {
		case !isStr:
			out = append(out, Finding{Rule: "MCP-PROP-003", Severity: Fail, Spans: []string{ev}, Ref: refSEP414,
				Message: fmt.Sprintf("%s: _meta.traceparent is not a string (MUST follow W3C Trace Context)", m.Method)})
		case !traceparentRe.MatchString(s) || s[3:35] == "00000000000000000000000000000000" || s[36:52] == "0000000000000000":
			out = append(out, Finding{Rule: "MCP-PROP-003", Severity: Fail, Spans: []string{ev}, Ref: refSEP414,
				Message: fmt.Sprintf("%s: _meta.traceparent does not match the W3C Trace Context format (MUST when present)", m.Method)})
		}
		for _, k := range []string{"tracestate", "baggage"} {
			if v, ok := m.Meta[k]; ok {
				if _, isStr := v.(string); !isStr {
					out = append(out, Finding{Rule: "MCP-PROP-003", Severity: Fail, Spans: []string{ev}, Ref: refSEP414,
						Message: fmt.Sprintf("%s: _meta.%s is not a string (MUST follow the W3C format)", m.Method, k)})
				}
			}
		}
	}
	if missing > 0 {
		out = append(out, Finding{Rule: "MCP-PROP-004", Severity: Info, Ref: refSEP414,
			Message: fmt.Sprintf("%d of %d captured messages carry no _meta.traceparent; the client is not propagating trace context (not required by the spec, but server spans will start new traces)", missing, len(msgs))})
	}
	return out
}
