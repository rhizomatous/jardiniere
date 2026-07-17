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

// `Run` builds the container command and either executes it (inheriting the
// terminal for an interactive session) or, under DryRun, prints it.
func Run(ctx context.Context, opts Options) error {
	args, warns := buildArgs(opts)
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "jard: warning: %s\n", w)
	}

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

// `buildArgs` assembles the `run` arguments and any non-fatal warnings (e.g. an
// SSH socket we can't forward on this backend).
func buildArgs(opts Options) (args []string, warns []string) {
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

	// SSH agent forwarding for push/auth. Linux can bind-mount the host socket
	// directly. on macOS the host socket lives outside the Linux VM and needs
	// backend-specific plumbing, so v0 forwards best-effort and warns.
	if sock := opts.SSHSock; sock != "" {
		if runtime.GOOS == "linux" {
			const inSock = "/ssh-agent"
			args = append(args, "-v", sock+":"+inSock, "-e", "SSH_AUTH_SOCK="+inSock)
		} else {
			warns = append(warns,
				"ssh-agent forwarding on "+runtime.GOOS+" is backend-specific and not wired yet; "+
					"pushes needing SSH keys may fail (coming in v1)")
		}
	}

	args = append(args, c.Image, "bash", "-lc", entrypoint(c.Startup))
	return args, warns
}

// `entrypoint` runs the configured startup command inside the repo's own dev shell.
func entrypoint(startup string) string {
	if startup == "" {
		startup = "bash"
	}
	// %[1]s is reused for the flake and no-flake branches.
	return fmt.Sprintf(
		`if [ -e %[2]s/flake.nix ]; then exec nix develop %[2]s --command %[1]s; else exec %[1]s; fi`,
		startup, workdir)
}
