# build.mk - Build, clean, and generate targets
#
# Provides: build, clean, generate, help-build.
# Prerequisite: include vars.mk first (for output vars). Output convention: section "# Build" / "# Clean" / "# Generate".
# Fail-fast (1c): no '-' prefix for build and generate. clean is idempotent (do not fail if out/ missing).

include $(SCRIPTS_DIR)/vars.mk

OUT_DIR ?= out

build: ## Verify package compiles (go build ./...)
	$(ECHO_EMPTY)
	@echo "# Build"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Building..."
	@go build ./...
	@echo "$(INDENT)$(PREFIX_OK)Build succeeded"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

clean: ## Remove artefacts (out/, coverage files)
	$(ECHO_EMPTY)
	@echo "# Clean"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Removing artefacts..."
	@rm -rf $(OUT_DIR)
	@echo "$(INDENT)$(PREFIX_OK)Clean completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

generate: ## Run go generate ./...
	$(ECHO_EMPTY)
	@echo "# Generate"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_RUN)Running go generate ./..."
	@go generate ./...
	@echo "$(INDENT)$(PREFIX_OK)Generate completed"
	$(ECHO_EMPTY)
	@echo "$(INDENT)$(PREFIX_OK)PROCESS COMPLETED SUCCESSFULLY"

help-build: ## Show build targets and descriptions
	@echo "# Build"
	@echo "  make build    ## Verify package compiles (go build ./...)"
	@echo "  make clean    ## Remove artefacts (out/, coverage files)"
	@echo "  make generate ## Run go generate ./..."
