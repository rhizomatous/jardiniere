package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rhizomatous/jardiniere/internal/api"
)

func testOCI(opts ...Option) *OCI {
	return NewOCI(Runtime{Name: "docker", Path: "/usr/bin/docker"}, opts...)
}

// argsAfter returns the value following flag, and whether it was present.
func argsAfter(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// allAfter returns every value following each occurrence of flag.
func allAfter(args []string, flag string) []string {
	var out []string
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			out = append(out, args[i+1])
		}
	}
	return out
}

func TestCreateInvocationCoreShape(t *testing.T) {
	inv := testOCI().CreateInvocation(api.Spec{Name: "demo", Image: "base:1"})

	if inv.Path != "/usr/bin/docker" {
		t.Errorf("path = %q, want the detected runtime binary", inv.Path)
	}
	if inv.Args[0] != "create" {
		t.Errorf("args[0] = %q, want create — a sandbox is built, not run", inv.Args[0])
	}
	if name, _ := argsAfter(inv.Args, "--name"); name != "jard-demo" {
		t.Errorf("--name = %q, want jard-demo", name)
	}
	if vol, _ := argsAfter(inv.Args, "--volume"); vol != "jard-demo-home:/home/agent" {
		t.Errorf("--volume = %q, want the home volume that makes a sandbox persistent", vol)
	}
	// image then the idle command, in that order, at the tail.
	tail := inv.Args[len(inv.Args)-3:]
	if tail[0] != "base:1" || tail[1] != "sleep" || tail[2] != "infinity" {
		t.Errorf("tail = %v, want the image followed by the idle command", tail)
	}
}

func TestCreateInvocationNeverPrivileged(t *testing.T) {
	inv := testOCI().CreateInvocation(api.Spec{Name: "demo", Image: "base:1"})
	for _, a := range inv.Args {
		if a == "--privileged" {
			t.Fatal("sandboxes must never run privileged")
		}
	}
}

func TestCreateInvocationWorkspacesBindAtHostPaths(t *testing.T) {
	inv := testOCI().CreateInvocation(api.Spec{
		Name:  "demo",
		Image: "base:1",
		Workspaces: []api.Workspace{
			{Host: "/home/viv/project"},
			{Host: "/home/viv/shared", ReadOnly: true},
		},
	})

	vols := allAfter(inv.Args, "--volume")
	want := []string{
		"jard-demo-home:/home/agent",
		"/home/viv/project:/home/viv/project",
		"/home/viv/shared:/home/viv/shared:ro",
	}
	if len(vols) != len(want) {
		t.Fatalf("volumes = %v, want %v", vols, want)
	}
	for i := range want {
		if vols[i] != want[i] {
			t.Errorf("volume %d = %q, want %q", i, vols[i], want[i])
		}
	}
	if wd, _ := argsAfter(inv.Args, "--workdir"); wd != "/home/viv/project" {
		t.Errorf("--workdir = %q, want the primary workspace", wd)
	}
}

func TestCreateInvocationResourcesAndPorts(t *testing.T) {
	inv := testOCI().CreateInvocation(api.Spec{
		Name:      "demo",
		Image:     "base:1",
		Resources: api.Resources{CPUs: 2.5, Memory: 8 << 30},
		Ports: []api.Port{
			{Host: 3000, Sandbox: 3000},
			{Host: 5353, Sandbox: 53, Proto: "udp"},
		},
	})

	if cpus, _ := argsAfter(inv.Args, "--cpus"); cpus != "2.5" {
		t.Errorf("--cpus = %q, want 2.5", cpus)
	}
	if mem, _ := argsAfter(inv.Args, "--memory"); mem != "8589934592" {
		t.Errorf("--memory = %q, want the byte count", mem)
	}
	ports := allAfter(inv.Args, "--publish")
	if len(ports) != 2 || ports[0] != "3000:3000" || ports[1] != "5353:53/udp" {
		t.Errorf("--publish = %v, want [3000:3000 5353:53/udp]", ports)
	}
}

func TestCreateInvocationOmitsUnsetLimits(t *testing.T) {
	inv := testOCI().CreateInvocation(api.Spec{Name: "demo", Image: "base:1"})
	for _, flag := range []string{"--cpus", "--memory", "--publish", "--env", "--workdir"} {
		if _, ok := argsAfter(inv.Args, flag); ok {
			t.Errorf("%s should be omitted when unset, so the runtime's own default applies", flag)
		}
	}
}

func TestCreateInvocationEnvIsSorted(t *testing.T) {
	// map iteration is random; a rendered command has to be stable to be useful
	// under --dry-run and in tests.
	spec := api.Spec{
		Name:  "demo",
		Image: "base:1",
		Env:   map[string]string{"ZED": "3", "ALPHA": "1", "MID": "2"},
	}
	want := []string{"ALPHA=1", "MID=2", "ZED=3"}
	for range 20 {
		got := allAfter(testOCI().CreateInvocation(spec).Args, "--env")
		if len(got) != len(want) {
			t.Fatalf("env = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("env = %v, want %v", got, want)
			}
		}
	}
}

func TestExecInvocation(t *testing.T) {
	inv := testOCI().ExecInvocation("jard-demo", api.ExecRequest{
		Cmd:         []string{"bash", "-lc", "echo hi"},
		Workdir:     "/home/viv/project",
		User:        "agent",
		Interactive: true,
		TTY:         true,
	})

	joined := strings.Join(inv.Args, " ")
	for _, want := range []string{"exec", "--interactive", "--tty", "--workdir /home/viv/project", "--user agent"} {
		if !strings.Contains(joined, want) {
			t.Errorf("exec args %q missing %q", joined, want)
		}
	}
	// the container must come before the command, or the runtime reads the
	// command as the container.
	tail := inv.Args[len(inv.Args)-4:]
	if tail[0] != "jard-demo" || tail[1] != "bash" {
		t.Errorf("tail = %v, want the container followed by the command", tail)
	}
}

func TestExecInvocationOmitsUnrequestedTTY(t *testing.T) {
	inv := testOCI().ExecInvocation("jard-demo", api.ExecRequest{Cmd: []string{"ls"}})
	for _, a := range inv.Args {
		if a == "--tty" || a == "--interactive" {
			t.Errorf("%s should not be set for a non-interactive exec", a)
		}
	}
}

func TestCopyInvocationRewritesSandboxSide(t *testing.T) {
	o := testOCI()

	in := o.CopyInvocation("jard-demo", api.Path{Path: "/tmp/a"}, api.Path{Sandbox: "demo", Path: "/home/agent/a"})
	if got := in.Args[1:]; got[0] != "/tmp/a" || got[1] != "jard-demo:/home/agent/a" {
		t.Errorf("host→sandbox = %v, want the sandbox side prefixed with the container", got)
	}

	out := o.CopyInvocation("jard-demo", api.Path{Sandbox: "demo", Path: "/home/agent/a"}, api.Path{Path: "/tmp/a"})
	if got := out.Args[1:]; got[0] != "jard-demo:/home/agent/a" || got[1] != "/tmp/a" {
		t.Errorf("sandbox→host = %v, want the sandbox side prefixed with the container", got)
	}
}

func TestDefaultExecutorDeclinesToRun(t *testing.T) {
	// phase 0 builds invocations but does not execute them.
	_, err := testOCI().Create(context.Background(), api.Spec{Name: "demo", Image: "base:1"})
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("err = %v, want ErrNotImplemented", err)
	}
}

func TestDryRunRendersWithoutExecuting(t *testing.T) {
	var out strings.Builder
	o := testOCI(WithDryRun(&out))

	id, err := o.Create(context.Background(), api.Spec{Name: "demo", Image: "base:1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != "jard-demo" {
		t.Errorf("id = %q, want jard-demo", id)
	}
	line := out.String()
	if !strings.HasPrefix(line, "/usr/bin/docker create --name jard-demo") {
		t.Errorf("rendered %q, want the create command line", line)
	}
	if !strings.HasSuffix(strings.TrimSpace(line), "base:1 sleep infinity") {
		t.Errorf("rendered %q, want it to end with the image and idle command", line)
	}
}

func TestDryRunCoversEveryMutation(t *testing.T) {
	var out strings.Builder
	o := testOCI(WithDryRun(&out))
	ctx := context.Background()

	if err := o.Start(ctx, "jard-demo"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := o.Stop(ctx, "jard-demo"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := o.Remove(ctx, "jard-demo", true); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("rendered %d lines, want 3", len(lines))
	}
	for i, want := range []string{"start jard-demo", "stop jard-demo", "rm --volumes --force jard-demo"} {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d = %q, want it to end with %q", i, lines[i], want)
		}
	}
}

func TestRemoveDropsVolumes(t *testing.T) {
	// a sandbox's home volume has to go with it, or `jard rm` leaks disk.
	var out strings.Builder
	if err := testOCI(WithDryRun(&out)).Remove(context.Background(), "jard-demo", false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !strings.Contains(out.String(), "--volumes") {
		t.Errorf("rendered %q, want --volumes so the home volume is reclaimed", out.String())
	}
}

func TestUnavailableFailsEveryOperation(t *testing.T) {
	sentinel := errors.New("no container runtime found")
	r := Unavailable(sentinel)
	ctx := context.Background()

	if _, err := r.Create(ctx, api.Spec{Name: "demo"}); !errors.Is(err, sentinel) {
		t.Errorf("Create err = %v, want the detection error", err)
	}
	if err := r.Start(ctx, "jard-demo"); !errors.Is(err, sentinel) {
		t.Errorf("Start err = %v, want the detection error", err)
	}
	if _, err := r.Inspect(ctx, "jard-demo"); !errors.Is(err, sentinel) {
		t.Errorf("Inspect err = %v, want the detection error", err)
	}
}
