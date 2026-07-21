# Package README standard

Every package `README.md` in this repo follows the section order below. This keeps all packages scannable and predictable. The rules are referenced by [../AGENTS.md](../AGENTS.md) and the [new-package checklist](NEW_PACKAGE_CHECKLIST.md).

## Section order

| # | Section | Required? | Purpose |
|---|---|---|---|
| 1 | **Title + quick description** | Always | `# <Name> Package` followed by one paragraph: what it is and the single most important thing it does. |
| 2 | **Overview** | Always | A few sentences of context — what problem it solves, how it fits the SDK, what it wraps/builds on. |
| 3 | **Features** | Always | Bulleted capabilities, each with a **bold lead-in**. Group under `### Core Capabilities` / `### Use Cases` only if the list is long. |
| 4 | **Usage** | Always | `### Installation` (the `go get` line) + one or more usage subsections (`### Basic usage`, plus any task-specific usage). |
| 5 | **Options** | If any | Functional options / config knobs, as a `\| Option \| Description \|` table. Omit if the package has none. |
| 6 | **Limitations** | Always | Known constraints and gotchas, as bullets with a **bold lead-in**. If genuinely none, write "None currently." |
| 7 | **Dependencies** | If any | Third-party modules the package pulls in (beyond the stdlib), as bullets with links. Omit if it's stdlib-only. |
| 8 | **See also** | Always | Cross-references: related SDK packages and external docs, as `- [Name](link) – short note`. |

### Optional sections (use when they add value)

A package may add more sections — place them as follows so the required tail (Limitations → Dependencies → See also) stays consistent:

- **Near the top** (after Overview or Features): `Package Structure`, `Core Interfaces` — for packages exposing several types/interfaces (e.g. `repository`).
- **After Usage / Options, before Limitations**: `Examples` (extended end-to-end), `API Reference` / `Configuration Reference`, `Security Considerations`, `Performance Considerations`, `Migration Path`.
- **Very end** (after See also): `License` — optional.

## Formatting conventions

- **Title**: `# <Name> Package` — capitalize the display name consistently (`# Errorz Package`, `# Httpkit Package`).
- **Headings**: `##` for the standard sections, `###` for subsections. Sentence case for multi-word headings (`## See also`, not `## See Also`).
- **Code fences**: ` ```bash ` for shell, ` ```go ` for Go, ` ```yaml ` for config — always tag the language.
- **Features / Limitations bullets**: lead with a **bold term**, then a colon and the explanation.
- **Links**: relative paths to sibling packages (`../sqlkit/README.md`); use `–` (en dash) before the short note.
- **Install line**: always `go get github.com/biairmal/go-sdk/lib/<package>`.

## Template

Copy the skeleton below into a new package's `README.md` and fill it in. Delete any section marked optional that doesn't apply.

````markdown
# <Name> Package

<One-paragraph quick description: what this package is and the one thing it does best.>

## Overview

<A few sentences: the problem it solves, how it fits the SDK, what it wraps or builds on.>

## Features

- **<Capability>**: <what it does>.
- **<Capability>**: <what it does>.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/<package>
```

### Basic usage

```go
package main

import "github.com/biairmal/go-sdk/lib/<package>"

func main() {
    // minimal, runnable example
}
```

<!-- Add more usage subsections (### …) for common tasks as needed. -->

## Options

<!-- Omit this whole section if the package has no options. -->

| Option | Description |
|--------|-------------|
| `WithX(...)` | <what it changes; default value> |

<!-- Optional deep sections (Examples, API Reference, Security/Performance Considerations)
     go here — after Options, before Limitations. -->

## Limitations

- **<Constraint>**: <explanation / workaround>.

## Dependencies

<!-- Omit if stdlib-only. -->

- [<module>](<url>) – <why it's used>.

## See also

- [<Related Package>](../<pkg>/README.md) – <how it relates>.
- [<External doc>](<url>) – <what it is>.
````
