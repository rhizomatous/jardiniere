// jard drops you into an isolated, Nix-based dev sandbox for a repo.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/vivshaw/jardiniere/internal/config"
	"github.com/vivshaw/jardiniere/internal/runtime"
	"github.com/vivshaw/jardiniere/internal/sandbox"
	"github.com/vivshaw/jardiniere/internal/ui"
)

// version is exactly what it says on the tin.
var version = "dev"

// cli is jard's command-line interface.
type cli struct {
	Dir     string           `default:"." help:"target repository directory to sandbox"`
	Image   string           `help:"override the base runner image"`
	DryRun  bool             `help:"print the container command instead of running it"`
	Version kong.VersionFlag `help:"print version and exit"`
}

func main() {
	if err := run(); err != nil {
		ui.Log.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	var cli cli
	kong.Parse(&cli,
		kong.Name("jard"),
		kong.Description("drops you into an isolated, Nix-based dev sandbox for a repo"),
		kong.UsageOnError(),
		kong.Vars{"version": buildVersion()},
	)

	repoDir, err := filepath.Abs(cli.Dir)
	if err != nil {
		return fmt.Errorf("resolving --dir: %w", err)
	}
	if fi, err := os.Stat(repoDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", repoDir)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		return err
	}
	if cli.Image != "" {
		cfg.Image = cli.Image
	}

	if err := preflightFlake(repoDir); err != nil {
		return err
	}

	// Ctrl-C should tear down the sandbox cleanly rather than orphan it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	detect := runtime.Detect
	if cli.DryRun {
		detect = runtime.DetectInstalled
	}
	rt, err := detect(ctx)
	if err != nil {
		return err
	}

	identity := hostGitIdentity()
	sshSock := os.Getenv("SSH_AUTH_SOCK")

	opts := sandbox.Options{
		Runtime:  rt,
		Config:   cfg,
		RepoDir:  repoDir,
		Identity: identity,
		SSHSock:  sshSock,
		DryRun:   cli.DryRun,
	}

	if !cli.DryRun {
		forwarded, detail := sandbox.SSHAgentStatus(rt.Name, sshSock)
		fmt.Fprintln(os.Stderr, ui.RenderSummary(ui.Summary{
			Runtime:      rt.Name,
			Image:        cfg.Image,
			Startup:      startupLabel(cfg),
			Network:      cfg.Network.Mode,
			AllowCount:   len(cfg.Network.Allow),
			MountCount:   len(cfg.Mounts),
			SSHForwarded: forwarded,
			SSHDetail:    detail,
			Identity:     identity.Label(),
		}))
	}
	return sandbox.Run(ctx, opts)
}

// prefligthFlake checks that there's a valid git flake.
func preflightFlake(repoDir string) error {
	if _, err := os.Stat(filepath.Join(repoDir, "flake.nix")); err != nil {
		return errors.New("no flake.nix found")
	}
	// non-Git directory: flake presence is enough.
	if !insideGitWorkTree(repoDir) {
		return nil
	}
	// Git repo: nix develop reads only tracked files, so flake must be tracked.
	if exec.Command("git", "-C", repoDir, "ls-files", "--error-unmatch", "flake.nix").Run() != nil {
		return errors.New("flake.nix is not tracked by git")
	}
	return nil
}

// insideGitWorkTree reports whether dir is within a git working tree.
func insideGitWorkTree(dir string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run() == nil
}

// hostGitIdentity reads the user's commit identity from their global git config
// so sandbox commits are authored as them. missing values are omitted.
func hostGitIdentity() sandbox.GitIdentity {
	return sandbox.GitIdentity{
		Name:  gitConfig("user.name"),
		Email: gitConfig("user.email"),
	}
}

func gitConfig(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// buildVersion resolves the string shown by --version. it prefers a value
// injected via -ldflags, then falls back to VCS build info if absent.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	// set when installed via `go install ...@version`
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	// otherwise synthesize from the embedded git revision
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev := s.Value
			if len(rev) > 12 {
				rev = rev[:12]
			}
			return "dev-" + rev
		}
	}
	return version
}

func startupLabel(c config.Config) string {
	if c.Startup == "" {
		return "bash"
	}
	return c.Startup
}
