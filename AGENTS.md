# AGENTS.md

**jardinière** (`jard`) is a Go CLI that runs coding agents inside isolated, Nix-based container sandboxes. Read `README.md` for an in-depth description.

## Dev environment

All tooling is provided in Nix dev shell: **work inside it.**

## Commands

- See `Makefile` for the common dev commands.
- `jard --dry-run`: print the exact `docker run` command without executing it. the best way to inspect behavior without a live runtime.

## Conventions

- **Formatting:** gofumpt & goimports.
- **Linting:** golangci-lint (staticcheck for bugs; revive for style; errcheck, gocritic, errorlint, etc.). Keep it at 0 issues.
- **Doc comments:** standard Go form. Do not write archaeological comments describing past states and changes.
- **Prose:** comments are lowercase and terse.
- **Errors:** lowercase, no trailing punctuation; `errors.New` for static strings, `fmt.Errorf` + `%w` when wrapping.

## Layout

- `main.go`: CLI flags, orchestration, preflight checks.
- `internal/config`: parses the `jardiniere.toml` config file.
- `internal/runtime`: detects the container runtime.
- `internal/sandbox`: assembles and runs the container invocation.
- `internal/ui`: Charm-based terminal output.

## Testing

Unit tests are **pure**, with no container runtime required. They cover arg-building, parsing, config generation, etc. Keep them that way: inject dependencies like `goos` rather than reading globals. To verify real container behavior, use a running docker/OrbStack with `jard --dry-run` or a live run.

## Committing, Versioning, Releasing

- Use [Conventional Commits](https://www.conventionalcommits.org/). Always include the scope:  `feat(sandbox): ...`, `chore(docs): ...`. View the git log for examples of scopes used in the project.
- Keep `CHANGELOG.md` up to date. Use [Keep a Changelog](https://keepachangelog.com/) guidelines for waht goes in it.
- Use [Semantic Versioning](https://semver.org)