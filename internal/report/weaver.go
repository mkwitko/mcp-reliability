package report

import (
	"encoding/json"
	"io"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/mkwitko/mcp-reliability/internal/trace"
)

// Weaver writes spans in the sample JSON format read by
// `weaver registry live-check --input-format json`, so users can validate
// attribute names, types and enum values against the semconv registry
// without this tool doing any network I/O. See examples/weaver/README.md.
//
// Format verified against weaver v0.28.0 on 2026-10-30
// (docs/research/weaver-live-check-feasibility.md).
func Weaver(w io.Writer, spans []trace.Span) error {
	type attr struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type status struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type span struct {
		Name         string `json:"name"`
		Kind         string `json:"kind"`
		Attributes   []attr `json:"attributes"`
		Status       status `json:"status"`
		TraceID      string `json:"trace_id"`
		SpanID       string `json:"span_id"`
		ParentSpanID string `json:"parent_span_id,omitempty"`
	}
	type sample struct {
		Span *span `json:"span,omitempty"`
	}
	out := make([]sample, 0, len(spans))
	for _, s := range spans {
		attrs := make([]attr, 0, len(s.Attrs))
		for k, v := range s.Attrs {
			attrs = append(attrs, attr{k, v})
		}
		out = append(out, sample{Span: &span{
			Name:         s.Name,
			Kind:         weaverKind(s.Kind),
			Attributes:   attrs,
			Status:       status{Code: weaverStatus(s.Status), Message: s.StatusMsg},
			TraceID:      s.TraceID,
			SpanID:       s.SpanID,
			ParentSpanID: s.ParentID,
		}})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(out)
}

func weaverKind(k ptrace.SpanKind) string {
	switch k {
	case ptrace.SpanKindServer:
		return "server"
	case ptrace.SpanKindClient:
		return "client"
	case ptrace.SpanKindProducer:
		return "producer"
	case ptrace.SpanKindConsumer:
		return "consumer"
	default:
		return "internal"
	}
}

func weaverStatus(c ptrace.StatusCode) string {
	switch c {
	case ptrace.StatusCodeOk:
		return "ok"
	case ptrace.StatusCodeError:
		return "error"
	default:
		return "unset"
	}
}
