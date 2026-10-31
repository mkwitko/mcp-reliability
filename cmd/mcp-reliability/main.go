// Command mcp-reliability analyzes OpenTelemetry traces for MCP-specific
// observability problems.
//
//	mcp-reliability doctor [-json] [-strict] [-wire capture.jsonl] <trace.jsonl>... | -
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mkwitko/mcp-reliability/internal/report"
	"github.com/mkwitko/mcp-reliability/internal/rules"
	"github.com/mkwitko/mcp-reliability/internal/trace"
	"github.com/mkwitko/mcp-reliability/internal/wire"
)

const usage = "usage: mcp-reliability doctor [-json] [-strict] [-wire capture.jsonl] <trace.jsonl>... | -"

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
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	var spans []trace.Span
	label := ""
	for _, f := range fs.Args() {
		var (
			got []trace.Span
			err error
		)
		if f == "-" {
			got, err = trace.Load(stdin)
			f = "<stdin>"
		} else {
			got, err = trace.LoadFile(f)
		}
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %s: %v\n", f, err)
			return 2
		}
		spans = append(spans, got...)
		if label != "" {
			label += ", "
		}
		label += f
	}

	findings := rules.Run(spans)
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
	fails, warns, _ := report.Text(out, label, len(spans), findings)
	if fails > 0 || (*strict && warns > 0) {
		return 1
	}
	return 0
}
