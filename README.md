# 🪴 jardinière

`jard` gives a coding agent its own long-lived container sandbox. Create one, set it up however you like, and it stays — packages, shell history, and agent state are all still there next time.

```sh
cd ~/work/myrepo
jard run          # first time: builds a sandbox, starts Claude Code
                  # every time after: reattaches, with your setup intact
```

## prerequisites

An OCI-compatible container runtime: Docker, Podman, OrbStack, or colima. `jard` ships none of them and autodetects whichever you have.

## install

**Nix (as system package):**

```sh
# run without installing
nix run github:rhizomatous/jardiniere

# or install into your profile
nix profile install github:rhizomatous/jardiniere
```

**Nix (in your project flake):**

Add jardinière as an input, then apply its overlay to get `pkgs.jard`:

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

Grab a prebuilt binary for macOS or Linux from the [latest release](https://github.com/rhizomatous/jardiniere/releases/latest). This _miiiiight_ not work on macOS, because codesigning isn't set up yet.

## usage

```sh
jard                          # the dashboard: everything you have, and its load
jard run                      # the current directory, with the default agent
jard run codex                # a different agent
jard run ~/work/myrepo        # somewhere else
jard run --name scratch       # reattach by name, from anywhere

jard run -- --dangerously-skip-permissions   # everything after -- goes to the agent
```

The first `jard run` in a directory creates a sandbox. Every one after that reattaches to it. That's the whole idea: you install what you need once, and it's still there.

```sh
jard ls                       # what exists, and what's running
jard inspect myrepo           # one sandbox in full
jard stop                     # stop the one for this directory
jard start myrepo
jard rm myrepo                # delete it and everything in it
jard exec myrepo bash -lc 'npm test'
jard cp myrepo:/home/agent/notes.md ./notes.md
jard agents                   # what you can run
```

`ls` and `inspect` take `--json`. Every command that takes a sandbox name will default to the one for your current directory if you leave it out.

### the daemon

A small background process, `jardd`, owns your sandboxes. It starts on its own the first time a command needs one, so there is nothing to set up and nothing to remember.

```sh
jard daemon status            # is it running, and where does it listen
jard daemon start             # start it deliberately
jard daemon stop              # stop it; your sandboxes keep running
```

It exists because some of what jard does has to outlive the command that asked for it — and it's where host-enforced network policy will live. Stopping it leaves every sandbox alone: they're containers in their own right, and the next command that needs a daemon starts a new one.

Two flags skip it. `--dry-run` prints what jard would do and works on a machine with no runtime and no daemon at all; `--state-dir` names a store the running daemon doesn't own, so it runs in-process rather than quietly reading the wrong one. To run a daemon against a store of your choosing, start it yourself with `jardd --state-dir`.

### the dashboard

Run `jard` with no arguments and you get a dashboard instead: every sandbox, its status, and live CPU and memory for the running ones.

| key | |
| --- | --- |
| `↑` `↓` / `k` `j` | move |
| `i` | show the selected sandbox's details |
| `c` | create a sandbox |
| `enter` | attach the agent |
| `x` | open a shell |
| `s` | start or stop |
| `r` | remove |
| `?` | show every binding |
| `q` | quit |

`i` opens a pane under the list with what `jard inspect` would print — image, workspaces, limits, ports, env. It follows the cursor, so moving between sandboxes moves the details with it.

Attaching leaves the dashboard and hands the terminal to the agent, the same as `jard run` would. When the session ends you're back at the dashboard.

Piped or run from a script, `jard` prints the `ls` table rather than trying to draw a dashboard into something that isn't a terminal.

### workspaces

The directory you run in is the sandbox's **primary workspace**. It's bind-mounted read-write at the same absolute path it has on your host, so a stack trace pointing at `/home/you/work/myrepo/main.go` means the same thing on both sides.

Mount more than one, with `:ro` for read-only:

```sh
jard run ~/work/frontend ~/work/backend ~/work/design-docs:ro
```

The first path is the primary: it's the working directory, and the path `jard run` reattaches by. Workspaces are fixed when the sandbox is created.

### resource limits and ports

```sh
jard create --cpus 4 -m 8GiB -p 3000 -p 8080:80 -e NODE_ENV=development
```

These are create-time settings. Passing them to `jard run` for a sandbox that already exists warns rather than silently doing nothing — recreate the sandbox to change them.

### seeing what it would do

```sh
jard run --dry-run
```

Prints the exact container commands and runs none of them. Works without a container runtime installed at all, which makes it the quickest way to understand what `jard` is actually doing.

## how it works

```
jard run  →  detect the OCI runtime (docker / podman / orbstack / colima)
          →  find the sandbox for this directory, or create one:
               • from the agent's base image
               • workspaces bind-mounted at their host paths
               • a named volume at /home/agent
          →  start it if it isn't running
          →  exec the agent, with your terminal attached
```

The named volume at `/home/agent` is what makes persistence work. Anything installed under your home directory — apt packages you `sudo apt install`, npm globals, rustup toolchains, shell history, the agent's own state — survives `jard stop`, and lives until `jard rm`.

Two things deliberately don't persist that way. The agent binary lives in `/usr/local`, outside the volume, so pulling a newer image updates it. And nothing is seeded from your host: a sandbox's contents come from its image and from what you run inside it.

### images

Each agent has a base image published at `ghcr.io/rhizomatous/jard-<agent>`. They follow the same contract Docker's `sbx` uses, so third-party images are portable: Ubuntu, a non-root `agent` user at UID 1000, passwordless sudo, and proxy environment preserved across sudo.

`--image` starts from something else:

```sh
jard create --image ghcr.io/acme/our-toolchain:latest
```

`jard` never builds images and never commits them. If you want a different starting point, build it yourself and point at it. The [`images/`](./images/Dockerfile) directory is a reasonable place to start from.

## what the sandbox actually protects

**Containers, not microVMs.** This matters, so here it is plainly:

- **macOS** — the container runs inside your runtime's Linux VM (Docker Desktop, OrbStack, colima). An escape lands the agent in that VM, not on your Mac.
- **Linux** — a shared-kernel boundary. A kernel privilege-escalation bug is a host compromise. Rootless Podman with user namespaces is the configuration to prefer.
- **Nested Docker** — not supported. No dind, no sysbox, no `--privileged`. Sandboxes never run privileged, and `docker build` inside one is out of scope.

Kernel-level isolation is the standing gap. If you need it today, this isn't the tool.

### your working tree is not protected

The workspace mount is read-write, by design — the agent has to be able to edit your code. That means it can also write files that execute on **your** machine later:

`.git/hooks/`, `.github/workflows/`, `Makefile`, `package.json` scripts, `.vscode/tasks.json`, `.claude/settings.json`.

Committing, pushing, building, or just opening the project is enough to run them. Treat a sandbox's changes the way you'd treat a pull request from a contributor you don't know.

Worth knowing: `.git/hooks/` doesn't show up in `git diff`. If you review only the diff, you won't see it.

### network

Not yet. Egress is currently unrestricted, and host-enforced network policy is the next thing being built. Until then, assume a sandbox can reach anything your machine can.

## development

A Nix dev shell is provided — `nix develop`, or `use flake` with direnv. See [the Makefile](./Makefile) for the usual commands, and [AGENTS.md](./AGENTS.md) for conventions.
