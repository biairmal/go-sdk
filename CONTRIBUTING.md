# Contributing

Thanks for working on `github.com/biairmal/go-sdk`. This page covers human onboarding and workflow. The **authoritative coding rules** — for both humans and AI assistants — live in **[AGENTS.md](./AGENTS.md)**; read it before you start.

## Prerequisites

| Requirement | Purpose |
|---|---|
| **Go 1.25.1+** | Build and test. Check with `go version`. |
| **Make** | Format, lint, test, coverage, tool install. On Windows use Git Bash, WSL, or GnuWin32 Make. |

Optional tools (installed on demand):

```bash
make install-tools   # gofumpt, golangci-lint, govulncheck
```

## Development loop

1. Make your change in the relevant sub-package (the SDK has no required entry point — packages are independent).
2. Follow the [Authoring rules](AGENTS.md#authoring-rules).
3. Add **table-driven tests** alongside the change.
4. Run the CI gate and make sure it is green:

   ```bash
   make check          # format → lint-fix → test-unit → coverage → vulncheck → deps-verify
   ```

5. Walk the [Definition of Done](AGENTS.md#definition-of-done) before opening a PR.

Adding a new sub-package? Follow [docs/NEW_PACKAGE_CHECKLIST.md](docs/NEW_PACKAGE_CHECKLIST.md).

## Tests

- **Unit:** `make test-unit` → `go test -short ./...`. Guard slow/integration-only code with `testing.Short()`.
- **Integration (live services, e.g. Redis):** `make test-integration` → `go test ./...`. Put these in `*_integration_test.go`.
- **Coverage:** `make coverage` writes `out/coverage.{out,html}`; `make coverage-view` opens the report.

## Pull requests

- Keep PRs scoped to one concern; update the relevant `README.md` and the [package map](AGENTS.md#package-map) when you add or change a package.
- Don't merge with a failing `make check`.
- Document any new convention in [AGENTS.md](./AGENTS.md) (rules) or [docs/](docs/) (reference) — not in tool-specific files.

## How the guideline files fit together

One canonical rules file, picked up automatically by every major AI tool — no duplicated content to drift:

| File | Read by |
|---|---|
| **[AGENTS.md](./AGENTS.md)** | Canonical rules. Cursor, Zed, Aider, GitHub Copilot coding agent read it natively. |
| [CLAUDE.md](./CLAUDE.md) | Claude Code — a pointer to `AGENTS.md`. |
| [.github/copilot-instructions.md](.github/copilot-instructions.md) | GitHub Copilot — a pointer to `AGENTS.md`. |

Change the rules in **`AGENTS.md` only**. The pointer files carry no rules of their own.
