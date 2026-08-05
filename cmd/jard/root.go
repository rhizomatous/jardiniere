package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rhizomatous/jardiniere/internal/api"
	"github.com/rhizomatous/jardiniere/internal/api/direct"
)

// globals are the flags every subcommand shares.
type globals struct {
	stateDir string
	dryRun   bool
	// service, when set, replaces the one open would build. Tests inject a fake
	// through it.
	service api.Service
}

// open returns the api.Service the commands run against.
func (g *globals) open(cmd *cobra.Command) (api.Service, error) {
	if g.service != nil {
		return g.service, nil
	}
	return direct.Open(cmd.Context(), direct.Options{
		StateDir:  g.stateDir,
		DryRun:    g.dryRun,
		DryRunOut: cmd.OutOrStdout(),
	})
}

// withService resolves the service, runs fn, and closes it.
func (g *globals) withService(cmd *cobra.Command, fn func(context.Context, api.Service) error) error {
	svc, err := g.open(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	return fn(cmd.Context(), svc)
}

// newRootCmd builds the command tree. It takes options so tests can drive the
// whole CLI against a fake service.
func newRootCmd(opts ...rootOption) *cobra.Command {
	g := &globals{}

	root := &cobra.Command{
		Use:   "jard",
		Short: "persistent, isolated container sandboxes for coding agents",
		Long: "jard gives each coding agent a long-lived container sandbox. create one, " +
			"set it up however you like, and it stays. Packages, shell history, and agent " +
			"state persist until you remove it.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.PersistentFlags().StringVar(&g.stateDir, "state-dir", "",
		"where sandbox records are kept (default: XDG data dir)")
	root.PersistentFlags().BoolVar(&g.dryRun, "dry-run", false,
		"print the container commands instead of running them")

	root.AddCommand(newLsCmd(g))

	for _, opt := range opts {
		opt(root, g)
	}
	return root
}

// rootOption tweaks the command tree at construction. Only tests use these.
type rootOption func(*cobra.Command, *globals)

// withService makes every subcommand run against svc.
func withService(svc api.Service) rootOption {
	return func(_ *cobra.Command, g *globals) { g.service = svc }
}
