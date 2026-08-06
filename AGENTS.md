# AGENTS.md

**jardinière** (`jard`) is a Go CLI that runs coding agents inside isolated, persistent container sandboxes. Read `README.md` for what it does and what it protects against.

`docs/next/plan.md` is the plan of record: it describes where each piece is headed, and is the thing to read before adding one.

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
- `internal/tui`: the bubbletea dashboard, which bare `jard` opens.
- `internal/ui`: Charm-based terminal output.
- `images/`: one multi-stage Dockerfile, a build target per agent, published to ghcr.

**The invariant:** `cmd/jard` and `internal/tui` hold an `api.Service` and never
reach past it to `internal/runner`, `internal/store`, or a container runtime.
`depguard` enforces this; don't work around it.

## Things that will bite you

- **A sandbox's definition is fixed at create time.** `Spec` is written once and reread on every reattach. Anything that should be changeable needs a deliberate story for changing it.
- **`/home/agent` is a volume mount point.** A volume takes its contents from the image only on first use, so anything the image puts under `/home/agent` is frozen the moment a sandbox is first started. Agent binaries go in `/usr/local`.
- **`rm --volumes` does not remove named volumes,** only anonymous ones. The home volume needs its own `volume rm`, or every removed sandbox leaks its disk.
- **Workspace paths cannot contain `:`.** A mount spec is colon-delimited, so such a path silently binds somewhere else. `Spec.Validate` rejects it; keep it that way.
- **A character-device check is not a terminal check.** `/dev/null` is a character device. Use `term.IsTerminal`, or a redirected command will claim a TTY it doesn't have.
- **Don't drive the TUI by feeding keys to `tea.Run` in tests.** It races the program's startup and produces tests that sometimes wait for a deadline. Call `Model.Update` directly; `run_test.go` covers only that the loop starts and stops.

## Testing

Unit tests are **pure**, with no container runtime required. They cover arg-building, parsing, validation, and rendering. Keep them that way: inject dependencies like `goos` rather than reading globals.

There are two fakes, at opposite ends of the stack. `runner.Fake` replaces only the container runtime, so `direct` and `store` run for real against a temp dir. `api.NewFake` replaces the whole sandbox layer, for driving the CLI — and it's required, not merely convenient, because `depguard` forbids `cmd/jard` from importing `internal/store`.

To verify real container behavior, use `jard --dry-run` (which needs no runtime) or a live run against docker/OrbStack.

## Committing, Versioning, Releasing

- Use [Conventional Commits](https://www.conventionalcommits.org/). Always include the scope:  `feat(sandbox): ...`, `chore(docs): ...`. View the git log for examples of scopes used in the project.
- Keep `CHANGELOG.md` up to date. Use [Keep a Changelog](https://keepachangelog.com/) guidelines for waht goes in it.
- Use [Semantic Versioning](https://semver.org). Keep the version in `flake.nix` in sync with the version used in the release tags.