# 09. Validation plan

Status: nothing validated yet. This plan defines what would count as evidence.

## Hypotheses

- **H1**: Context propagation breaks by default in at least one real library. Prior evidence: openllmetry#4374 (client injection skipped when `params.meta` is None), mcp-otel-go and opentel-mcp have no client inject (`docs/research/instrumentation-libraries.md`). Status 2026-10-30: NOT reproduced with the official Python SDK 2.3.0, which propagates by default and across SDKs (Go client to Python server links correctly). Still open for openllmetry and mcp-otel-go (client side). The doctor's value so far is in error representation and semconv drift, not propagation; the framing should reflect that if E1 on the other libraries also passes.
- **H2**: `doctor` detects that failure from a recorded trace, with no false FAIL on a correct trace. Status 2026-10-30: detects the injected Go failure (MCP-PROP-001), zero false FAILs across 9 fixtures; found a real false negative (pairing by tool name) that is now fixed.
- **H3**: At least one SDK or instrumentation maintainer finds a doctor report useful.
- **H4**: At least one external user runs `doctor` on their own traces.

## Experiments

- **E1 (H1)**: run a default-configured client and server per library (start with openllmetry Python and mcp-otel-go with go-sdk), export via Collector fileexporter, inspect traces manually: same trace id on both sides or not. Record versions and commands.
- **E2 (H2)**: run `doctor` on E1 traces and on the Go example with and without `-break-propagation`; compare with the manual verdict. Check every FAIL against its quoted text.
- **E3 (H3)**: send a reproducible report (command, trace, doctor output) to 2-3 maintainers, via issue or PR. Ask one question: was this useful, and what is wrong?
- **E4 (H4)**: publish v0.1 with a working quick start; ask 5+ people building MCP servers to run it on a trace; collect results via issues or direct feedback.

## Success criteria
- H1: at least one library reproduced to lose propagation (different trace ids) in a default setup.
- H2: doctor flags it with the right rule; 0 false FAILs across all fixtures and E1 traces.
- H3: at least one maintainer replies that the report is useful, or merges a fix citing it.
- H4: at least one external run reported, with a trace or findings.

## Kill / pivot criteria
- H1 fails (all libraries propagate correctly by default): drop the "propagation is broken" framing; reconsider whether doctor has value, or narrow to attribute checks that weaver already covers (then stop).
- H2 fails after rework (false FAILs we cannot justify by the spec text): fix rules; if the standard is too ambiguous to check deterministically, narrow scope to checks that are unambiguous.
- H3 and H4 both get nothing after outreach to 5+ people over 8 weeks from v0.1: stop active development; keep the repo as reference.
Silence is not proof of failure; it is a signal to try a different channel once before killing.

## Timeline (targets)
- Week of 2026-10-30: M0 PoC, E1 started.
- By 2026-11-30: E1-E2 done, M1 v0.1 tagged.
- By 2026-12-31: E3 outreach done (at least 3 reports sent); E4 started.
- By 2027-02-28: evaluate H3/H4 against criteria; decide on M2/M3 or stop.
