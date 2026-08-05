package runner

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// containerPrefix namespaces every container and volume jard owns, so a stray
// `docker ps` makes it obvious who created what.
const containerPrefix = "jard-"

// agentHome is the base image contract's home directory. It gets its own named
// volume, which is what makes a sandbox persistent: packages, shell history,
// and agent state all live under it.
const agentHome = "/home/agent"

// idle keeps a created sandbox alive with no agent attached. Sessions arrive
// later over exec.
var idle = []string{"sleep", "infinity"}

// Executor runs a built invocation. Swapping it is how --dry-run and unit tests
// avoid a live runtime.
type Executor func(ctx context.Context, inv Invocation) ([]byte, error)

// OCI drives a docker-compatible CLI: docker, podman, OrbStack, or colima.
type OCI struct {
	rt   Runtime
	exec Executor
}

var _ Runner = (*OCI)(nil)

// Option configures an [OCI] runner.
type Option func(*OCI)

// WithExecutor replaces how invocations are run.
func WithExecutor(e Executor) Option {
	return func(o *OCI) { o.exec = e }
}

// WithDryRun renders every invocation to w and executes nothing.
func WithDryRun(w io.Writer) Option {
	return WithExecutor(func(_ context.Context, inv Invocation) ([]byte, error) {
		_, err := fmt.Fprintln(w, inv)
		return nil, err
	})
}

// NewOCI returns a runner driving rt. Without [WithExecutor] or [WithDryRun] it
// builds invocations but declines to run them; live execution lands in phase 1.
func NewOCI(rt Runtime, opts ...Option) *OCI {
	o := &OCI{
		rt: rt,
		exec: func(context.Context, Invocation) ([]byte, error) {
			return nil, ErrNotImplemented
		},
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Runtime reports which container CLI this runner drives.
func (o *OCI) Runtime() Runtime { return o.rt }

// ContainerName is the runtime-side name for a sandbox.
func ContainerName(sandbox string) string { return containerPrefix + sandbox }

// HomeVolume is the named volume backing a sandbox's /home/agent.
func HomeVolume(sandbox string) string { return containerPrefix + sandbox + "-home" }

// Create builds the container without starting it.
func (o *OCI) Create(ctx context.Context, spec api.Spec) (ID, error) {
	if _, err := o.run(ctx, o.CreateInvocation(spec)); err != nil {
		return "", err
	}
	return ID(ContainerName(spec.Name)), nil
}

// Start boots a created or stopped container.
func (o *OCI) Start(ctx context.Context, id ID) error {
	_, err := o.run(ctx, o.invoke("start", string(id)))
	return err
}

// Stop halts a running container.
func (o *OCI) Stop(ctx context.Context, id ID) error {
	_, err := o.run(ctx, o.invoke("stop", string(id)))
	return err
}

// Remove deletes a container and the volumes jard created for it.
func (o *OCI) Remove(ctx context.Context, id ID, force bool) error {
	args := []string{"rm", "--volumes"}
	if force {
		args = append(args, "--force")
	}
	_, err := o.run(ctx, o.invoke(append(args, string(id))...))
	return err
}

// Exec runs a command inside a running container.
func (o *OCI) Exec(ctx context.Context, id ID, req api.ExecRequest) (api.ExecResult, error) {
	if _, err := o.run(ctx, o.ExecInvocation(id, req)); err != nil {
		return api.ExecResult{}, err
	}
	return api.ExecResult{}, nil
}

// Copy moves files between the host and a container.
func (o *OCI) Copy(ctx context.Context, id ID, src, dst api.Path) error {
	_, err := o.run(ctx, o.CopyInvocation(id, src, dst))
	return err
}

// Stats streams resource samples for a running container.
func (o *OCI) Stats(context.Context, ID) (<-chan Stats, error) {
	return nil, ErrNotImplemented
}

// Inspect reports a container's observed state.
func (o *OCI) Inspect(ctx context.Context, id ID) (api.State, error) {
	if _, err := o.run(ctx, o.InspectInvocation(id)); err != nil {
		return api.State{}, err
	}
	return api.State{Status: api.StatusUnknown}, nil
}

// CreateInvocation renders the `create` command line for spec. It is pure, so
// arg-building is testable without a runtime.
func (o *OCI) CreateInvocation(spec api.Spec) Invocation {
	name := ContainerName(spec.Name)
	args := []string{
		"create",
		"--name", name,
		"--hostname", spec.Name,
		"--label", "jard.sandbox=" + spec.Name,
		// persistence lives here: everything the user installs is under $HOME.
		"--volume", HomeVolume(spec.Name) + ":" + agentHome,
	}

	// workspaces bind in at their host paths, so paths resolve on both sides.
	for _, ws := range spec.Workspaces {
		mount := ws.Host + ":" + ws.Host
		if ws.ReadOnly {
			mount += ":ro"
		}
		args = append(args, "--volume", mount)
	}
	if primary := spec.Primary(); primary.Host != "" {
		args = append(args, "--workdir", primary.Host)
	}

	if spec.Resources.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.Resources.CPUs, 'g', -1, 64))
	}
	if spec.Resources.Memory > 0 {
		args = append(args, "--memory", strconv.FormatInt(spec.Resources.Memory, 10))
	}

	// map order is random; sort so the rendered command is stable.
	for _, k := range sortedKeys(spec.Env) {
		args = append(args, "--env", k+"="+spec.Env[k])
	}
	for _, p := range spec.Ports {
		args = append(args, "--publish", publishSpec(p))
	}

	args = append(args, spec.Image)
	args = append(args, idle...)
	return o.invoke(args...)
}

// ExecInvocation renders the `exec` command line for req.
func (o *OCI) ExecInvocation(id ID, req api.ExecRequest) Invocation {
	args := []string{"exec"}
	if req.Interactive {
		args = append(args, "--interactive")
	}
	if req.TTY {
		args = append(args, "--tty")
	}
	if req.Workdir != "" {
		args = append(args, "--workdir", req.Workdir)
	}
	if req.User != "" {
		args = append(args, "--user", req.User)
	}
	for _, k := range sortedKeys(req.Env) {
		args = append(args, "--env", k+"="+req.Env[k])
	}
	args = append(args, string(id))
	args = append(args, req.Cmd...)
	return o.invoke(args...)
}

// CopyInvocation renders the `cp` command line, rewriting sandbox-side paths to
// the runtime's "<container>:<path>" form.
func (o *OCI) CopyInvocation(id ID, src, dst api.Path) Invocation {
	return o.invoke("cp", copyEndpoint(id, src), copyEndpoint(id, dst))
}

// InspectInvocation renders the `inspect` command line, asking for just the
// state fields jard reads back.
func (o *OCI) InspectInvocation(id ID) Invocation {
	return o.invoke("inspect", "--format", "{{.State.Status}} {{.Id}} {{.State.StartedAt}} {{.State.ExitCode}}", string(id))
}

// invoke pairs args with the runtime binary.
func (o *OCI) invoke(args ...string) Invocation {
	return Invocation{Path: o.rt.Path, Args: args}
}

// run hands an invocation to the executor.
func (o *OCI) run(ctx context.Context, inv Invocation) ([]byte, error) {
	return o.exec(ctx, inv)
}

// copyEndpoint renders one side of a copy for the runtime's cp.
func copyEndpoint(id ID, p api.Path) string {
	if p.InSandbox() {
		return string(id) + ":" + p.Path
	}
	return p.Path
}

// publishSpec renders a port mapping as "host:sandbox[/proto]".
func publishSpec(p api.Port) string {
	s := strconv.Itoa(p.Host) + ":" + strconv.Itoa(p.Sandbox)
	if p.Proto != "" && p.Proto != "tcp" {
		s += "/" + p.Proto
	}
	return s
}

// sortedKeys returns m's keys in a stable order.
func sortedKeys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}
