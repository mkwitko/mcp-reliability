GO ?= go
GOVULNCHECK_VERSION ?= v1.9.0
STATICCHECK_VERSION ?= 2026.2.2

.PHONY: all fmt vet test lint vuln check fixtures build

all: check

build:
	$(GO) build ./...

fmt:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -count=1 ./...

lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

check: fmt vet test lint vuln

# Regenerates testdata/*.jsonl from a live go-sdk client/server. Needs Docker.
fixtures:
	./examples/run-scenarios.sh
