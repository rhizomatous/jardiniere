// Package direct implements api.Service in-process, against a local store and
// container runtime. It is what jard uses through phase 2; from phase 3 the
// daemon wraps this same type and api/rpc puts a socket in front of it.
package direct

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rhizomatous/jardiniere/internal/api"
	"github.com/rhizomatous/jardiniere/internal/runner"
	"github.com/rhizomatous/jardiniere/internal/store"
)

// Service is the in-process implementation of [api.Service].
type Service struct {
	store  *store.Store
	runner runner.Runner
	now    func() time.Time
}

var _ api.Service = (*Service)(nil)

// Option configures a [Service].
type Option func(*Service)

// WithClock replaces the clock used for created-at timestamps.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New returns a service backed by st and rn.
func New(st *store.Store, rn runner.Runner, opts ...Option) *Service {
	s := &Service{store: st, runner: rn, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create registers a sandbox and builds its container, without starting it.
func (s *Service) Create(ctx context.Context, spec api.Spec) (api.Sandbox, error) {
	if !store.ValidName(spec.Name) {
		return api.Sandbox{}, fmt.Errorf("invalid sandbox name %q: use letters, digits, dot, dash, or underscore", spec.Name)
	}
	if _, err := s.store.Get(spec.Name); err == nil {
		return api.Sandbox{}, fmt.Errorf("%q: %w", spec.Name, api.ErrExists)
	} else if !errors.Is(err, store.ErrNotFound) {
		return api.Sandbox{}, err
	}

	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = s.now().UTC()
	}
	id, err := s.runner.Create(ctx, spec)
	if err != nil {
		return api.Sandbox{}, err
	}

	sb := api.Sandbox{
		Spec:  spec,
		State: api.State{Status: api.StatusCreated, ContainerID: string(id)},
	}
	if err := s.store.Put(sb); err != nil {
		return api.Sandbox{}, err
	}
	return sb, nil
}

// List returns every known sandbox, each refreshed against the runtime.
func (s *Service) List(ctx context.Context) ([]api.Sandbox, error) {
	stored, err := s.store.List()
	if err != nil {
		return nil, err
	}
	for i, sb := range stored {
		stored[i].State = s.observe(ctx, sb)
	}
	return stored, nil
}

// Inspect returns one sandbox, refreshed against the runtime.
func (s *Service) Inspect(ctx context.Context, ref api.Ref) (api.Sandbox, error) {
	sb, err := s.find(ref)
	if err != nil {
		return api.Sandbox{}, err
	}
	sb.State = s.observe(ctx, sb)
	return sb, nil
}

// Start boots a created or stopped sandbox.
func (s *Service) Start(ctx context.Context, ref api.Ref) error {
	return s.act(ctx, ref, func(id runner.ID) error {
		return s.runner.Start(ctx, id)
	})
}

// Stop halts a running sandbox, leaving its contents intact.
func (s *Service) Stop(ctx context.Context, ref api.Ref) error {
	return s.act(ctx, ref, func(id runner.ID) error {
		return s.runner.Stop(ctx, id)
	})
}

// Remove deletes a sandbox and everything in it. A running sandbox is refused
// unless force is set — the same guard `sbx rm` has, for the same reason.
func (s *Service) Remove(ctx context.Context, ref api.Ref, force bool) error {
	sb, err := s.find(ref)
	if err != nil {
		return err
	}
	if !force && s.observe(ctx, sb).Status == api.StatusRunning {
		return fmt.Errorf("%q: %w (use --force)", sb.Spec.Name, api.ErrRunning)
	}
	if err := s.runner.Remove(ctx, containerID(sb), force); err != nil {
		return err
	}
	return s.store.Delete(sb.Spec.Name)
}

// Exec runs a command inside a sandbox.
func (s *Service) Exec(ctx context.Context, ref api.Ref, req api.ExecRequest) (api.ExecResult, error) {
	sb, err := s.find(ref)
	if err != nil {
		return api.ExecResult{}, err
	}
	return s.runner.Exec(ctx, containerID(sb), req)
}

// Copy moves files between the host and a sandbox, named by whichever side of
// the copy carries one.
func (s *Service) Copy(ctx context.Context, src, dst api.Path) error {
	ref, err := api.CopyRef(src, dst)
	if err != nil {
		return err
	}
	sb, err := s.find(ref)
	if err != nil {
		return err
	}
	return s.runner.Copy(ctx, containerID(sb), src, dst)
}

// Close releases nothing: the direct service holds no long-lived handles.
func (s *Service) Close() error { return nil }

// act resolves a ref and applies fn to its container.
func (s *Service) act(ctx context.Context, ref api.Ref, fn func(runner.ID) error) error {
	sb, err := s.find(ref)
	if err != nil {
		return err
	}
	if err := fn(containerID(sb)); err != nil {
		return err
	}
	sb.State = s.observe(ctx, sb)
	return s.store.Put(sb)
}

// find resolves a ref to a stored sandbox, translating the store's not-found
// into the api's so callers only match one sentinel.
func (s *Service) find(ref api.Ref) (api.Sandbox, error) {
	sb, err := s.store.Find(ref)
	if errors.Is(err, store.ErrNotFound) {
		return api.Sandbox{}, fmt.Errorf("%v: %w", ref, api.ErrNotFound)
	}
	return sb, err
}

// observe asks the runtime for a sandbox's live state, falling back to the last
// state we recorded when the runtime can't answer. A stale status is better
// than an error on a command that only wanted to list.
func (s *Service) observe(ctx context.Context, sb api.Sandbox) api.State {
	state, err := s.runner.Inspect(ctx, containerID(sb))
	if err != nil {
		return sb.State
	}
	if state.ContainerID == "" {
		state.ContainerID = sb.State.ContainerID
	}
	return state
}

// containerID is the runtime handle for a sandbox, recovered from its name when
// the record predates one.
func containerID(sb api.Sandbox) runner.ID {
	if sb.State.ContainerID != "" {
		return runner.ID(sb.State.ContainerID)
	}
	return runner.ID(runner.ContainerName(sb.Spec.Name))
}
