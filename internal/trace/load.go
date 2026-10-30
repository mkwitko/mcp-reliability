// Package trace loads OTLP JSON trace files into a flat, analyzer-friendly form.
//
// The input format is the newline-delimited OTLP JSON written by the
// OpenTelemetry Collector fileexporter: one ExportTraceServiceRequest per line.
// Parsing uses pdata's JSON unmarshaler so hex trace/span ids and enums are
// decoded exactly as the collector encodes them.
package trace

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Span is a flattened span with only the fields the rules need.
type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string // empty for root spans
	Name      string
	Kind      ptrace.SpanKind
	Status    ptrace.StatusCode
	StatusMsg string
	Service   string
	Attrs     map[string]string // scalar attributes rendered as strings
	Line      int               // 1-based line in the input file (for evidence)
}

// Attr returns the attribute value and whether it was present.
func (s Span) Attr(key string) (string, bool) {
	v, ok := s.Attrs[key]
	return v, ok
}

// LoadFile reads a newline-delimited OTLP JSON file.
func LoadFile(path string) ([]Span, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load reads newline-delimited OTLP JSON from r. Blank lines are skipped.
func Load(r io.Reader) ([]Span, error) {
	var out []Span
	var um ptrace.JSONUnmarshaler
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // fileexporter lines can be large
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		td, err := um.UnmarshalTraces(b)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, flatten(td, line)...)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func flatten(td ptrace.Traces, line int) []Span {
	var out []Span
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		service := ""
		if v, ok := rs.Resource().Attributes().Get("service.name"); ok {
			service = v.AsString()
		}
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				s := spans.At(k)
				fs := Span{
					TraceID:   s.TraceID().String(),
					SpanID:    s.SpanID().String(),
					Name:      s.Name(),
					Kind:      s.Kind(),
					Status:    s.Status().Code(),
					StatusMsg: s.Status().Message(),
					Service:   service,
					Attrs:     map[string]string{},
					Line:      line,
				}
				if !s.ParentSpanID().IsEmpty() {
					fs.ParentID = s.ParentSpanID().String()
				}
				for key, v := range s.Attributes().All() {
					fs.Attrs[key] = renderAttr(v)
				}
				out = append(out, fs)
			}
		}
	}
	return out
}

func renderAttr(v pcommon.Value) string { return v.AsString() }
