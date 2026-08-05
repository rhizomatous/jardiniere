package api

import (
	"context"
	"slices"
)

// Fake is an in-memory [Service] for testing the CLI and the TUI without a
// store, a runner, or a container runtime.
type Fake struct {
	// Sandboxes is the fake's whole world, in list order.
	Sandboxes []Sandbox
	// Err, when set, is returned by every method.
	Err error
	// Calls records method names in the order they were called.
	Calls []string
}

var _ Service = (*Fake)(nil)

// NewFake returns an empty fake service.
func NewFake(sandboxes ...Sandbox) *Fake {
	return &Fake{Sandboxes: sandboxes}
}

func (f *Fake) record(name string) error {
	f.Calls = append(f.Calls, name)
	return f.Err
}

// Create appends a sandbox in the created state.
func (f *Fake) Create(_ context.Context, spec Spec) (Sandbox, error) {
	if err := f.record("Create"); err != nil {
		return Sandbox{}, err
	}
	if _, ok := f.find(ByName(spec.Name)); ok {
		return Sandbox{}, ErrExists
	}
	sb := Sandbox{Spec: spec, State: State{Status: StatusCreated}}
	f.Sandboxes = append(f.Sandboxes, sb)
	return sb, nil
}

// List returns every sandbox the fake holds.
func (f *Fake) List(context.Context) ([]Sandbox, error) {
	if err := f.record("List"); err != nil {
		return nil, err
	}
	return slices.Clone(f.Sandboxes), nil
}

// Inspect returns one sandbox by reference.
func (f *Fake) Inspect(_ context.Context, ref Ref) (Sandbox, error) {
	if err := f.record("Inspect"); err != nil {
		return Sandbox{}, err
	}
	i, ok := f.find(ref)
	if !ok {
		return Sandbox{}, ErrNotFound
	}
	return f.Sandboxes[i], nil
}

// Start marks a sandbox running.
func (f *Fake) Start(_ context.Context, ref Ref) error {
	return f.setStatus("Start", ref, StatusRunning)
}

// Stop marks a sandbox stopped.
func (f *Fake) Stop(_ context.Context, ref Ref) error {
	return f.setStatus("Stop", ref, StatusStopped)
}

// Remove drops a sandbox, refusing a running one unless force is set.
func (f *Fake) Remove(_ context.Context, ref Ref, force bool) error {
	if err := f.record("Remove"); err != nil {
		return err
	}
	i, ok := f.find(ref)
	if !ok {
		return ErrNotFound
	}
	if f.Sandboxes[i].State.Status == StatusRunning && !force {
		return ErrRunning
	}
	f.Sandboxes = slices.Delete(f.Sandboxes, i, i+1)
	return nil
}

// Exec reports a clean exit for any known sandbox.
func (f *Fake) Exec(_ context.Context, ref Ref, _ ExecRequest) (ExecResult, error) {
	if err := f.record("Exec"); err != nil {
		return ExecResult{}, err
	}
	if _, ok := f.find(ref); !ok {
		return ExecResult{}, ErrNotFound
	}
	return ExecResult{}, nil
}

// Copy succeeds for any known sandbox.
func (f *Fake) Copy(_ context.Context, src, dst Path) error {
	if err := f.record("Copy"); err != nil {
		return err
	}
	ref, err := CopyRef(src, dst)
	if err != nil {
		return err
	}
	if _, ok := f.find(ref); !ok {
		return ErrNotFound
	}
	return nil
}

// Close does nothing.
func (f *Fake) Close() error { return f.record("Close") }

func (f *Fake) setStatus(call string, ref Ref, status Status) error {
	if err := f.record(call); err != nil {
		return err
	}
	i, ok := f.find(ref)
	if !ok {
		return ErrNotFound
	}
	f.Sandboxes[i].State.Status = status
	return nil
}

// find locates a sandbox by name, then by primary workspace path.
func (f *Fake) find(ref Ref) (int, bool) {
	for i, sb := range f.Sandboxes {
		if ref.Name != "" && sb.Spec.Name == ref.Name {
			return i, true
		}
		if ref.Path != "" && sb.Spec.Primary().Host == ref.Path {
			return i, true
		}
	}
	return 0, false
}
