// Command mcp-reliability analyzes OpenTelemetry traces for MCP-specific
// observability problems.
//
//	mcp-reliability doctor [-json] [-strict] [-wire capture.jsonl] [-weaver-json out.json] <trace.jsonl>... | -
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mkwitko/mcp-reliability/internal/metrics"
	"github.com/mkwitko/mcp-reliability/internal/report"
	"github.com/mkwitko/mcp-reliability/internal/rules"
	"github.com/mkwitko/mcp-reliability/internal/trace"
	"github.com/mkwitko/mcp-reliability/internal/wire"
)

const usage = "usage: mcp-reliability doctor [-json] [-strict] [-wire capture.jsonl] [-weaver-json out.json] <trace.jsonl>... | -"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "doctor" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit findings as JSON instead of text")
	strict := fs.Bool("strict", false, "exit non-zero on WARN as well as FAIL")
	wireFile := fs.String("wire", "", "optional JSON-RPC capture (newline-delimited {method,_meta}) for wire-format checks")
	weaverOut := fs.String("weaver-json", "", "also write spans as weaver live-check sample JSON to this path (see examples/weaver)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	var spans []trace.Span
	var points []metrics.Point
	label := ""
	for _, f := range fs.Args() {
		// Read once: stdin cannot be rewound, and spans and metrics share the file.
		var (
			data []byte
			err  error
		)
		if f == "-" {
			data, err = io.ReadAll(stdin)
			f = "<stdin>"
		} else {
			data, err = os.ReadFile(f)
		}
		var gotS []trace.Span
		var gotP []metrics.Point
		if err == nil {
			gotS, err = trace.Load(bytes.NewReader(data))
		}
		if err == nil {
			gotP, err = metrics.Load(bytes.NewReader(data))
		}
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %s: %v\n", f, err)
			return 2
		}
		spans = append(spans, gotS...)
		points = append(points, gotP...)
		if label != "" {
			label += ", "
		}
		label += f
	}

	if *weaverOut != "" {
		f, err := os.Create(*weaverOut)
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %v\n", err)
			return 2
		}
		err = report.Weaver(f, spans)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %s: %v\n", *weaverOut, err)
			return 2
		}
	}

	findings := append(rules.Run(spans), rules.Metrics(points, spans)...)
	findings = rules.Sort(findings)
	if *wireFile != "" {
		msgs, err := wire.LoadFile(*wireFile)
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %s: %v\n", *wireFile, err)
			return 2
		}
		findings = append(findings, rules.Wire(msgs)...)
		findings = rules.Sort(findings)
	}

	out := stdout
	if *asJSON {
		if err := report.JSON(stdout, findings); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		out = io.Discard
	}
	fails, warns, _ := report.Text(out, label, len(spans), len(points), findings)
	if fails > 0 || (*strict && warns > 0) {
		return 1
	}
	return 0
}
