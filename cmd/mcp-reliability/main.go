// Command mcp-reliability analyzes OpenTelemetry traces for MCP-specific
// observability problems.
//
//	mcp-reliability doctor [-json] [-strict] trace.jsonl
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mkwitko/mcp-reliability/internal/report"
	"github.com/mkwitko/mcp-reliability/internal/rules"
	"github.com/mkwitko/mcp-reliability/internal/trace"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "doctor" {
		fmt.Fprintln(os.Stderr, "usage: mcp-reliability doctor [-json] [-strict] <trace.jsonl>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit findings as JSON")
	strict := fs.Bool("strict", false, "exit non-zero on WARN as well as FAIL")
	_ = fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "doctor: expected exactly one trace file (newline-delimited OTLP JSON)")
		os.Exit(2)
	}
	file := fs.Arg(0)
	spans, err := trace.LoadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
		os.Exit(2)
	}
	findings := rules.Run(spans)
	if *asJSON {
		if err := report.JSON(os.Stdout, findings); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	var fails, warns int
	if *asJSON {
		fails, warns, _ = report.Text(io.Discard, file, len(spans), findings)
	} else {
		fails, warns, _ = report.Text(os.Stdout, file, len(spans), findings)
	}
	if fails > 0 || (*strict && warns > 0) {
		os.Exit(1)
	}
}
