SHELL := /bin/sh

.DEFAULT_GOAL := help

BIN ?= jobscout
CMD_PATH ?= ./cmd/jobscout
GO ?= go
TIMEOUT ?= 300s
TOOLS_BIN ?= $(CURDIR)/.tools/bin
MODULE_PATH := github.com/wallentx/jobscout
LATEST_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null || printf '%s' v0.0.0)
GIT_SHORT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf '%s' unknown)
DEV_VERSION ?= $(LATEST_TAG)-$(GIT_SHORT_SHA)
VERSION ?= $(shell if [ -n "$$RELEASE_TAG" ]; then printf '%s' "$$RELEASE_TAG"; else printf '%s' "$(DEV_VERSION)"; fi)
DIST_DIR ?= dist
RELEASE_GOOS ?= $(shell $(GO) env GOOS)
RELEASE_GOARCH ?= $(shell $(GO) env GOARCH)
LDFLAGS ?= -s -w -X $(MODULE_PATH)/internal/jobscout.version=$(VERSION)

PKGS := $(shell $(GO) list ./...)
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*' -not -path './.dump/*' -not -path './.tools/*')

GOIMPORTS := $(shell command -v goimports 2>/dev/null || printf '%s' '$(TOOLS_BIN)/goimports')
STATICCHECK := $(shell command -v staticcheck 2>/dev/null || printf '%s' '$(TOOLS_BIN)/staticcheck')
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || printf '%s' '$(TOOLS_BIN)/golangci-lint')
ERRCHECK := $(shell command -v errcheck 2>/dev/null || printf '%s' '$(TOOLS_BIN)/errcheck')
GOSEC := $(shell command -v gosec 2>/dev/null || printf '%s' '$(TOOLS_BIN)/gosec')
GOVULNCHECK := $(shell command -v govulncheck 2>/dev/null || printf '%s' '$(TOOLS_BIN)/govulncheck')

GOIMPORTS_PKG := golang.org/x/tools/cmd/goimports@v0.42.0
STATICCHECK_PKG := honnef.co/go/tools/cmd/staticcheck@v0.7.0
GOLANGCI_LINT_PKG := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
ERRCHECK_PKG := github.com/kisielk/errcheck@v1.10.0
GOSEC_PKG := github.com/securego/gosec/v2/cmd/gosec@v2.24.6
GOVULNCHECK_PKG := golang.org/x/vuln/cmd/govulncheck@v1.3.0

.PHONY: all
all: ensure-hooks ## Apply mechanical fixes, then run the standard local release gate
	@$(MAKE) --no-print-directory fix
	@$(MAKE) --no-print-directory check
	@scripts/git-hooks/stamp-checks.sh all

.PHONY: check
check: ensure-hooks verify-modules format-check tidy-check lint test race build ## Run the standard local release gate
	@scripts/git-hooks/stamp-checks.sh check

.PHONY: full-check
full-check: ensure-hooks check security ## Run the standard gate plus security scanners
	@scripts/git-hooks/stamp-checks.sh full-check

.PHONY: security
security: ensure-hooks gosec govulncheck ## Run heavier security scanners

.PHONY: fix
fix: ensure-hooks format tidy-fix ## Apply mechanical formatting/import/module fixes
	@scripts/git-hooks/stamp-checks.sh fix

.PHONY: qa
qa: ensure-hooks format-check tidy-check lint ## Run strict quality checks without tests or build

.PHONY: qa-simple
qa-simple: ensure-hooks format test ## Run basic quality checks with auto-formatting

.PHONY: qa-relaxed
qa-relaxed: ensure-hooks ## Run quality checks and report failures without stopping
	@printf '%s\n' 'Running relaxed quality checks...'
	@$(MAKE) format || printf '%s\n' 'WARN: formatting failed'
	@$(MAKE) tidy-fix || printf '%s\n' 'WARN: go mod tidy failed'
	@$(MAKE) lint || printf '%s\n' 'WARN: lint checks failed'
	@$(MAKE) test || printf '%s\n' 'WARN: tests failed'
	@printf '%s\n' 'Relaxed quality checks completed'

.PHONY: lint
lint: ensure-hooks vet staticcheck golangci-lint errcheck ## Run static analysis checks

.PHONY: format
format: ensure-hooks fmt-fix imports-fix ## Format code with gofmt and goimports

.PHONY: format-check
format-check: ensure-hooks fmt-check imports-check ## Check formatting with gofmt and goimports

.PHONY: verify-modules
verify-modules: ensure-hooks ## Download and verify modules
	@printf '%s\n' '==> Module verification'
	@$(GO) mod download
	@$(GO) mod verify
	@printf '%s\n' 'OK: go mod verified'

.PHONY: fmt-check
fmt-check: ensure-hooks ## Check gofmt formatting
	@printf '%s\n' '==> gofmt check'
	@if [ -n "$(GOFILES)" ]; then \
		issues="$$(gofmt -l -s $(GOFILES))"; \
		if [ -n "$$issues" ]; then \
			printf '%s\n' 'FAIL: gofmt issues found (run make fmt-fix or make fix)' >&2; \
			printf '%s\n' "$$issues"; \
			exit 1; \
		fi; \
	fi
	@printf '%s\n' 'OK: gofmt clean'

.PHONY: fmt-fix
fmt-fix: ensure-hooks ## Apply gofmt formatting
	@printf '%s\n' '==> gofmt fix'
	@if [ -n "$(GOFILES)" ]; then gofmt -w -s $(GOFILES); fi
	@printf '%s\n' 'OK: gofmt applied'

.PHONY: imports-check
imports-check: ensure-hooks $(GOIMPORTS) ## Check goimports formatting
	@printf '%s\n' '==> goimports check'
	@if [ -n "$(GOFILES)" ]; then \
		issues="$$($(GOIMPORTS) -l $(GOFILES))"; \
		if [ -n "$$issues" ]; then \
			printf '%s\n' 'FAIL: goimports issues found (run make imports-fix or make fix)' >&2; \
			printf '%s\n' "$$issues"; \
			exit 1; \
		fi; \
	fi
	@printf '%s\n' 'OK: goimports clean'

.PHONY: imports-fix
imports-fix: ensure-hooks $(GOIMPORTS) ## Apply goimports formatting
	@printf '%s\n' '==> goimports fix'
	@if [ -n "$(GOFILES)" ]; then $(GOIMPORTS) -w $(GOFILES); fi
	@printf '%s\n' 'OK: goimports applied'

.PHONY: tidy-check
tidy-check: ensure-hooks ## Check go.mod/go.sum tidiness
	@printf '%s\n' '==> go mod tidy check'
	@tmpdir="$$(mktemp -d "$${TMPDIR:-.}/jobscout-build.XXXXXX")"; \
		trap 'rm -rf "$$tmpdir"' EXIT; \
		cp go.mod "$$tmpdir/go.mod"; \
		if [ -f go.sum ]; then cp go.sum "$$tmpdir/go.sum"; fi; \
		$(GO) mod tidy; \
		if ! diff -q "$$tmpdir/go.mod" go.mod >/dev/null 2>&1 || \
			{ [ -f "$$tmpdir/go.sum" ] && ! diff -q "$$tmpdir/go.sum" go.sum >/dev/null 2>&1; }; then \
			printf '%s\n' 'FAIL: go.mod/go.sum not tidy (run make tidy-fix or make fix)' >&2; \
			exit 1; \
		fi
	@printf '%s\n' 'OK: go mod tidy clean'

.PHONY: tidy-fix
tidy-fix: ensure-hooks ## Apply go mod tidy
	@printf '%s\n' '==> go mod tidy fix'
	@$(GO) mod tidy
	@printf '%s\n' 'OK: go mod tidy applied'

.PHONY: vet
vet: ensure-hooks ## Run go vet
	@printf '%s\n' '==> go vet'
	@$(GO) vet $(PKGS)
	@printf '%s\n' 'OK: go vet passed'

.PHONY: staticcheck
staticcheck: ensure-hooks $(STATICCHECK) ## Run staticcheck
	@printf '%s\n' '==> staticcheck'
	@$(STATICCHECK) $(PKGS)
	@printf '%s\n' 'OK: staticcheck passed'

.PHONY: golangci-lint
golangci-lint: ensure-hooks $(GOLANGCI_LINT) ## Run golangci-lint
	@printf '%s\n' '==> golangci-lint'
	@$(GOLANGCI_LINT) run
	@printf '%s\n' 'OK: golangci-lint passed'

.PHONY: errcheck
errcheck: ensure-hooks $(ERRCHECK) ## Run errcheck
	@printf '%s\n' '==> errcheck'
	@$(ERRCHECK) $(PKGS)
	@printf '%s\n' 'OK: errcheck passed'

.PHONY: gosec
gosec: ensure-hooks $(GOSEC) ## Run gosec
	@printf '%s\n' '==> gosec'
	@$(GOSEC) -exclude-dir=.dump -exclude-dir=.tools ./...
	@printf '%s\n' 'OK: gosec passed'

.PHONY: govulncheck
govulncheck: ensure-hooks $(GOVULNCHECK) ## Run govulncheck
	@printf '%s\n' '==> govulncheck'
	@$(GOVULNCHECK) -test ./...
	@printf '%s\n' 'OK: govulncheck passed'

.PHONY: test
test: ensure-hooks ## Run tests
	@printf '%s\n' '==> go test'
	@$(GO) test -timeout $(TIMEOUT) $(PKGS)
	@printf '%s\n' 'OK: tests passed'

.PHONY: race
race: ensure-hooks ## Run race detector where supported
	@printf '%s\n' '==> go test -race'
	@if [ "$$($(GO) env GOOS)" = "android" ]; then \
		printf 'Skipping race detector: unsupported on %s/%s\n' "$$($(GO) env GOOS)" "$$($(GO) env GOARCH)"; \
	else \
		$(GO) test -race -timeout $(TIMEOUT) $(PKGS); \
		printf '%s\n' 'OK: race detector clean'; \
	fi

.PHONY: build
build: ensure-hooks ## Build the jobscout binary
	@printf '%s\n' '==> go build'
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$(BIN)" "$(CMD_PATH)"
	@printf 'OK: Build successful: %s\n' "$(BIN)"
	@printf '\nRun: %s\n' "$(BIN)"

.PHONY: install
install: ensure-hooks ## Install jobscout into GOBIN or GOPATH/bin
	@printf '%s\n' '==> go install'
	@$(GO) install -trimpath -ldflags "$(LDFLAGS)" "$(CMD_PATH)"
	@printf '%s\n' 'OK: jobscout installed'

.PHONY: release
release: ensure-hooks ## Build a versioned release archive for RELEASE_GOOS/RELEASE_GOARCH
	@printf '==> release %s %s/%s\n' "$(VERSION)" "$(RELEASE_GOOS)" "$(RELEASE_GOARCH)"
	@set -eu; \
		base="jobscout_$(VERSION)_$(RELEASE_GOOS)_$(RELEASE_GOARCH)"; \
		outdir="$(DIST_DIR)/$$base"; \
		bin="$$outdir/jobscout"; \
		if [ "$(RELEASE_GOOS)" = "windows" ]; then bin="$$bin.exe"; fi; \
		rm -rf "$$outdir" "$(DIST_DIR)/$$base.tar.gz" "$(DIST_DIR)/$$base.zip"; \
		mkdir -p "$$outdir"; \
		GOOS="$(RELEASE_GOOS)" GOARCH="$(RELEASE_GOARCH)" CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$$bin" "$(CMD_PATH)"; \
		cp README.md LICENSE "$$outdir/"; \
		if [ "$(RELEASE_GOOS)" = "windows" ]; then \
			if ! command -v zip >/dev/null 2>&1; then \
				printf '%s\n' 'FAIL: zip is required for Windows release archives' >&2; \
				exit 1; \
			fi; \
			archive="$(DIST_DIR)/$$base.zip"; \
			(cd "$(DIST_DIR)" && zip -qr "$$base.zip" "$$base"); \
		else \
			archive="$(DIST_DIR)/$$base.tar.gz"; \
			tar -C "$(DIST_DIR)" -czf "$$archive" "$$base"; \
		fi; \
		printf 'OK: release archive: %s\n' "$$archive"

.PHONY: c4-diagram
c4-diagram: ensure-hooks ## Regenerate the docs C4 component diagram
	@printf '%s\n' '==> C4 diagram update'
	@scripts/update-c4-diagram.sh

.PHONY: c4-diagram-check
c4-diagram-check: ensure-hooks ## Check that the docs C4 component diagram is current
	@printf '%s\n' '==> C4 diagram check'
	@scripts/update-c4-diagram.sh --check

.PHONY: tools
tools: ensure-hooks $(GOIMPORTS) $(STATICCHECK) $(GOLANGCI_LINT) $(ERRCHECK) $(GOSEC) $(GOVULNCHECK) ## Install local toolchain helpers

$(TOOLS_BIN):
	@mkdir -p "$@"

$(TOOLS_BIN)/goimports: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing goimports'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOIMPORTS_PKG)

$(TOOLS_BIN)/staticcheck: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing staticcheck'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(STATICCHECK_PKG)

$(TOOLS_BIN)/golangci-lint: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing golangci-lint'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOLANGCI_LINT_PKG)

$(TOOLS_BIN)/errcheck: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing errcheck'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(ERRCHECK_PKG)

$(TOOLS_BIN)/gosec: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing gosec'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOSEC_PKG)

$(TOOLS_BIN)/govulncheck: | $(TOOLS_BIN)
	@printf '%s\n' '==> installing govulncheck'
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOVULNCHECK_PKG)

.PHONY: ensure-hooks
ensure-hooks:
	@scripts/git-hooks/install-pre-commit.sh

.PHONY: clean
clean: ## Remove local build artifacts
	@printf '%s\n' '==> clean'
	@rm -rf "$(TOOLS_BIN)" "$(BIN)" "$(DIST_DIR)"
	@printf '%s\n' 'OK: clean complete'

.PHONY: help
help: ## Show Makefile targets
	@printf '%s\n' 'JobScout Development Commands'
	@printf '%s\n' '=============================='
	@awk 'BEGIN {FS = ":.*## "}; /^[A-Za-z0-9_.-]+:.*## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
