# Contributing

The project is pre-alpha. Expect churn. Read `docs/` first, especially `docs/06-architecture.md` and `docs/07-roadmap.md`. New to the code? `docs/11-code-tour.md` walks one `doctor` run file by file.

## Ways to help
- **New rule**: open a "New rule" issue first. A rule must quote the MCP spec or OTel semconv text it enforces. FAIL needs a MUST or a Required attribute; never report a violation the standard does not require.
- **New SDK scenario**: an example client+server under `examples/` that reproduces a propagation or attribute problem, with a fixture and expected findings.
- **Fixtures from real traces**: redacted `.jsonl` in `testdata/`. Strip secrets, hostnames, and user data. Say which library and version produced it.
- **Bug reports**: include the command, version, and a minimal redacted trace.

## Workflow
1. Fork, branch, keep the change small.
2. Commits follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
3. Run `make check` (gofmt, vet, test -race, staticcheck, govulncheck). `make test` for tests only; `make fixtures` regenerates `testdata/` (needs Docker).
4. Update `CHANGELOG.md` under Unreleased.
5. Open a PR using the template.

## Rule checklist
- Quote the normative text (MCP spec or OTel semconv) in the code comment, with URL.
- FAIL only for MUST / Required; WARN for SHOULD, Conditionally Required, sensitive data; INFO for ambiguous evidence.
- A fixture that triggers the rule, a row in `TestFixtures` (`internal/rules/rules_test.go`), and the healthy fixture still clean.
- Findings cite span ids and never echo attribute values.
- Update the catalogue in `docs/06-architecture.md` and the README table.

## License
Contributions are licensed under Apache-2.0 (see `LICENSE`). Sign-off is not required yet; this may change (see `docs/08-community-strategy.md`).

By participating you agree to the `CODE_OF_CONDUCT.md`. Security issues: see `SECURITY.md`.
