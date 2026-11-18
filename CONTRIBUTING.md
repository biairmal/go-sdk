# Contributing

How a change gets from an idea to `main`. It applies to everyone, human or AI agent. How the code itself is written
(package boundaries, API shape, configuration, tests) is in [AGENTS.md](AGENTS.md), and it binds humans too.

## Before you start

1. Install Go 1.26+ and Make (on Windows, Git Bash or WSL), then the tools:

   ```bash
   make install-tools   # gofumpt, golangci-lint, govulncheck
   ```

2. Read [AGENTS.md](AGENTS.md) for the authoring rules, and [docs/PATTERNS.md](docs/PATTERNS.md) for examples.
   Adding a package? Follow [docs/NEW_PACKAGE_CHECKLIST.md](docs/NEW_PACKAGE_CHECKLIST.md).
3. Find the spec. Planned work comes from the workspace specs (go-identity's TRD and the
   [development plan](docs/DEVELOPMENT_PLAN.md)). When a spec and AGENTS.md disagree, the spec wins. When the spec
   is unclear, pick the simplest option that fits it and say so in the PR's Judgement calls.

## Development loop

1. Change one package (packages are independent, and there is no entry point).
2. Add table-driven tests in the same change.
3. Run the gate:

   ```bash
   make check              # format, lint-fix, unit tests, coverage, vulncheck, deps-verify
   make test-integration   # -tags=integration; needs the live services below
   ```

   Integration tests need Redis and Kafka: run `scripts/gen-kafka-certs.sh` once, then
   `docker compose -f docker-compose.integration.yaml up -d`.

4. Walk the [Definition of Done](#definition-of-done), then open a PR.

## Git conventions

`.github/workflows/pr-conventions.yml` checks these on every PR. PRs are squash-merged, so the PR title becomes the
commit on `main`.

- **Branch:** `<type>/<group>/<task-id>-<slug>`. `<group>` is optional and names the work stream, so parallel streams
  don't collide. The task ID keeps the plan's casing. Examples: `feat/p2/T10.3-audit-target`, `fix/deps/klauspost-cve`.
- **Commit and PR title:** [Conventional Commits](https://www.conventionalcommits.org/),
  `<type>(<scope>)?: <what it does, lower case>`, plus ` (<task id>)` on the PR title. Types:
  `feat fix refactor perf test docs build ci chore revert`. A breaking change adds `!` and a `BREAKING CHANGE:`
  footer. The commit body says why, not how.
- **PR body:** fill in [.github/pull_request_template.md](.github/pull_request_template.md). `### Why` (requirement,
  bug, security, or refactor, with its source), `### What changed` and `### Verification` are required. Verification
  quotes each command's real output from the final pushed head, keeps failed or skipped checks in the table, and
  lists what was not verified.
- **Releases:** tag `vX.Y.Z` on `main`. A change to an interface the `mocks` module covers also needs a
  `mocks/vX.Y.Z` tag after the mocks are regenerated.

## Definition of Done

- [ ] `make check` passes (format, lint-fix, unit tests, coverage, vulncheck, deps-verify).
- [ ] New or changed behaviour has **table-driven tests**; live-service paths are covered by `*_integration_test.go`.
- [ ] Every touched or new package has an up-to-date **package doc comment and `README.md`**.
- [ ] Errors use `errorz` with an appropriate code and **wrap the underlying cause** (`errors.Is`-friendly).
- [ ] Any `logger.Logger` use is **optional and nil-safe**; no stdlib `log`.
- [ ] No new **unjustified cross-package imports**.
- [ ] A new package has its row in the AGENTS.md [package map](AGENTS.md#package-map) and in the README's package
      list.
- [ ] Public API uses stdlib types; constructors are `NewX`; options are `WithX`.
- [ ] Any configuration is an exported `Config` struct with **`mapstructure` snake_case tags** (non-serializable
      fields tagged `-`), a `DefaultConfig()`, and a `Validate()`; `WithX` is used only for optional deps and
      non-serializable behaviour.
- [ ] Every new exported interface has a **`//go:generate mockgen` directive** writing into `mocks/<pkg>/`;
      `make mocks` was run and the generated files committed.
- [ ] The README, [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [SECURITY.md](SECURITY.md) match the change.

## How the guideline files fit together

| File | Holds | Read by |
|---|---|---|
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process: this page | Everyone. GitHub links it on the new-PR page |
| [AGENTS.md](AGENTS.md) | Code rules | Everyone. Loaded natively by Cursor, Zed, Aider and the Copilot coding agent |
| [CLAUDE.md](CLAUDE.md) | Imports AGENTS.md and CONTRIBUTING.md | Claude Code |
| [.github/copilot-instructions.md](.github/copilot-instructions.md) | Points to both | GitHub Copilot |

Change rules in AGENTS.md or CONTRIBUTING.md only. The pointer files hold no rules of their own.
