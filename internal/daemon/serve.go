package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/rhizomatous/jardiniere/internal/api/direct"
	"github.com/rhizomatous/jardiniere/internal/api/rpc"
	"github.com/rhizomatous/jardiniere/internal/proxy"
)

// ErrAlreadyRunning means a daemon is already listening on the socket.
var ErrAlreadyRunning = errors.New("a jard daemon is already running")

// Options configures a daemon.
type Options struct {
	// Socket overrides where the daemon listens.
	Socket string
	// StateDir overrides where the daemon keeps sandbox records.
	StateDir string
	// ProxyAddr is where the egress proxy listens. Loopback by default: it is
	// reached from a sandbox through the relay, and binding it wider would
	// offer the whole LAN a way out through this machine.
	ProxyAddr string
	// Ready, when set, is called once the daemon is listening. Tests use it to
	// learn when it is safe to connect.
	Ready func()
}

// DefaultProxyAddr is where the egress proxy listens unless told otherwise.
//
// A fixed port rather than an ephemeral one: a sandbox is given its proxy
// address when it is created, and that address has to still be right after the
// daemon restarts.
const DefaultProxyAddr = "127.0.0.1:47821"

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

	// everything the daemon needs is stood up before the socket is, because
	// the socket is what tells the world it is ready. Bound first, a daemon
	// that then fails to start its proxy would spend its last moments
	// accepting connections it is about to drop — which is what a client sees
	// as an unexplained EOF rather than as the error that actually happened.
	connections := proxy.NewLog(0)
	svc, err := direct.Open(ctx, direct.Options{
		StateDir:      opts.StateDir,
		ConnectionLog: connections,
	})
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	stopProxy, err := serveProxy(ctx, opts.ProxyAddr, svc, connections)
	if err != nil {
		return err
	}
	defer stopProxy()

	// the pidfile still goes down before the listener, so anything that finds
	// the socket answering can rely on the record being there.
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

// serveProxy starts the egress proxy and returns a function that stops it.
//
// The policy is read from the service per request rather than captured, so
// `jard policy allow` reaches every sandbox on its next connection.
func serveProxy(ctx context.Context, addr string, svc *direct.Service, log *proxy.Log) (func(), error) {
	if addr == "" {
		addr = DefaultProxyAddr
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listening for sandbox egress on %s: %w", addr, err)
	}

	handler := proxy.NewServer(func() proxy.Policy {
		p, err := svc.Policy(ctx)
		if err != nil {
			// no policy chosen yet, or a store we could not read. Balanced is
			// the documented default and denies by default, so the failure
			// mode is a sandbox that cannot reach something rather than one
			// that can reach everything.
			return proxy.New(proxy.PresetBalanced)
		}
		return p
	}, proxy.WithLog(log))

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() { _ = srv.Serve(lis) }()

	return func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}, nil
}

// ProxyAddress reports where a daemon's egress proxy listens.
func ProxyAddress(opts Options) string {
	if opts.ProxyAddr != "" {
		return opts.ProxyAddr
	}
	return DefaultProxyAddr
}
