// Package report renders findings for humans and machines.
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mkwitko/mcp-reliability/internal/rules"
)

// Text writes a plain-text report. It returns the count per severity.
func Text(w io.Writer, file string, nSpans int, findings []rules.Finding) (fails, warns, infos int) {
	fmt.Fprintf(w, "MCP Observability Report\nfile: %s\nspans: %d\n\n", file, nSpans)
	if len(findings) == 0 {
		fmt.Fprintln(w, "[PASS] no findings")
	}
	for _, f := range findings {
		switch f.Severity {
		case rules.Fail:
			fails++
		case rules.Warn:
			warns++
		default:
			infos++
		}
		fmt.Fprintf(w, "[%s] %s: %s\n        spans: %v\n        ref: %s\n", f.Severity, f.Rule, f.Message, f.Spans, f.Ref)
	}
	fmt.Fprintf(w, "\nResult: %d FAIL, %d WARN, %d INFO\n", fails, warns, infos)
	return
}

// JSON writes findings as a JSON array.
func JSON(w io.Writer, findings []rules.Finding) error {
	type out struct {
		Rule     string   `json:"rule"`
		Severity string   `json:"severity"`
		Message  string   `json:"message"`
		Spans    []string `json:"spans"`
		Ref      string   `json:"ref"`
	}
	arr := make([]out, 0, len(findings))
	for _, f := range findings {
		arr = append(arr, out{f.Rule, f.Severity.String(), f.Message, f.Spans, f.Ref})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(arr)
}
