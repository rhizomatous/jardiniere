// `sandbox` assembles and runs a container invocation to drop the
// user into an isolated, Nix-based dev environment for a target repo.
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vivshaw/jardiniere/internal/config"
	rt "github.com/vivshaw/jardiniere/internal/runtime"
)

// `nixStoreVolume` is a persistent named volume mounted at /nix so the Nix store
// survives across runs. after the first run, subsequent runs can reuse everything
// already built or fetched for faster cold starts.
const nixStoreVolume = "jardiniere-nix"

// `GitIdentity` carries the host user's commit identity into the sandbox via
// environment variables. this way, commits can be authored "as you".
type GitIdentity struct {
	Name  string
	Email string
}

// `label` renders the identity for display, e.g. "viv shaw <hey@vivsha.ws>".
// returns `""` when nothing is configured.
func (g GitIdentity) Label() string {
	switch {
	case g.Name != "" && g.Email != "":
		return g.Name + " <" + g.Email + ">"
	case g.Name != "":
		return g.Name
	case g.Email != "":
		return g.Email
	default:
		return ""
	}
}

// `Options` fully describes one sandbox run.
type Options struct {
	Runtime  rt.Runtime
	Config   config.Config
	RepoDir  string // absolute path to the repo, bind-mounted at /work
	Identity GitIdentity
	SSHSock  string // host SSH_AUTH_SOCK, forwarded when non-empty
	DryRun   bool   // print the command instead of running it
}

// `workdir` is where the repo is mounted inside the container.
const workdir = "/work"

const (
	// `containerSSHSock` is where the forwarded agent socket lands inside the sandbox.
	containerSSHSock = "/ssh-agent"
	// `dockerVMSSHSock` is the fixed path where both Docker Desktop and OrbStack
	// bridge the macOS host's SSH agent into the Linux VM.
	dockerVMSSHSock = "/run/host-services/ssh-auth.sock"
)

// `sshAgentHostSocket` decides which host-side socket to bind-mount for ssh-agent
// forwarding, or returns ok=false with a reason.
//
//   - Linux: the container shares the host kernel, mount $SSH_AUTH_SOCK directly.
//   - macOS & docker-family (Docker Desktop/OrbStack): mount the VM bridge path.
//   - macOS & podman: doesn't have a VM bridge, forwarding is unsupported.
func sshAgentHostSocket(goos, rtName, hostSock string) (path string, ok bool, reason string) {
	if hostSock == "" {
		return "", false, "" // no host agent to forward, nothing to warn about
	}
	switch goos {
	case "linux":
		return hostSock, true, ""
	case "darwin":
		if rtName == "docker" {
			return dockerVMSSHSock, true, ""
		}
		return "", false, "ssh-agent forwarding on macOS is wired for docker-family runtimes " +
			"(Docker Desktop/OrbStack) only; " + rtName + " isn't supported yet, so SSH pushes may fail"
	default:
		return "", false, "ssh-agent forwarding is not supported on " + goos
	}
}

// `Run` builds the container command and either executes it (inheriting the
// terminal for an interactive session) or, under DryRun, prints it.
func Run(ctx context.Context, opts Options) error {
	// in allowlist mode the sandbox joins an isolated network fronted by a
	// filtering proxy.
	var proxy *proxySidecar
	if opts.Config.Network == config.NetworkAllowlist {
		proxy = planProxySidecar(opts)
		if !opts.DryRun {
			if err := proxy.start(ctx); err != nil {
				proxy.cleanup()
				return err
			}
			defer proxy.cleanup()
		}
	}

	args := buildArgs(opts, runtime.GOOS, proxy)

	if opts.DryRun {
		fmt.Println(opts.Runtime.Path, strings.Join(args, " "))
		return nil
	}

	cmd := exec.CommandContext(ctx, opts.Runtime.Path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// `SSHAgentStatus` reports whether ssh-agent forwarding will be active for this
// runtime and host, plus a human-readable detail. Callers use it to render the
// run summary.
func SSHAgentStatus(rtName, hostSock string) (forwarded bool, detail string) {
	_, ok, reason := sshAgentHostSocket(runtime.GOOS, rtName, hostSock)
	switch {
	case ok:
		return true, ""
	case hostSock == "":
		return false, "no host agent running"
	default:
		return false, reason
	}
}

// `buildArgs` assembles the container `run` arguments. goos is injected (rather
// than read from the global runtime.GOOS) so tests can exercise each platform's
// ssh-forwarding path, independent of the host they run on.
func buildArgs(opts Options, goos string, proxy *proxySidecar) (args []string) {
	c := opts.Config
	args = []string{
		"run", "--rm", "-it",
		"--hostname", "jardiniere",
		"-v", opts.RepoDir + ":" + workdir,
		"-w", workdir,
		"-v", nixStoreVolume + ":/nix",
		// enable flakes & the nix command in images (like nixos/nix) that ship with them off.
		"-e", "NIX_CONFIG=experimental-features = nix-command flakes",
	}

	// apply network config
	args = append(args, networkArgs(c.Network, proxy)...)

	// commit as the host user, without mutating the repo's git config.
	if opts.Identity.Name != "" {
		args = append(args,
			"-e", "GIT_AUTHOR_NAME="+opts.Identity.Name,
			"-e", "GIT_COMMITTER_NAME="+opts.Identity.Name)
	}
	if opts.Identity.Email != "" {
		args = append(args,
			"-e", "GIT_AUTHOR_EMAIL="+opts.Identity.Email,
			"-e", "GIT_COMMITTER_EMAIL="+opts.Identity.Email)
	}

	// SSH agent forwarding, so Git pushes can authenticate as you.
	if host, ok, _ := sshAgentHostSocket(goos, opts.Runtime.Name, opts.SSHSock); ok {
		args = append(args, "-v", host+":"+containerSSHSock, "-e", "SSH_AUTH_SOCK="+containerSSHSock)
	}

	args = append(args, c.Image, "bash", "-lc", entrypoint(c.Startup))
	return args
}

// `networkArgs` maps a network policy to its `docker run` networking arguments:
//
//	none      → --network none
//	allowlist → --network <isolated net> + proxy env vars
//	full      → no network args
func networkArgs(mode string, proxy *proxySidecar) []string {
	switch mode {
	case config.NetworkNone:
		return []string{"--network", "none"}
	case config.NetworkAllowlist:
		return append([]string{"--network", proxy.internalNet}, proxy.envArgs()...)
	default: // full
		return nil
	}
}

// `entrypoint` enters the repo's own dev shell and runs the startup command.
// preflightFlake guarantees a tracked flake.nix before we get here, so we can
// always `nix develop` — no need to handle a missing flake.
func entrypoint(startup string) string {
	if startup == "" {
		startup = "bash"
	}
	return fmt.Sprintf("exec nix develop %s --command %s", workdir, startup)
}
