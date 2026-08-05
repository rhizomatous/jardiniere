# AGENTS.md

**jardinière** (`jard`) is a Go CLI that runs coding agents inside isolated, persistent container sandboxes.

It is mid-rearchitecture. `docs/next/plan.md` is the plan of record and describes where each piece is headed; this file describes what exists now. `README.md` still documents the pre-rearchitecture tool and is rewritten at the end of phase 1.

## Dev environment

All tooling is provided in Nix dev shell: **work inside it.**

## Commands

- See `Makefile` for the common dev commands.
- `jard --dry-run`: print the exact container commands without executing them. the best way to inspect behavior without a live runtime.

## Conventions

- **Formatting:** gofumpt & goimports.
- **Linting:** golangci-lint (staticcheck for bugs; revive for style; errcheck, gocritic, errorlint, etc.). Keep it at 0 issues.
- **Doc comments:** standard Go form. Do not write archaeological comments describing past states and changes.
- **Prose:** comments are lowercase and terse.
- **Errors:** lowercase, no trailing punctuation; `errors.New` for static strings, `fmt.Errorf` + `%w` when wrapping.

## Layout

- `cmd/jard`: the CLI (cobra + fang). The `main` package.
- `internal/api`: the `Service` interface and its types.
- `internal/api/direct`: in-process implementation of `Service`.
- `internal/store`: sandbox specs + state, on disk, XDG-respecting.
- `internal/runner`: the `Runner` interface, runtime detection, and the OCI adapter.
- `internal/ui`: Charm-based terminal output.

**The invariant:** `cmd/jard` and `internal/tui` hold an `api.Service` and never
reach past it to `internal/runner`, `internal/store`, or a container runtime.
`depguard` enforces this; don't work around it.

## Testing

Unit tests are **pure**, with no container runtime required. They cover arg-building, parsing, config generation, etc. Keep them that way: inject dependencies like `goos` rather than reading globals. `runner.Fake` covers store and service logic; `api.NewFake` covers the CLI and TUI. To verify real container behavior, use a running docker/OrbStack with `jard --dry-run` or a live run.

## Committing, Versioning, Releasing

- Use [Conventional Commits](https://www.conventionalcommits.org/). Always include the scope:  `feat(sandbox): ...`, `chore(docs): ...`. View the git log for examples of scopes used in the project.
- Keep `CHANGELOG.md` up to date. Use [Keep a Changelog](https://keepachangelog.com/) guidelines for waht goes in it.
- Use [Semantic Versioning](https://semver.org). Keep the version in `flake.nix` in sync with the version used in the release tags.