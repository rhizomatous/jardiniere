# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-20

### Added

- `agent` config field: optionally drop Opencode, Claude Code, or Codex into the sandbox's Nix env.
- Nix flake package output, so `jard` can be installed with `nix profile install github:rhizomatous/jardiniere` or run with `nix run github:rhizomatous/jardiniere`.
- Nix flake `overlays.default`, so downstream flakes can add jardinière as an input and get `pkgs.jard`.

## [0.1.4] - 2026-07-19

### Added

- Homebrew cask distribution: `brew install rhizomatous/tap/jard` (macOS).

## [0.1.3] - 2026-07-19

### Fixed

- `go install github.com/rhizomatous/jardiniere/cmd/jard@latest` now produces a binary named `jard` rather than `jardiniere`.

## [0.1.1] - 2026-07-19

### Fixed

- Corrected the Go module path from `github.com/vivshaw/jardiniere` to `github.com/rhizomatous/jardiniere` so it matches the repository host and `go install github.com/rhizomatous/jardiniere@latest` resolves.

## [0.1.0] - 2026-07-19

### Added

- Core sandbox: run coding agents inside a Nix-enabled Linux container, with the target repo bind-mounted at `/work` and a persistent `/nix` store volume.
- Container runtime autodetection across Docker, Podman, OrbStack, and other OCI-compatible runtimes.
- Git identity injection so the agent can author commits as you.
- SSH-agent forwarding on Linux, and on macOS when using Docker or OrbStack.
- Network policy, including an allowlist mode.
- Configurable extra host mounts.
- `jardiniere.toml` config file, supporting a custom `startup` command, `image` override, and network policy.
- Kong-based CLI with `--version` and `--dry-run` flags.

[Unreleased]: https://github.com/rhizomatous/jardiniere/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/rhizomatous/jardiniere/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/rhizomatous/jardiniere/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/rhizomatous/jardiniere/compare/v0.1.1...v0.1.3
[0.1.1]: https://github.com/rhizomatous/jardiniere/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/rhizomatous/jardiniere/releases/tag/v0.1.0
