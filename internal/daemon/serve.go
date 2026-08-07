package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"github.com/rhizomatous/jardiniere/internal/api/direct"
	"github.com/rhizomatous/jardiniere/internal/api/rpc"
)

// ErrAlreadyRunning means a daemon is already listening on the socket.
var ErrAlreadyRunning = errors.New("a jard daemon is already running")

// Options configures a daemon.
type Options struct {
	// Socket overrides where the daemon listens.
	Socket string
	// StateDir overrides where the daemon keeps sandbox records.
	StateDir string
	// Ready, when set, is called once the daemon is listening. Tests use it to
	// learn when it is safe to connect.
	Ready func()
}

// Serve runs the daemon until ctx is cancelled.
//
// It refuses to start alongside another daemon, and clears a socket left behind
// by one that died: a stale file is indistinguishable from a live one until you
// try to connect, so connecting is how it decides.
func Serve(ctx context.Context, opts Options) error {
	socket := opts.Socket
	if socket == "" {
		var err error
		if socket, err = Socket(HostEnv(runtime.GOOS)); err != nil {
			return err
		}
	}
	if err := ensureRuntimeDir(socket); err != nil {
		return fmt.Errorf("preparing the runtime directory: %w", err)
	}
	if alive(ctx, socket) {
		return fmt.Errorf("%w at %s", ErrAlreadyRunning, socket)
	}
	// nothing answered, so whatever is at that path is a corpse.
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clearing a stale socket: %w", err)
	}

	// the pidfile goes down before the listener, so that anything which finds
	// the socket answering can rely on the record being there. Written after,
	// it would race every client that connects and immediately asks who is
	// serving.
	if err := writePid(opts); err != nil {
		return err
	}
	defer removePid(opts)

	lis, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socket, err)
	}
	defer func() { _ = os.Remove(socket) }()

	// the socket is an unauthenticated door onto every sandbox. Only its owner
	// gets to open it.
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = lis.Close()
		return fmt.Errorf("securing %s: %w", socket, err)
	}

	svc, err := direct.Open(ctx, direct.Options{StateDir: opts.StateDir})
	if err != nil {
		_ = lis.Close()
		return err
	}
	defer func() { _ = svc.Close() }()

	server := grpc.NewServer()
	rpc.NewServer(svc).Register(server)

	served := make(chan error, 1)
	go func() { served <- server.Serve(lis) }()

	if opts.Ready != nil {
		opts.Ready()
	}

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
		// let sessions in flight finish rather than cutting the terminal out
		// from under whoever is sitting in one.
		server.GracefulStop()
		return nil
	}
}

// alive reports whether a daemon answers on socket.
func alive(ctx context.Context, socket string) bool {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Running reports the pid of the daemon answering on socket, and whether one
// does. A pidfile alone is not evidence: the process it names may be gone, or
// may be something else entirely by now.
func Running(ctx context.Context, env Env) (pid int, ok bool) {
	socket, err := Socket(env)
	if err != nil || !alive(ctx, socket) {
		return 0, false
	}
	path, err := PidPath(env)
	if err != nil {
		return 0, true // answering, but we cannot say as whom
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, true
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, true
	}
	return pid, true
}

func writePid(opts Options) error {
	path, err := pidPathFor(opts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

func removePid(opts Options) {
	if path, err := pidPathFor(opts); err == nil {
		_ = os.Remove(path)
	}
}

// pidPathFor puts the pidfile beside whichever socket this daemon is using, so
// a daemon on a custom socket does not overwrite the default one's record.
func pidPathFor(opts Options) (string, error) {
	if opts.Socket != "" {
		return replaceBase(opts.Socket, pidFile), nil
	}
	return PidPath(HostEnv(runtime.GOOS))
}

// replaceBase swaps a path's filename, keeping its directory.
func replaceBase(path, name string) string {
	return filepath.Join(filepath.Dir(path), name)
}
