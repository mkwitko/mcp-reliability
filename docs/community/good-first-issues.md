# Good first issues (drafts)

Draft issue bodies to open on GitHub once a second person shows up. Each one
is small, has a test to prove it, and touches one package.

## 1. `doctor -html`: self-contained static report

Render the same findings as a single HTML file (no JS framework, no network).
Acceptance: `doctor -html out.html testdata/broken-propagation.jsonl` writes a
file that lists findings grouped by severity with rule id, message, span ids,
and the reference link. Test: golden file comparison. See ADR-0005.
Labels: good first issue.

## 2. Recognise `server/discover` and other go-sdk internal methods

`looksMCP` only knows the semconv enum. go-sdk v1.8.0 emits `server/discover`
(stateless mode) which is recognised only through its attributes. Decide, with
a citation, whether such methods belong in `knownMethods` and add a unit test.
Labels: good first issue, rule, research.

## 3. MCP-PROP-001: match client/server pairs by time window

Today the rule pairs on (method, tool name) only, so two concurrent calls to
one tool can mask a broken link. Use start/end timestamps (add them to
`trace.Span`) to pair the nearest overlapping client span.
Acceptance: a synthetic test with two interleaved calls where only one is
disconnected reports exactly one finding.
Labels: rule.

## 4. Jaeger JSON input

Add a loader for Jaeger's `/api/traces` JSON export so users of a Jaeger UI can
`doctor` what they see. Keep `trace.Span` as the common form. Verify the format
against a real export, cite the source.
Labels: good first issue, research.

## 5. Severity policy flag

`-fail-on <rule-id,...>` to promote specific WARN rules to FAIL in CI without
`-strict` promoting all of them. Test in `cmd/mcp-reliability/main_test.go`.
Labels: good first issue.

## 6. Document how to capture wire records from a Python or TypeScript server

The `-wire` input only has a Go producer. Write a 20-line middleware/hook for
one other SDK that appends `{"method","_meta"}` lines, and a fixture.
Labels: sdk-scenario.
