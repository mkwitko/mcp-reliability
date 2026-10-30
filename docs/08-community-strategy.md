# 08. Community strategy

Status: plan for a pre-alpha, single-maintainer project. No community exists yet.

## Governance
- Now: single maintainer (benevolent decision maker). Decisions recorded as ADRs in `docs/adr/`.
- Target: 2+ maintainers before v0.2, so no single point of failure. Criteria: sustained quality contributions (e.g. 3+ merged non-trivial PRs, including at least one rule or scenario) and agreement with the rule-citation policy.
- Rule policy is the core governance principle: no violation without a quotable normative source.
- If the project fits, consider donating to or aligning with an OTel or MCP working group later. Not before it has external users.

## Contribution paths
- **New rules**: issue template "New rule"; needs citation and fixtures.
- **New SDK scenarios**: issue template "New SDK scenario"; reproduces a failure for a specific library version.
- **Fixtures from real traces**: redacted `.jsonl` with library and version noted. Highest-value contribution, because real traces find real bugs.
- Docs fixes and bug reports.

## Partnerships to approach
Only after M1 gives something concrete to show.
- OpenTelemetry GenAI SIG (semantic-conventions-genai): share findings on MCP semconv gaps; ask about weaver registry use.
- MCP conformance maintainers: propose the `_meta.traceparent` scenario (M3).
- Authors of mcp-otel-go and opentel-mcp, and openllmetry maintainers: send reproducible reports, not demands.
- MCP SDK maintainers (Go SDK, Python SDK) for feedback on reports (H3).
Expect slow or no replies; maintainers are busy. Lead with a reproducible report and a small PR.

## Funding options
- GitHub Sponsors, Open Collective: realistic expectation is little or nothing for a niche developer tool; do not plan on it.
- Grants (e.g. open-source programs from foundations or companies): possible but competitive and slow; need external users and a track record first.
- Employer or consulting sponsorship: more likely than donations; unproven.
No funding is assumed in the roadmap. The project must be maintainable as a side effort.

## Metrics beyond stars
- External users who ran `doctor` on their own traces (self-reported, issues, discussions).
- Fixtures contributed from real traces.
- Upstream PRs opened/merged and issues linked to doctor evidence.
- Distinct contributors and time-to-first-response on issues.
- Rules added with a citation by someone other than the maintainer.
- Bugs found in third-party libraries that were confirmed by their maintainers.
