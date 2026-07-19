package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	rt "github.com/vivshaw/jardiniere/internal/runtime"
	"github.com/vivshaw/jardiniere/internal/ui"
)

// we implement network = "allowlist" via a proxy sidecar.
// the sandbox container joins an --internal docker network, with no route
// to the internet. its only reachable peer is a dual-homed tinyproxy container
// that filters CONNECT by host.

const (
	proxyPort  = "8888"
	proxyImage = "nixos/nix:latest" // runs tinyproxy via nix run
	// readyMarker is tinyproxy's log line once it's accepting connections.
	readyMarker = "Accepting connections"
	// proxyReadyTimeout is generous because the first run fetches tinyproxy.
	proxyReadyTimeout = 3 * time.Minute
)

// proxySidecar holds the resources backing one allowlist run's egress proxy.
type proxySidecar struct {
	rt          rt.Runtime
	internalNet string // sandbox joins this; no internet route
	externalNet string // proxy also joins this to reach the world
	proxyName   string
	allow       []string
	tmpDir      string // host dir holding the generated tinyproxy config
	started     bool   // whether resources were actually created (for cleanup)
}

// planProxySidecar derives the names/wiring without creating anything, so
// --dry-run can show a representative command.
func planProxySidecar(opts Options) *proxySidecar {
	id := strconv.Itoa(os.Getpid())
	return &proxySidecar{
		rt:          opts.Runtime,
		internalNet: "jard-int-" + id,
		externalNet: "jard-ext-" + id,
		proxyName:   "jard-proxy-" + id,
		allow:       opts.Config.Network.Allow,
	}
}

// proxyURL is where the sandbox reaches the proxy over the internal network,
// resolved by docker's embedded DNS.
func (p *proxySidecar) proxyURL() string {
	return "http://" + p.proxyName + ":" + proxyPort
}

// envArgs points the sandbox's clients at the proxy. without these,
// the agent won't know how to connect to anything.
func (p *proxySidecar) envArgs() []string {
	u := p.proxyURL()
	return []string{
		"-e", "HTTP_PROXY=" + u, "-e", "HTTPS_PROXY=" + u,
		"-e", "http_proxy=" + u, "-e", "https_proxy=" + u,
		"-e", "NO_PROXY=localhost,127.0.0.1", "-e", "no_proxy=localhost,127.0.0.1",
	}
}

// start creates the networks, launches the filtering proxy, and blocks until
// it's ready. n.b., on any failure the caller must still call cleanup.
func (p *proxySidecar) start(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "jard-proxy-")
	if err != nil {
		return fmt.Errorf("creating proxy config dir: %w", err)
	}
	p.tmpDir = dir
	if err := os.WriteFile(filepath.Join(dir, "tinyproxy.conf"), []byte(tinyproxyConf()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "filter"), []byte(filterFile(p.allow)), 0o644); err != nil {
		return err
	}

	p.started = true // from here on, cleanup has resources to remove
	if err := p.docker(ctx, "network", "create", "--internal", p.internalNet); err != nil {
		return err
	}
	if err := p.docker(ctx, "network", "create", p.externalNet); err != nil {
		return err
	}
	// start on the external network so nix run can fetch tinyproxy...
	if err := p.docker(ctx, "run", "-d", "--name", p.proxyName, "--network", p.externalNet,
		"-v", dir+":/jard-proxy:ro", "-v", nixStoreVolume+":/nix",
		"-e", "NIX_CONFIG=experimental-features = nix-command flakes",
		proxyImage, "nix", "run", "nixpkgs#tinyproxy", "--", "-d", "-c", "/jard-proxy/tinyproxy.conf"); err != nil {
		return err
	}
	// ...then also attach the internal network the sandbox will use.
	if err := p.docker(ctx, "network", "connect", p.internalNet, p.proxyName); err != nil {
		return err
	}

	ui.Log.Info("setting up egress allowlist proxy (first run fetches tinyproxy)…")
	return p.waitReady(ctx)
}

// waitReady polls the proxy's logs for the ready marker. we read logs rather
// than docker exec because exec-ing a shell trips an "openat etc/group"
// error on some OrbStack setups.
func (p *proxySidecar) waitReady(ctx context.Context) error {
	deadline := time.Now().Add(proxyReadyTimeout)
	for time.Now().Before(deadline) {
		out, _ := exec.CommandContext(ctx, p.rt.Path, "logs", p.proxyName).CombinedOutput()
		if strings.Contains(string(out), readyMarker) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	out, _ := exec.Command(p.rt.Path, "logs", "--tail", "20", p.proxyName).CombinedOutput()
	return fmt.Errorf("allowlist proxy did not become ready within %s:\n%s", proxyReadyTimeout, strings.TrimSpace(string(out)))
}

// cleanup does a best-effort teardown on all resources. deliberately uses plain
// exec (not the run's context) so teardown still happens after a Ctrl-C.
func (p *proxySidecar) cleanup() {
	if p.started {
		_ = exec.Command(p.rt.Path, "rm", "-f", p.proxyName).Run()
		_ = exec.Command(p.rt.Path, "network", "rm", p.internalNet).Run()
		_ = exec.Command(p.rt.Path, "network", "rm", p.externalNet).Run()
	}
	if p.tmpDir != "" {
		_ = os.RemoveAll(p.tmpDir)
	}
}

// docker runs a runtime subcommand, surfacing combined output on failure.
func (p *proxySidecar) docker(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, p.rt.Path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", p.rt.Name, args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

// the static proxy config: default-deny, filter by host.
func tinyproxyConf() string {
	return "Port " + proxyPort + `
Listen 0.0.0.0
Timeout 600
Allow 0.0.0.0/0
FilterDefaultDeny Yes
Filter "/jard-proxy/filter"
FilterType ere
FilterCaseSensitive Off
`
}

// filterFile renders one extended-regex per allowed host. each pattern matches
// the host exactly and any subdomain of it, e.g. "github.com" also permits
// "api.github.com". hosts are regex-escaped, so dots are literal.
func filterFile(allow []string) string {
	var b strings.Builder
	for _, h := range allow {
		b.WriteString(`(^|\.)` + regexp.QuoteMeta(strings.TrimSpace(h)) + "$\n")
	}
	return b.String()
}
