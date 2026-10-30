## What and why

## Checklist
- [ ] `gofmt -l .` prints nothing
- [ ] `go vet ./...` and `go test -race ./...` pass
- [ ] New or changed rule: cites the spec/semconv text it enforces (quoted, with version) and has a passing and a failing fixture
- [ ] No real secrets or customer data in fixtures
- [ ] `CHANGELOG.md` updated under Unreleased
