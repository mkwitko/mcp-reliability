package rules

import (
	"testing"

	"github.com/mkwitko/mcp-reliability/internal/wire"
)

func TestWire(t *testing.T) {
	good := "00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01"
	cases := []struct {
		name string
		meta map[string]any
		want []string
	}{
		{"valid", map[string]any{"traceparent": good}, nil},
		{"uppercase hex", map[string]any{"traceparent": "00-0AF7651916CD43DD8448EB211C80319C-00F067AA0BA902B7-01"}, []string{"MCP-PROP-003"}},
		{"garbage", map[string]any{"traceparent": "nope"}, []string{"MCP-PROP-003"}},
		{"zero trace id", map[string]any{"traceparent": "00-00000000000000000000000000000000-00f067aa0ba902b7-01"}, []string{"MCP-PROP-003"}},
		{"non-string", map[string]any{"traceparent": 42.0}, []string{"MCP-PROP-003"}},
		{"bad tracestate type", map[string]any{"traceparent": good, "tracestate": []any{}}, []string{"MCP-PROP-003"}},
		{"absent", map[string]any{}, []string{"MCP-PROP-004"}},
		{"nil meta", nil, []string{"MCP-PROP-004"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(Wire([]wire.Message{{Method: "tools/call", Meta: c.meta, Line: 1}}))
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
