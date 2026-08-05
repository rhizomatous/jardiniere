// Package runner is jard's adapter boundary over the container runtimes.
// docker, podman, OrbStack, and colima differ enough — rootless podman most of
// all — to want one seam, and a fake behind the same interface is what lets the
// rest of jard be unit-tested with no live runtime.
//
// No runtime-specific type crosses this line.
package runner

import (
	"context"
	"errors"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// ErrNotImplemented marks runner surface that is declared but not yet built.
var ErrNotImplemented = errors.New("not implemented")

// ID is a runtime's own handle on a container.
type ID string

// Runner drives one container runtime.
type Runner interface {
	Create(ctx context.Context, spec api.Spec) (ID, error)
	Start(ctx context.Context, id ID) error
	Stop(ctx context.Context, id ID) error
	Remove(ctx context.Context, id ID, force bool) error
	Exec(ctx context.Context, id ID, req api.ExecRequest) (api.ExecResult, error)
	Copy(ctx context.Context, id ID, src, dst api.Path) error
	Stats(ctx context.Context, id ID) (<-chan Stats, error)
	Inspect(ctx context.Context, id ID) (api.State, error)
}

// Stats is one sample of a running sandbox's resource use, as the TUI streams
// it in phase 2.
type Stats struct {
	CPUPercent  float64
	MemoryBytes int64
	MemoryLimit int64
}
