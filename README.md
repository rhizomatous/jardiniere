# 🪴 jardinière

a Nix-based sandbox for running coding agents in a repo. point `jard` at a repo with a Nix flake to spin up a Linux container, install the repo's Nix config, and drop into a working dev env with the agent of your choice.

## prerequisites

you'll need an OCI-compatible container runtime on your system. any that provide a Docker-compatible interface should be fine, as is podman.

## install

**Nix (as system package):**

```sh
# run without installing
nix run github:rhizomatous/jardiniere

# or install into your profile
nix profile install github:rhizomatous/jardiniere
```

**Nix (in your project flake):**

add jardinière as an input, then apply its overlay to get `pkgs.jard`:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    jardiniere.url = "github:rhizomatous/jardiniere";
    # optional: dedupe nixpkgs so you don't pull a second copy
    jardiniere.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { nixpkgs, jardiniere, ... }:
    let
      system = "aarch64-darwin";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ jardiniere.overlays.default ];
      };
    in {
      devShells.${system}.default = pkgs.mkShell {
        packages = [ pkgs.jard ];
      };
    };
}
```

**Homebrew (macOS):**

```sh
brew install rhizomatous/tap/jard
```

**Go:**

```sh
go install github.com/rhizomatous/jardiniere/cmd/jard@latest
```

**Binary:**

grab a prebuilt binary for macOS or Linux from the [latest release](https://github.com/rhizomatous/jardiniere/releases/latest). this _miiiiight_ not work on macOS, because codesigning is not set up yet.

## usage

```sh
# sandbox the current repo
jard

# sandbox a different repo
jard --dir ../some-repo
```

### configuring

provide a `jardiniere.toml` in your repo to set defaults for your jardinière sessinos. for example, you might wish a repo to drop you directly into a Claude session, block all network egress, or provision Opencode for you.

```toml
# command run inside `nix develop`. default "bash"
startup = "claude"

# disable outgoing network
[network]
mode = "none"

# inject an Opencode agent
agent = "opencode"
```

### flags

CLI flag override what the repo's config sets.

| flag | config key | default | description |
| --- | --- | --- | --- |
| `--dir` | — | `.` | repo directory to sandbox |
| `--startup` | `startup` | `bash` | command to run inside the dev shell |
| `--image` | `image` | `nixos/nix:latest` | base runner image |
| `--agent` | `agent` | `none` | coding agent to inject: `none`, `opencode`, `claude-code`, or `codex` |
| `--mount` | `mounts` | — | extra host mount: `source[:target][:ro\|rw]` (repeatable) |
| `--network` | `network.mode` | `full` | network egress policy: `full`, `none`, or `allowlist` |
| `--allow` | `network.allow` | — | host reachable in `allowlist` mode (repeatable) |
| `--dry-run` | — | — | print the container command instead of running it |
| `--version` | — | — | print version and exit |

### working with multiple repos

you might wish to have a session that touches multiple repos, for example working on your app's frontend and backend together. use the `mounts` parameter for this. with each keeping its own Nix dev shell.

#### from a host repo

if you only want to reference other repos with read access, add the `mounts` parameter to your existing `jardiniere.toml`. (N.B., this will _not_ auto-install the Nix shell for that repo!)

```toml
# ~/work/frontend/
mounts = [
  "../backend:/work/backend:r"
]
```

#### as a custom workspace

for more complex uses, such as if you need a full development environment for multiple services, set up a workspace. this is a directory with its own `flake.nix` and a `jardiniere.toml` that mounts the repos you want:

```
# ~/work/fullstack/
├── flake.nix
└── jardiniere.toml
```

```toml
# ~/work/fullstack/jardiniere.toml
startup = "claude"

mounts = [
  "../frontend:/work/frontend:rw",
  "../backend:/work/backend:rw",
]
```

if you want a full, batteries-included workspace, you can populate the workspace `flake.nix` with all the tools you need. (perhaps by composing together the `flake.nix`es from each mounted repo.) this might be preferable if you want a no-fuss dev environment. alternatively, you can use a minimal `flake.nix`, and instruct your agent to work with the `flake.nix` from each repo by prefixing commands with `nix develop`. this might be preferable if your services may use conflicting dependency versions.

## how it works

```
jard  →  read ./jardiniere.toml
      →  detect whatever OCI runtime is present (docker / podman / orbstack / etc.)
      →  run a NixOS container with:
           • repo bind-mounted at /work
           • persistent /nix store volume
           • your git identity injected
           • ssh-agent forwarded (if Linux, or macOS on OrbStack/Docker)
      →  exec `nix develop /work --command <startup>`
```

### agents

jardinière is totally agnostic to which agent you use. configure your tool of choice in the target repo's own Nix flake and point `startup` at it. if you'd rather not, `jard` can drop Opencode, Claude Code, or Codex into the sandbox for you via the `agent` config param. (note that Claude Code has an unfree license! if you pick it, jardinière wll set `NIXPKGS_ALLOW_UNFREE=1` in your sandbox!)

### sandboxing 

jardinière uses a Linux container to sandbox your agent. use any Docker or Podman compatible runtime of your choice. it will autodetect and use whichever you have present.

on Linux, jardinière can SSH forward for you. on macOS, it can do so _if_ you're using Docker, OrbStack, or another runtime that's compatible. (podman is not.)

### network policy

set `[network].mode` in `jardiniere.toml` to control what the agent can reach:

- `"full"` (default): unrestricted network.
- `"none"`: no network at all.
- `"allowlist"`: only the hosts in `allow` (and their subdomains).

in `allowlist` mode, the sandbox joins an isolated network with no direct route out. its only egress is a proxy sidecar that permits `CONNECT` to allowed hosts only. unfortunately, this only supports HTTP(S)! so use HTTPS git remotes in this mode.

## development

a Nix dev shell is provided. use `nix develop` or `use flake` to enter. consult [the Makefile](./Makefile) for relevant dev commands.
