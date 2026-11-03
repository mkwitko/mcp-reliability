package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	cases := []struct {
		args []string
		code int
		out  string
	}{
		{[]string{}, 2, ""},
		{[]string{"doctor"}, 2, ""},
		{[]string{"doctor", "../../testdata/healthy.jsonl"}, 0, "[PASS]"},
		{[]string{"doctor", "../../testdata/missing-method-attr.jsonl"}, 1, "MCP-SEM-001"},
		{[]string{"doctor", "../../testdata/broken-propagation.jsonl"}, 0, "MCP-PROP-001"},
		{[]string{"doctor", "-strict", "../../testdata/broken-propagation.jsonl"}, 1, "MCP-PROP-001"},
		{[]string{"doctor", "-json", "../../testdata/healthy.jsonl"}, 0, "[]"},
		{[]string{"doctor", "../../testdata/healthy.jsonl", "../../testdata/leaked-arguments.jsonl"}, 0, "MCP-PRIV-001"},
		{[]string{"doctor", "../../testdata/healthy.jsonl"}, 0, "metric points: 12"},
		{[]string{"doctor", "../../testdata/missing-method-attr.jsonl"}, 1, "MCP-MET-001"},
		{[]string{"doctor", "../../testdata/python-sdk-default.jsonl"}, 0, "MCP-MET-003"},
		{[]string{"doctor", "nope.jsonl"}, 2, ""},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			var out, errb bytes.Buffer
			code := run(c.args, strings.NewReader(""), &out, &errb)
			if code != c.code {
				t.Fatalf("exit %d want %d; stderr=%s", code, c.code, errb.String())
			}
			if !strings.Contains(out.String(), c.out) {
				t.Fatalf("stdout missing %q:\n%s", c.out, out.String())
			}
		})
	}
}

func TestRunWeaverJSON(t *testing.T) {
	out := t.TempDir() + "/w.json"
	var buf bytes.Buffer
	if code := run([]string{"doctor", "-weaver-json", out, "../../testdata/healthy.jsonl"}, strings.NewReader(""), &buf, &buf); code != 0 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"mcp.method.name"`) {
		t.Fatalf("weaver file: %v %s", err, b)
	}
}

func TestRunStdin(t *testing.T) {
	b, err := os.ReadFile("../../testdata/healthy.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"doctor", "-"}, bytes.NewReader(b), &out, &out); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "<stdin>") {
		t.Fatal(out.String())
	}
}
