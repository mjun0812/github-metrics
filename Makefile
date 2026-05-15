# github-metrics Makefile
#
# Targets are referenced by the project plan
# (specs/001-project-foundation/plan.md) and CI workflow
# (.github/workflows/go-ci.yml).

SHELL := /bin/bash

GO        ?= go
GOFLAGS   ?=
BIN_DIR   := bin
LDFLAGS   := -s -w -X main.version=$(shell git describe --tags --dirty --always 2>/dev/null || echo dev)

BINARIES := metrics-action metrics-cli

# Pin developer tooling so `make tools` produces a reproducible local
# environment matching CI. Bump these together with the CI workflow.
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION   := latest
GOFUMPT_VERSION       := latest

.PHONY: all build test test-race lint vet bench gen docker e2e \
        tools pre-commit-install pre-commit-run \
        check-compat sync-assets clean help

all: build

help:
	@echo "Targets:"
	@echo "  build               Build cmd/metrics-action and cmd/metrics-cli into bin/"
	@echo "  test                Run unit tests (go test ./...)"
	@echo "  test-race           Run tests with the race detector"
	@echo "  vet                 Run go vet ./..."
	@echo "  lint                Run golangci-lint and govulncheck"
	@echo "  bench               Run benchmarks (go test -bench=. -run=^$$)"
	@echo "  gen                 Run code generation (go generate ./...)"
	@echo "  docker              Build the production Docker image (placeholder; T-126 owns the impl)"
	@echo "  e2e                 Run end-to-end integration tests (placeholder; T122 owns the impl)"
	@echo "  tools               Install developer tooling (golangci-lint, govulncheck, gofumpt)"
	@echo "  pre-commit-install  Wire the pre-commit git hook (requires \`pre-commit\` on PATH)"
	@echo "  pre-commit-run      Run every pre-commit hook over the whole tree"
	@echo "  check-compat        Diff metadata keys against ./org_repo upstream (placeholder; T058 owns the impl)"
	@echo "  sync-assets         Sync assets/ from ./org_repo (placeholder until T024 lands)"
	@echo "  clean               Remove bin/ and other build artifacts"

build: $(addprefix $(BIN_DIR)/, $(BINARIES))

$(BIN_DIR)/%: cmd/%/main.go
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" $(GOFLAGS) -o $@ ./cmd/$*

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run --timeout=10m
	govulncheck ./...

bench:
	$(GO) test -bench=. -benchmem -run=^$$ ./...

gen:
	$(GO) generate ./...

docker:
	@echo "docker target placeholder - implemented in T-126 (M10)"

e2e:
	@echo "e2e target placeholder - implemented in T122 (M9)"

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)

pre-commit-install:
	@if ! command -v pre-commit >/dev/null 2>&1; then \
		echo "pre-commit not found on PATH."; \
		echo "Install with one of:"; \
		echo "  pipx install pre-commit"; \
		echo "  brew install pre-commit"; \
		echo "  pip install --user pre-commit"; \
		exit 1; \
	fi
	pre-commit install

pre-commit-run:
	pre-commit run --all-files

check-compat:
	@echo "check-compat placeholder - implemented in T058 (Phase 8)"

sync-assets:
	./scripts/sync-assets.sh

clean:
	rm -rf $(BIN_DIR)
