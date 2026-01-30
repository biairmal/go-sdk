# linter.mk - Golangci-lint targets (install, run, fix) and help
#
# Provides: install-linter, lint, lint-fix, help-linter.
# Install: uses go install (works in PowerShell and other shells).
# Default version: latest (v1.61.0 can fail on Go 1.25 with golang.org/x/tools tokeninternal error).
# Override with GOLANGCI_LINT_VERSION=v1.xx if you need a pinned version.
# Prerequisite: include vars.mk first (for GOPATH_BIN, PATH_SEP, BIN_EXT, output vars).
# Config: root .golangci.yml. Output convention: section "# Linter", body $(INDENT)$(PREFIX_*).
# Fail-fast (1c): do not use '-' prefix; command failures propagate so Make stops.

include $(SCRIPTS_DIR)/vars.mk

GOLANGCI_LINT_VERSION ?= latest
GOLANGCI_LINT_BIN := $(GOPATH_BIN)$(PATH_SEP)golangci-lint$(BIN_EXT)

install-linter: ## Install golangci-lint into GOPATH/bin (go install)
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Installing golangci-lint@$(GOLANGCI_LINT_VERSION)..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "$(INDENT)$(PREFIX_OK)golangci-lint installed successfully"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

lint: ## Run golangci-lint (config: .golangci.yml)
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running golangci-lint..."
	@$(GOLANGCI_LINT_BIN) run ./...
	@echo "$(INDENT)$(PREFIX_OK)Lint passed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

lint-fix: ## Run golangci-lint with --fix
	$(ECHO_EMPTY)
	@echo "# Linter"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running golangci-lint --fix..."
	@$(GOLANGCI_LINT_BIN) run --fix ./...
	@echo "$(INDENT)$(PREFIX_OK)Lint fix completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-linter: ## Show linter targets and descriptions
	@echo "# Linter"
	@echo "  make install-linter  ## Install golangci-lint into GOPATH/bin (go install)"
	@echo "  make lint            ## Run golangci-lint (config: .golangci.yml)"
	@echo "  make lint-fix        ## Run golangci-lint with --fix"
