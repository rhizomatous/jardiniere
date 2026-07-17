# 🪴 jardinière

a Nix-based sandbox for running coding agents in a repo. point `jard` at a Nix-based repo to spin up Linux container, install the repo's Nix config, and drop into a working dev env with the agent of your choice.

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
      →  detect whatever OCI runtime is present (docker / podman / orbstack)
      →  run a Nix-enabled Linux container with:
           • repo bind-mounted at /work   (commits land on the host)
           • persistent /nix store volume (fast cold starts)
           • your git identity injected    (authored as you)
           • ssh-agent forwarded           (push works; Linux today)
      →  exec `nix develop /work --command <startup>`
```

### bring your own agent

jardinière is completely agnostic to which model or harness you use. configure your tool of choice in the target repo's own Nix flake.

### sandboxing 

jardinière uses Linux containers to sandbox your agent. use any Docker or Podman compatible runtime of your choice. it will autodetect and use whichever you have present.

## development

```sh
# preview the exact terminal command that will be run
jard --dry-run
```

### roadmap

- **v0** ✅ config, runtime detection, repo mount, `nix develop` loop, git identity
- **v1** ssh-agent forwarding on macOS backends; polished Charm TUI
- **v2** persistent-store tuning, network policy (`none`/`allowlist`/`full`),
  extra mounts, a purpose-built two-layer runner image, `colima` auto-provision
