# 🪴 jardinière

a Nix-based sandbox for running coding agents in a repo. point `jard` at a Nix-based repo to spin up Linux container, install the repo's Nix config, and drop into a working dev env with the agent of your choice.

## install

on macOS, install with Homebrew:

```sh
brew install rhizomatous/tap/jard
```

or grab a prebuilt binary for macOS or Linux from the [latest release](https://github.com/rhizomatous/jardiniere/releases/latest), or install with Go:

```sh
go install github.com/rhizomatous/jardiniere/cmd/jard@latest
```

## usage

```sh
# sandbox the current repo
jard

# sandbox a different repo
jard --dir ../some-repo
```

### configuring

provide a `jardiniere.toml` in your repo to override jardinière's defaults. for example, you might wish one repo to drop you directly into a Claude session, or another to be built off a different base image. 

```toml
# command run inside `nix develop`. default "bash"
startup = "claude"
# image used for container. default "nixos/nix:latest"
image = "nixos/nix:latest"
```

## how it works

```
jard  →  read ./jardiniere.toml
      →  detect whatever OCI runtime is present (docker / podman / orbstack / etc.)
      →  run a Nix-enabled Linux container with:
           • repo bind-mounted at /work    (git commit to the host fs)
           • persistent /nix store volume  (fast cold starts)
           • your git identity injected    (author commits as you)
           • ssh-agent forwarded           (if Linux, or macOS on OrbStack/Docker)
      →  exec `nix develop /work --command <startup>`
```

### bring your own agent

jardinière is completely agnostic to which model or harness you use. configure your tool of choice in the target repo's own Nix flake.

### sandboxing 

jardinière uses Linux containers to sandbox your agent. use any Docker or Podman compatible runtime of your choice. it will autodetect and use whichever you have present.

on Linux, jardinière can SSH forward for you. on macOS, it can do so _if_ you're using Docker, OrbStack, or another runtime that's compatible. (podman is not.)

### network policy

set `[network].mode` in `jardiniere.toml` to control what the agent can reach:

- `"full"` (default): unrestricted network.
- `"none"`: no network at all.
- `"allowlist"`: only the hosts in `allow` (and their subdomains).

in `allowlist` mode, the sandbox joins an isolated network with no direct route out. its only egress is a proxy sidecar that permits `CONNECT` to allowed hosts only. unfortunately, this only supports HTTP(S)! so use HTTPS git remotes in this mode.

## development

```sh
# preview the exact terminal command that will be run
jard --dry-run
```
