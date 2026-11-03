#!/usr/bin/env python3
"""Render docs/compatibility-report.md from doctor -json over testdata/*.jsonl.

Usage: go build -o /tmp/doctor ./cmd/mcp-reliability && python3 scripts/compat-report.py /tmp/doctor
"""
import glob, json, os, subprocess, sys, datetime

DOCTOR = sys.argv[1]
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# fixture -> (producer, what it shows). Keep in sync with examples/*/run.sh.
META = {
    "healthy": ("go-sdk v1.8.0 client+server, hand-rolled middleware", "baseline, no injected defect"),
    "broken-propagation": ("go-sdk harness", "client skips _meta inject"),
    "broken-error-type": ("go-sdk harness", "server omits error.type on isError"),
    "leaked-arguments": ("go-sdk harness", "server records gen_ai.tool.call.arguments"),
    "missing-method-attr": ("go-sdk harness", "server omits mcp.method.name"),
    "healthy-stdio": ("go-sdk v1.8.0 client spawning the server over stdio", "baseline over stdio"),
    "broken-propagation-stdio": ("go-sdk harness over stdio", "client skips _meta inject"),
    "bad-traceparent": ("go-sdk harness", "client sends malformed _meta.traceparent (needs -wire)"),
    "python-sdk-default": ("mcp 2.3.0 (Python) client+server, built-in OTel", "SDK defaults"),
    "cross-go-client-python-server": ("go-sdk harness client -> mcp 2.3.0 server", "cross-SDK propagation"),
    "cross-python-client-go-server": ("mcp 2.3.0 client -> go-sdk harness server", "cross-SDK propagation"),
    "typescript-sdk-default": ("@modelcontextprotocol/sdk 1.32.2 (TypeScript) client+server, no MCP instrumentation", "SDK defaults"),
    "cross-go-client-typescript-server": ("go-sdk harness client -> @modelcontextprotocol/sdk 1.32.2 server", "cross-SDK propagation"),
    "cross-typescript-client-go-server": ("@modelcontextprotocol/sdk 1.32.2 client -> go-sdk harness server", "cross-SDK propagation"),
    "typescript-sdk-pr3115": ("typescript-sdk#3115 build (2.4.0 + built-in OTel) client+server", "open PR, not released"),
    "openllmetry-default": ("opentelemetry-instrumentation-mcp (traceloop) client+server", "library defaults"),
    "openllmetry-legacy-transport": ("opentelemetry-instrumentation-mcp 0.62.4, legacy SSE transport", "Python SDK mcp 1.30.0"),
    "openllmetry-with-meta": ("opentelemetry-instrumentation-mcp client with non-None meta", "#4374 workaround"),
    "mcp-otel-go-default": ("mcp-otel-go server, plain go-sdk client", "library defaults"),
    "mcp-otel-go-client-inject": ("mcp-otel-go server, client with inject middleware", "proposed upstream fix"),
}

rows = []
for path in sorted(glob.glob(os.path.join(ROOT, "testdata", "*.jsonl"))):
    name = os.path.basename(path)[:-6]
    if name.endswith(".wire"):
        continue
    args = [DOCTOR, "doctor", "-json"]
    wire = os.path.join(ROOT, "testdata", name + ".wire.jsonl")
    if os.path.exists(wire):
        args += ["-wire", wire]
    out = subprocess.run(args + [path], capture_output=True, text=True).stdout
    findings = json.loads(out)
    counts = {"FAIL": 0, "WARN": 0, "INFO": 0}
    rules = {}
    for f in findings:
        counts[f["severity"]] += 1
        rules[f["rule"]] = rules.get(f["rule"], 0) + 1
    producer, shows = META.get(name, ("(add to scripts/compat-report.py META)", ""))
    rule_txt = ", ".join(f"{r} x{n}" for r, n in sorted(rules.items())) or "none"
    rows.append(f"| `{name}` | {producer} | {shows} | {counts['FAIL']} | {counts['WARN']} | {counts['INFO']} | {rule_txt} |")

today = datetime.date.today().isoformat()
doc = f"""# Compatibility report

Generated {today} by `make report` from the fixtures in `testdata/`. Every row is
reproducible: the producer is in `examples/`, the capture script is next to it,
and the counts come from `mcp-reliability doctor -json`. Do not edit by hand.

Severity meaning: FAIL = spec MUST or semconv Required violated; WARN = SHOULD,
Conditionally Required, or sensitive data present; INFO = ambiguous evidence.
Rule texts and citations: `docs/06-architecture.md`, section 6.

| Fixture | Producer | Shows | FAIL | WARN | INFO | Rules fired |
|---|---|---|---|---|---|---|
{chr(10).join(rows)}

Reading the table:

- Rows from the go-sdk harness with an injected defect are self-tests of the
  rules, not statements about the Go SDK.
- Rows named after a library or SDK describe that library's default output on
  the date above, with the version pinned in its `examples/` directory.
- A WARN against an official SDK is a conversation to have upstream, not a
  bug in the SDK by itself: most WARN rules enforce SHOULD-level text in a
  Development-status convention. Drafts live in `docs/upstream/`.
"""
open(os.path.join(ROOT, "docs", "compatibility-report.md"), "w").write(doc)
print(f"wrote docs/compatibility-report.md ({len(rows)} rows)")
