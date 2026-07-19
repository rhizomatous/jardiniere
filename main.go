// jard drops you into an isolated, Nix-based dev sandbox for a repo.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/vivshaw/jardiniere/internal/config"
	"github.com/vivshaw/jardiniere/internal/runtime"
	"github.com/vivshaw/jardiniere/internal/sandbox"
	"github.com/vivshaw/jardiniere/internal/ui"
)

func main() {
	if err := run(); err != nil {
		ui.Log.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		dir    = flag.String("dir", ".", "target repository directory to sandbox")
		image  = flag.String("image", "", "override the base runner image")
		dryRun = flag.Bool("dry-run", false, "print the container command instead of running it")
	)
	flag.Parse()

	repoDir, err := filepath.Abs(*dir)
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
	if *image != "" {
		cfg.Image = *image
	}

	if err := preflightFlake(repoDir); err != nil {
		return err
	}

	// Ctrl-C should tear down the sandbox cleanly rather than orphan it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	detect := runtime.Detect
	if *dryRun {
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
		DryRun:   *dryRun,
	}

	if !*dryRun {
		forwarded, detail := sandbox.SSHAgentStatus(rt.Name, sshSock)
		fmt.Fprintln(os.Stderr, ui.RenderSummary(ui.Summary{
			Runtime:      rt.Name,
			Image:        cfg.Image,
			Startup:      startupLabel(cfg),
			Network:      cfg.Network,
			AllowCount:   len(cfg.Allow),
			MountCount:   len(cfg.Mounts),
			SSHForwarded: forwarded,
			SSHDetail:    detail,
			Identity:     identity.Label(),
		}))
	}
	return sandbox.Run(ctx, opts)
}

// check that there's a valid git flake.
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

// reads the user's commit identity from their global git config
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

func startupLabel(c config.Config) string {
	if c.Startup == "" {
		return "bash"
	}
	return c.Startup
}
