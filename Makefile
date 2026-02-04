# Makefile for go-sdk
#
# Aggregates script modules from scripts/ (formatter, linter, test, security, deps, build).
# Default goal: help (calls each script's help-* target). Use make check for CI; make install-tools to install tools.

.PHONY: help help-formatter help-linter help-test help-security help-deps help-build
.PHONY: format format-check install-formatter lint lint-fix install-linter
.PHONY: test test-unit test-integration test-race bench coverage coverage-view
.PHONY: vulncheck install-govulncheck deps-tidy deps-verify deps deps-outdated deps-upgrade
.PHONY: build clean generate check ci install-tools

.DEFAULT_GOAL := help

SCRIPTS_DIR := ./scripts

# Suppress "Entering/Leaving directory" when invoking sub-make (help, ci, install-tools)
MAKE := $(MAKE) --no-print-directory

include $(SCRIPTS_DIR)/vars.mk
include $(SCRIPTS_DIR)/formatter.mk
include $(SCRIPTS_DIR)/linter.mk
include $(SCRIPTS_DIR)/test.mk
include $(SCRIPTS_DIR)/security.mk
include $(SCRIPTS_DIR)/deps.mk
include $(SCRIPTS_DIR)/build.mk

help: ## Show all targets (aggregates help from each script)
	@echo ">>>> Go-SDK Makefile targets <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) -s help-formatter
	$(ECHO_EMPTY)
	@$(MAKE) -s help-linter
	$(ECHO_EMPTY)
	@$(MAKE) -s help-test
	$(ECHO_EMPTY)
	@$(MAKE) -s help-security
	$(ECHO_EMPTY)
	@$(MAKE) -s help-deps
	$(ECHO_EMPTY)
	@$(MAKE) -s help-build
	$(ECHO_EMPTY)
	@echo "# Other"
	@echo "  make check         ## Run format-check, lint, test, coverage, vulncheck, deps-verify (CI)"
	@echo "  make install-tools ## Install formatter, linter, govulncheck"

check: ci ## Alias for ci

ci: ## Run all checks (format-check, lint, test, coverage, vulncheck, deps-verify); fail-fast
	@echo ">>>> CI <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) format
	@$(MAKE) lint-fix
	@$(MAKE) test-unit
	@$(MAKE) coverage
	@$(MAKE) deps-verify
	$(ECHO_EMPTY)
	@echo "[OK] CI COMPLETED SUCCESSFULLY"

install-tools: ## Install formatter, linter, govulncheck; fail-fast
	@echo ">>>> Install tools <<<<"
	$(ECHO_EMPTY)
	@$(MAKE) install-formatter
	@$(MAKE) install-linter
	@$(MAKE) install-govulncheck
	$(ECHO_EMPTY)
	@echo "[OK] Install tools completed successfully"
