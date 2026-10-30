# 10. Engineering practices

Status: practices this repo adopts as of 2026-10-30. Items marked "todo" are not done.

## Go

- `gofmt -l .` must be empty; CI fails otherwise. Removes style debate.
- `go vet`, `staticcheck` (version pinned in the Makefile), `govulncheck` (pinned), and `go test -race`. Each catches a different class of bug; all run in CI and in `make check`.
- Table tests where inputs vary. Fixtures are golden contracts: `TestFixtures` pins which rule ids fire on each captured trace.
- Minimal dependencies. Current direct dependencies serve two jobs: pdata to parse OTLP JSON, and the Go SDK plus OTel SDK in the example harness. A new dependency needs a written reason in the PR.
- `internal/` for everything. There is no public Go API to keep stable yet.
- No interface unless there are two implementations. Rules are plain functions.
- Deterministic output: same input, same bytes.

## Versioning

- SemVer. Stay on v0.x until the OTel GenAI/MCP semantic conventions leave Development stability, because rule ids and severities may have to change with them.
- `CHANGELOG.md` follows Keep a Changelog; every user-visible change adds an Unreleased line.
- Tags are created only by the maintainer.

## CI and supply chain

- GitHub Actions: gofmt, vet, test -race, govulncheck, staticcheck on push to main and on PRs, with read-only token permissions.
- Tool versions are Makefile variables (`GOVULNCHECK_VERSION`, `STATICCHECK_VERSION`), so local and CI runs match.
- Dependabot weekly for gomod (OpenTelemetry modules grouped into one PR) and github-actions.
- `.github/CODEOWNERS` assigns review to the maintainer.
- Todo (not done): pin Actions by commit SHA.

## Security

- `SECURITY.md` describes private reporting.
- No secrets in fixtures. Fixture payloads are reviewed by a human before merge; the demo string `secret-token-123` is deliberate and fake.
- `doctor` does no network I/O. It reads one file.
- Untrusted input: trace files are untrusted. The loader returns errors instead of panicking; keep it that way. Fuzzing the loader is a candidate task, not done.

## Privacy

- Findings name attributes, span ids, trace ids and span names, never attribute values.
- MCP-PRIV-001 flags opt-in payload attributes so teams notice them.

## Documentation

- Decisions go in `docs/adr/` (MADR-style: status, context, decision, consequences, alternatives). A decision that reverses an ADR needs a new ADR.
- Every external claim cites URL and verification date.
- `docs/research/` is evidence with dates, not marketing. It is not edited retroactively; new findings are added as new dated entries.
- No marketing language and no claims of maturity. State what exists and what was tested.
- `docs/06-architecture.md` must match the code; update it in the same change.

## Community

- Conventional Commits.
- Plain commits. When an AI co-authors, a `Co-Authored-By` trailer records it. No DCO sign-off and no CLA: Apache-2.0 inbound = outbound is enough at this size. Revisit if the project joins a foundation.
- Issues: templates for bug, new rule, new SDK scenario.
- Code of conduct: `CODE_OF_CONDUCT.md`.

## Testing

- Rule tests are fixture-driven: a real trace in, a set of rule ids out. A rule that never fires on a fixture is unverified.
- Fixtures come from `make fixtures`, not from hand edits. The exception is a synthetic span in a unit test for a rule the harness cannot trigger yet; say so in the test.
- The healthy fixture is the false-positive guard: any new rule must leave it clean.
- Run with `-race -count=1` so cached results never hide a failure.
- Gaps today: four rules have no live fixture, the CLI has no end-to-end test of exit codes, and the loader has no fuzz test.

## Review checklist

For every PR, the reviewer (human or AI) checks:
1. Is each new FAIL backed by a quoted MUST or Required?
2. Is there span-id evidence, and no attribute values in the message?
3. Do the fixture and the test row exist and agree?
4. Do `docs/06-architecture.md`, the README table, and `CHANGELOG.md` match the code?
5. Is any new dependency justified, and is `go.sum` change explained?
6. Could a crafted trace file make the loader panic or allocate without bound?

## Definition of done

`make check` green; fixture and test row for rule changes; docs and changelog updated; diff self-reviewed. The same list is in `AGENTS.md` for agents.

## Reproducibility

- Pin tool versions in the Makefile, not in prose.
- Record versions in docs when reporting results (Go, go-sdk, pdata, collector image). `docs/04-technical-feasibility.md` "PoC results" is the model.
- A claim about a third-party library needs a command or trace that reproduces it.

## Release (later)

Not done. Planned for M1/M2 (`docs/07-roadmap.md`):
- goreleaser for cross-platform binaries.
- SHA-256 checksums file.
- SBOM (SPDX or CycloneDX) attached to releases.
- Optionally signed artifacts.
- Install instructions in the README once the first tag exists.
