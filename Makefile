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

COLOR ?= auto
COLOR_ENABLED :=
ifeq ($(COLOR),always)
COLOR_ENABLED := 1
else ifeq ($(COLOR),never)
COLOR_ENABLED :=
else ifneq ($(NO_COLOR),)
COLOR_ENABLED :=
else ifneq ($(MAKE_TERMOUT),)
COLOR_ENABLED := 1
endif

ifeq ($(COLOR_ENABLED),1)
COLOR_STEP := \033[1;36m
COLOR_OK := \033[1;32m
COLOR_WARN := \033[1;33m
COLOR_FAIL := \033[1;31m
COLOR_TITLE := \033[1;37m
COLOR_TARGET := \033[36m
COLOR_DIM := \033[2m
COLOR_RESET := \033[0m
endif

PRINT_STEP = @printf '$(COLOR_STEP)==>$(COLOR_RESET) %s\n' '$(1)'
PRINT_OK = @printf '$(COLOR_OK)OK:$(COLOR_RESET) %s\n' '$(1)'

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
	@printf '$(COLOR_DIM)%s$(COLOR_RESET)\n' 'Running relaxed quality checks...'
	@$(MAKE) format || printf '$(COLOR_WARN)WARN:$(COLOR_RESET) %s\n' 'formatting failed'
	@$(MAKE) tidy-fix || printf '$(COLOR_WARN)WARN:$(COLOR_RESET) %s\n' 'go mod tidy failed'
	@$(MAKE) lint || printf '$(COLOR_WARN)WARN:$(COLOR_RESET) %s\n' 'lint checks failed'
	@$(MAKE) test || printf '$(COLOR_WARN)WARN:$(COLOR_RESET) %s\n' 'tests failed'
	@printf '$(COLOR_DIM)%s$(COLOR_RESET)\n' 'Relaxed quality checks completed'

.PHONY: lint
lint: ensure-hooks vet staticcheck golangci-lint errcheck ## Run static analysis checks

.PHONY: format
format: ensure-hooks fmt-fix imports-fix ## Format code with gofmt and goimports

.PHONY: format-check
format-check: ensure-hooks fmt-check imports-check ## Check formatting with gofmt and goimports

.PHONY: verify-modules
verify-modules: ensure-hooks ## Download and verify modules
	$(call PRINT_STEP,Module verification)
	@$(GO) mod download
	@$(GO) mod verify
	$(call PRINT_OK,go mod verified)

.PHONY: fmt-check
fmt-check: ensure-hooks ## Check gofmt formatting
	$(call PRINT_STEP,gofmt check)
	@if [ -n "$(GOFILES)" ]; then \
		issues="$$(gofmt -l -s $(GOFILES))"; \
		if [ -n "$$issues" ]; then \
			printf '$(COLOR_FAIL)FAIL:$(COLOR_RESET) %s\n' 'gofmt issues found (run make fmt-fix or make fix)' >&2; \
			printf '%s\n' "$$issues"; \
			exit 1; \
		fi; \
	fi
	$(call PRINT_OK,gofmt clean)

.PHONY: fmt-fix
fmt-fix: ensure-hooks ## Apply gofmt formatting
	$(call PRINT_STEP,gofmt fix)
	@if [ -n "$(GOFILES)" ]; then gofmt -w -s $(GOFILES); fi
	$(call PRINT_OK,gofmt applied)

.PHONY: imports-check
imports-check: ensure-hooks $(GOIMPORTS) ## Check goimports formatting
	$(call PRINT_STEP,goimports check)
	@if [ -n "$(GOFILES)" ]; then \
		issues="$$($(GOIMPORTS) -l $(GOFILES))"; \
		if [ -n "$$issues" ]; then \
			printf '$(COLOR_FAIL)FAIL:$(COLOR_RESET) %s\n' 'goimports issues found (run make imports-fix or make fix)' >&2; \
			printf '%s\n' "$$issues"; \
			exit 1; \
		fi; \
	fi
	$(call PRINT_OK,goimports clean)

.PHONY: imports-fix
imports-fix: ensure-hooks $(GOIMPORTS) ## Apply goimports formatting
	$(call PRINT_STEP,goimports fix)
	@if [ -n "$(GOFILES)" ]; then $(GOIMPORTS) -w $(GOFILES); fi
	$(call PRINT_OK,goimports applied)

.PHONY: tidy-check
tidy-check: ensure-hooks ## Check go.mod/go.sum tidiness
	$(call PRINT_STEP,go mod tidy check)
	@tmpdir="$$(mktemp -d "$${TMPDIR:-.}/jobscout-build.XXXXXX")"; \
		trap 'rm -rf "$$tmpdir"' EXIT; \
		cp go.mod "$$tmpdir/go.mod"; \
		if [ -f go.sum ]; then cp go.sum "$$tmpdir/go.sum"; fi; \
		$(GO) mod tidy; \
		if ! diff -q "$$tmpdir/go.mod" go.mod >/dev/null 2>&1 || \
			{ [ -f "$$tmpdir/go.sum" ] && ! diff -q "$$tmpdir/go.sum" go.sum >/dev/null 2>&1; }; then \
			printf '$(COLOR_FAIL)FAIL:$(COLOR_RESET) %s\n' 'go.mod/go.sum not tidy (run make tidy-fix or make fix)' >&2; \
			exit 1; \
		fi
	$(call PRINT_OK,go mod tidy clean)

.PHONY: tidy-fix
tidy-fix: ensure-hooks ## Apply go mod tidy
	$(call PRINT_STEP,go mod tidy fix)
	@$(GO) mod tidy
	$(call PRINT_OK,go mod tidy applied)

.PHONY: vet
vet: ensure-hooks ## Run go vet
	$(call PRINT_STEP,go vet)
	@$(GO) vet $(PKGS)
	$(call PRINT_OK,go vet passed)

.PHONY: staticcheck
staticcheck: ensure-hooks $(STATICCHECK) ## Run staticcheck
	$(call PRINT_STEP,staticcheck)
	@$(STATICCHECK) $(PKGS)
	$(call PRINT_OK,staticcheck passed)

.PHONY: golangci-lint
golangci-lint: ensure-hooks $(GOLANGCI_LINT) ## Run golangci-lint
	$(call PRINT_STEP,golangci-lint)
	@$(GOLANGCI_LINT) run
	$(call PRINT_OK,golangci-lint passed)

.PHONY: errcheck
errcheck: ensure-hooks $(ERRCHECK) ## Run errcheck
	$(call PRINT_STEP,errcheck)
	@$(ERRCHECK) $(PKGS)
	$(call PRINT_OK,errcheck passed)

.PHONY: gosec
gosec: ensure-hooks $(GOSEC) ## Run gosec
	$(call PRINT_STEP,gosec)
	@$(GOSEC) -exclude-dir=.dump -exclude-dir=.tools ./...
	$(call PRINT_OK,gosec passed)

.PHONY: govulncheck
govulncheck: ensure-hooks $(GOVULNCHECK) ## Run govulncheck
	$(call PRINT_STEP,govulncheck)
	@$(GOVULNCHECK) -test ./...
	$(call PRINT_OK,govulncheck passed)

.PHONY: test
test: ensure-hooks ## Run tests
	$(call PRINT_STEP,go test)
	@$(GO) test -timeout $(TIMEOUT) $(PKGS)
	$(call PRINT_OK,tests passed)

.PHONY: race
race: ensure-hooks ## Run race detector where supported
	$(call PRINT_STEP,go test -race)
	@if [ "$$($(GO) env GOOS)" = "android" ]; then \
		printf '$(COLOR_WARN)Skipping$(COLOR_RESET) race detector: unsupported on %s/%s\n' "$$($(GO) env GOOS)" "$$($(GO) env GOARCH)"; \
	else \
		$(GO) test -race -timeout $(TIMEOUT) $(PKGS); \
		printf '$(COLOR_OK)OK:$(COLOR_RESET) %s\n' 'race detector clean'; \
	fi

.PHONY: build
build: ensure-hooks ## Build the jobscout binary
	$(call PRINT_STEP,go build)
	@$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$(BIN)" "$(CMD_PATH)"
	@printf '$(COLOR_OK)OK:$(COLOR_RESET) Build successful: %s\n' "$(BIN)"
	@printf '\n$(COLOR_DIM)Run:$(COLOR_RESET) %s\n' "$(BIN)"

.PHONY: install
install: ensure-hooks ## Install jobscout into GOBIN or GOPATH/bin
	$(call PRINT_STEP,go install)
	@$(GO) install -trimpath -ldflags "$(LDFLAGS)" "$(CMD_PATH)"
	$(call PRINT_OK,jobscout installed)

.PHONY: release
release: ensure-hooks ## Build a versioned release archive for RELEASE_GOOS/RELEASE_GOARCH
	@printf '$(COLOR_STEP)==>$(COLOR_RESET) release %s %s/%s\n' "$(VERSION)" "$(RELEASE_GOOS)" "$(RELEASE_GOARCH)"
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
				printf '$(COLOR_FAIL)FAIL:$(COLOR_RESET) %s\n' 'zip is required for Windows release archives' >&2; \
				exit 1; \
			fi; \
			archive="$(DIST_DIR)/$$base.zip"; \
			(cd "$(DIST_DIR)" && zip -qr "$$base.zip" "$$base"); \
		else \
			archive="$(DIST_DIR)/$$base.tar.gz"; \
			tar -C "$(DIST_DIR)" -czf "$$archive" "$$base"; \
		fi; \
		printf '$(COLOR_OK)OK:$(COLOR_RESET) release archive: %s\n' "$$archive"

.PHONY: c4-diagram
c4-diagram: ensure-hooks ## Regenerate the docs C4 component diagram
	$(call PRINT_STEP,C4 diagram update)
	@scripts/update-c4-diagram.sh

.PHONY: c4-diagram-check
c4-diagram-check: ensure-hooks ## Check that the docs C4 component diagram is current
	$(call PRINT_STEP,C4 diagram check)
	@scripts/update-c4-diagram.sh --check

.PHONY: tools
tools: ensure-hooks $(GOIMPORTS) $(STATICCHECK) $(GOLANGCI_LINT) $(ERRCHECK) $(GOSEC) $(GOVULNCHECK) ## Install local toolchain helpers

$(TOOLS_BIN):
	@mkdir -p "$@"

$(TOOLS_BIN)/goimports: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing goimports)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOIMPORTS_PKG)

$(TOOLS_BIN)/staticcheck: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing staticcheck)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(STATICCHECK_PKG)

$(TOOLS_BIN)/golangci-lint: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing golangci-lint)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOLANGCI_LINT_PKG)

$(TOOLS_BIN)/errcheck: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing errcheck)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(ERRCHECK_PKG)

$(TOOLS_BIN)/gosec: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing gosec)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOSEC_PKG)

$(TOOLS_BIN)/govulncheck: | $(TOOLS_BIN)
	$(call PRINT_STEP,installing govulncheck)
	@env GOBIN="$(TOOLS_BIN)" $(GO) install $(GOVULNCHECK_PKG)

.PHONY: ensure-hooks
ensure-hooks:
	@scripts/git-hooks/install-pre-commit.sh

.PHONY: clean
clean: ## Remove local build artifacts
	$(call PRINT_STEP,clean)
	@rm -rf "$(TOOLS_BIN)" "$(BIN)" "$(DIST_DIR)"
	$(call PRINT_OK,clean complete)

.PHONY: help
help: ## Show Makefile targets
	@printf '$(COLOR_TITLE)%s$(COLOR_RESET)\n' 'JobScout Development Commands'
	@printf '$(COLOR_DIM)%s$(COLOR_RESET)\n' '=============================='
	@awk -v c='$(COLOR_TARGET)' -v r='$(COLOR_RESET)' 'BEGIN {FS = ":.*## "}; /^[A-Za-z0-9_.-]+:.*## / {printf "%s%-20s%s %s\n", c, $$1, r, $$2}' $(MAKEFILE_LIST) | sort
