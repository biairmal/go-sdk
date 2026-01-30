# vars.mk - Common variables for cross-platform Makefile scripts
#
# Defines: GO_OS, GOPATH, GOPATH_BIN, PATH_SEP, BIN_EXT for paths and binaries.
# Defines output convention variables: ECHO_EMPTY, INDENT, PREFIX_RUN, PREFIX_OK,
# PREFIX_FAIL, PREFIX_SKIP for consistent script output (see section 1b in plan).
# No user-facing targets; include this file first in script modules that need it.
#
# Conventions: All scripts are fail-fast (no Make '-' prefix). Combined targets
# (ci, install-tools) rely on Make stopping at the first failing prerequisite.

GO_OS := $(shell go env GOOS)
GOPATH := $(shell go env GOPATH)

ifeq ($(GO_OS),windows)
	PATH_SEP := \\
	BIN_EXT := .exe
else
	PATH_SEP := /
	BIN_EXT :=
endif

GOPATH_BIN := $(GOPATH)$(PATH_SEP)bin

# Output convention (section 1b): top-level >>>> TITLE <<<<, section # Name, 2-space indent
ECHO_EMPTY := @echo ""
EMPTY :=
SPACE := $(EMPTY)  $(EMPTY)
INDENT := $(SPACE)
PREFIX_RUN := [RUN] 
PREFIX_OK := [OK] 
PREFIX_FAIL := [FAIL] 
PREFIX_SKIP := [SKIP] 
