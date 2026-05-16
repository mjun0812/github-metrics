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
LEFTHOOK_VERSION      := latest

.PHONY: all build test test-chromedp test-heavy test-race lint vet bench gen \
        gen-octicons verify-octicons docker e2e \
        tools hooks-install hooks-run hooks-uninstall \
        check-compat sync-assets clean help

all: build

help:
	@echo "Targets:"
	@echo "  build               Build cmd/metrics-action and cmd/metrics-cli into bin/"
	@echo "  test                Run unit tests (go test ./...)"
	@echo "  test-chromedp       Run chromedp-tagged tests (requires chromium; set METRICS_CHROME_PATH)"
	@echo "  test-heavy          Run heavy-tagged tests (M4 languages.recent / languages.indepth)"
	@echo "  test-race           Run tests with the race detector"
	@echo "  vet                 Run go vet ./..."
	@echo "  lint                Run golangci-lint and govulncheck"
	@echo "  bench               Run benchmarks (go test -bench=. -run=^$$)"
	@echo "  gen                 Run code generation (go generate ./...)"
	@echo "  gen-octicons        Regenerate assets/octicons/data.json from @primer/octicons"
	@echo "  verify-octicons     Ensure committed assets/octicons/data.json matches gen-octicons output"
	@echo "  docker              Build the production Docker image (placeholder; T-126 owns the impl)"
	@echo "  e2e                 Run end-to-end integration tests (placeholder; T122 owns the impl)"
	@echo "  tools               Install developer tooling (golangci-lint, govulncheck, gofumpt, lefthook)"
	@echo "  hooks-install       Wire the lefthook git hooks (run once per checkout after \`make tools\`)"
	@echo "  hooks-run           Run every pre-commit hook over the whole tree"
	@echo "  hooks-uninstall     Remove the lefthook git hooks"
	@echo "  check-compat        Diff metadata keys against ./org_repo upstream (placeholder; T058 owns the impl)"
	@echo "  sync-assets         Sync assets/ from ./org_repo (placeholder until T024 lands)"
	@echo "  clean               Remove bin/ and other build artifacts"

build: $(addprefix $(BIN_DIR)/, $(BINARIES))

$(BIN_DIR)/%: cmd/%/main.go
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" $(GOFLAGS) -o $@ ./cmd/$*

test:
	$(GO) test ./...

# Runs the chromedp-tagged tests (svg.Resize, Browser lifecycle, etc).
# Requires a chromium binary; set METRICS_CHROME_PATH or rely on PATH
# auto-detection. Default `make test` deliberately skips these so
# contributors without chromium installed stay green.
test-chromedp:
	$(GO) test -tags=chromedp ./...

# Runs the heavy-tagged tests (M4 languages.recent / languages.indepth).
# These tests depend on go-enry's embedded language DB and go-git's
# pure-Go shallow clone, both of which add wall time / I/O overhead.
# Default `make test` skips them; CI runs them as a separate parallel
# job. See specs/004-m4-github-plugins/quickstart.md §3.
test-heavy:
	$(GO) test -tags=heavy ./...

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
	$(GO) run ./internal/tools/gen-graphql

# Regenerate assets/octicons/data.json from the npm-installed
# @primer/octicons build/data.json. Requires `npm install --no-save
# @primer/octicons` to have populated node_modules/ beforehand.
gen-octicons:
	$(GO) run ./internal/tools/gen-octicons \
	  -in node_modules/@primer/octicons/build/data.json \
	  -out assets/octicons/data.json

# Verify the committed octicons asset is byte-identical to what
# gen-octicons produces from the current upstream. Used in CI to catch
# stale artifacts when @primer/octicons gets bumped.
verify-octicons: gen-octicons
	git diff --exit-code assets/octicons/data.json

docker:
	@echo "docker target placeholder - implemented in T-126 (M10)"

e2e:
	@echo "e2e target placeholder - implemented in T122 (M9)"

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	$(GO) install github.com/evilmartians/lefthook@$(LEFTHOOK_VERSION)

hooks-install:
	@if ! command -v lefthook >/dev/null 2>&1; then \
		echo "lefthook not found on PATH. Run 'make tools' first."; \
		exit 1; \
	fi
	lefthook install

hooks-run:
	lefthook run pre-commit --all-files

hooks-uninstall:
	@if command -v lefthook >/dev/null 2>&1; then \
		lefthook uninstall; \
	else \
		echo "lefthook not on PATH; nothing to uninstall."; \
	fi

check-compat:
	$(GO) run ./internal/tools/check-compat

check-output-compat:
	$(GO) test ./tests/compatibility/...

sync-assets:
	./scripts/sync-assets.sh

sync-fixtures:
	$(GO) run ./internal/tools/sync-fixtures --user octocat

clean:
	rm -rf $(BIN_DIR)
