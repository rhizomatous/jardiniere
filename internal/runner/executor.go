package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor runs built invocations. Swapping it is how --dry-run and unit tests
// avoid a live runtime.
//
// The two methods exist because jard needs both shapes: Output for the commands
// whose result it parses, Attach for the ones the user is sitting in front of.
type Executor interface {
	// Output runs inv and returns its stdout.
	Output(ctx context.Context, inv Invocation) ([]byte, error)
	// Attach runs inv with the terminal wired straight through and returns its
	// exit status. A non-zero status is the command's answer, not an error.
	Attach(ctx context.Context, inv Invocation) (int, error)
}

// hostExecutor runs invocations as real subprocesses.
type hostExecutor struct{}

func (hostExecutor) Output(ctx context.Context, inv Invocation) ([]byte, error) {
	cmd := exec.CommandContext(ctx, inv.Path, inv.Args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, invocationError(inv, stderr.Bytes(), err)
	}
	return out, nil
}

func (hostExecutor) Attach(ctx context.Context, inv Invocation) (int, error) {
	cmd := exec.CommandContext(ctx, inv.Path, inv.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := cmd.Run()
	// an agent exiting non-zero is the agent's business, not a jard failure.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 0, invocationError(inv, nil, err)
	}
	return 0, nil
}

// dryRunExecutor renders invocations instead of running them.
type dryRunExecutor struct{ w io.Writer }

func (d dryRunExecutor) Output(_ context.Context, inv Invocation) ([]byte, error) {
	_, err := fmt.Fprintln(d.w, inv)
	return nil, err
}

func (d dryRunExecutor) Attach(_ context.Context, inv Invocation) (int, error) {
	_, err := fmt.Fprintln(d.w, inv)
	return 0, err
}

// invocationError turns a failed invocation into a message that names the
// subcommand and carries the runtime's own complaint, which is almost always
// more useful than the exit status.
func invocationError(inv Invocation, stderr []byte, err error) error {
	label := filepath.Base(inv.Path)
	if len(inv.Args) > 0 {
		label += " " + inv.Args[0]
	}
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return fmt.Errorf("%s: %s (%w)", label, msg, err)
	}
	return fmt.Errorf("%s: %w", label, err)
}

// isNotFound reports whether err is a runtime complaining that a container or
// volume does not exist. Matched on the message because the runtimes offer no
// distinguishable exit status: docker says "no such", podman "not found".
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such") || strings.Contains(msg, "not found")
}
