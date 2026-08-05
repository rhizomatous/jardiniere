package direct

import (
	"context"
	"io"
	"runtime"

	"github.com/rhizomatous/jardiniere/internal/runner"
	"github.com/rhizomatous/jardiniere/internal/store"
)

// Options describes how to assemble a service against the local host.
type Options struct {
	// StateDir overrides where sandbox records are kept.
	StateDir string
	// DryRun renders every runtime invocation to DryRunOut and executes none of
	// them. It also relaxes runtime detection, since nothing needs to run.
	DryRun bool
	// DryRunOut receives rendered invocations. Defaults to io.Discard.
	DryRunOut io.Writer
}

// Open assembles a service against the local store and container runtime. It
// exists so cmd/jard and internal/tui can get a working api.Service without
// importing internal/store or internal/runner themselves.
func Open(ctx context.Context, opts Options) (*Service, error) {
	dir := opts.StateDir
	if dir == "" {
		env, err := store.HostEnv(runtime.GOOS)
		if err != nil {
			return nil, err
		}
		if dir, err = store.Root(env); err != nil {
			return nil, err
		}
	}
	st, err := store.Open(dir)
	if err != nil {
		return nil, err
	}

	detect := runner.Detect
	var runnerOpts []runner.Option
	if opts.DryRun {
		detect = runner.DetectInstalled
		out := opts.DryRunOut
		if out == nil {
			out = io.Discard
		}
		runnerOpts = append(runnerOpts, runner.WithDryRun(out))
	}

	// a runtime we can't reach isn't necessarily fatal. hand back a runner that fails every
	// call with this error, so a failure only surfaces on the first command that needs a runner.
	rt, err := detect(ctx)
	if err != nil {
		return New(st, runner.Unavailable(err)), nil
	}
	return New(st, runner.NewOCI(rt, runnerOpts...)), nil
}
