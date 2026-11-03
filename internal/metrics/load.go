// Package metrics loads the metric data points from the same newline-delimited
// OTLP JSON files that package trace reads spans from. The Collector
// fileexporter writes one ExportMetricsServiceRequest per line with a top-level
// "resourceMetrics" key; lines of any other signal are skipped.
package metrics

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Point is one flattened data point.
type Point struct {
	Name    string
	Unit    string
	Kind    string // "histogram", "counter" (Sum) or "gauge"; other types are skipped
	Service string
	Attrs   map[string]string
	Line    int // 1-based line in the input file (for evidence)
}

// LoadFile reads a newline-delimited OTLP JSON file.
func LoadFile(path string) ([]Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load reads metric points from r. Blank lines and lines without a
// "resourceMetrics" key (spans, logs) are skipped.
func Load(r io.Reader) ([]Point, error) {
	var out []Point
	var um pmetric.JSONUnmarshaler
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 || !isMetricsLine(b) {
			continue
		}
		md, err := um.UnmarshalMetrics(b)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, flatten(md, line)...)
	}
	return out, sc.Err()
}

// isMetricsLine reports whether a JSON line is an OTLP metrics export. It
// decodes only the top-level keys.
func isMetricsLine(b []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(b, &top) != nil {
		return false
	}
	_, ok := top["resourceMetrics"]
	return ok
}

func flatten(md pmetric.Metrics, line int) []Point {
	var out []Point
	for _, rm := range md.ResourceMetrics().All() {
		service := ""
		if v, ok := rm.Resource().Attributes().Get("service.name"); ok {
			service = v.AsString()
		}
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				p := Point{Name: m.Name(), Unit: m.Unit(), Service: service, Line: line}
				switch m.Type() {
				case pmetric.MetricTypeHistogram:
					p.Kind = "histogram"
					for _, dp := range m.Histogram().DataPoints().All() {
						out = append(out, withAttrs(p, dp.Attributes()))
					}
				case pmetric.MetricTypeSum:
					p.Kind = "counter"
					for _, dp := range m.Sum().DataPoints().All() {
						out = append(out, withAttrs(p, dp.Attributes()))
					}
				case pmetric.MetricTypeGauge:
					p.Kind = "gauge"
					for _, dp := range m.Gauge().DataPoints().All() {
						out = append(out, withAttrs(p, dp.Attributes()))
					}
				}
			}
		}
	}
	return out
}

func withAttrs(p Point, attrs pcommon.Map) Point {
	p.Attrs = map[string]string{}
	for k, v := range attrs.All() {
		p.Attrs[k] = v.AsString()
	}
	return p
}
