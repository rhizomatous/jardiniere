package sandbox

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivshaw/jardiniere/internal/config"
	rt "github.com/vivshaw/jardiniere/internal/runtime"
)

func TestSSHAgentHostSocket(t *testing.T) {
	tests := []struct {
		name             string
		goos, rt, hostSk string
		wantPath         string
		wantOK           bool
		wantReasonSubstr string // "" means reason must be empty
	}{
		{"no host agent", "darwin", "docker", "", "", false, ""},
		{"linux mounts real socket", "linux", "docker", "/tmp/agent.sock", "/tmp/agent.sock", true, ""},
		{"darwin docker uses vm bridge", "darwin", "docker", "/var/folders/x/agent", dockerVMSSHSock, true, ""},
		{"darwin podman unsupported", "darwin", "podman", "/var/folders/x/agent", "", false, "podman"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, ok, reason := sshAgentHostSocket(tc.goos, tc.rt, tc.hostSk)
			if path != tc.wantPath || ok != tc.wantOK {
				t.Errorf("got (%q, %v), want (%q, %v)", path, ok, tc.wantPath, tc.wantOK)
			}
			if tc.wantReasonSubstr == "" && reason != "" {
				t.Errorf("expected no reason, got %q", reason)
			}
			if tc.wantReasonSubstr != "" && !strings.Contains(reason, tc.wantReasonSubstr) {
				t.Errorf("reason %q missing %q", reason, tc.wantReasonSubstr)
			}
		})
	}
}

func TestNetworkArgs(t *testing.T) {
	if got := networkArgs(config.NetworkNone, nil); !slices.Equal(got, []string{"--network", "none"}) {
		t.Errorf("none: got %v", got)
	}
	if got := networkArgs(config.NetworkFull, nil); got != nil {
		t.Errorf("full: got %v, want nil", got)
	}
	p := &proxySidecar{internalNet: "jard-int-1", proxyName: "jard-proxy-1"}
	got := strings.Join(networkArgs(config.NetworkAllowlist, p), " ")
	if !strings.Contains(got, "--network jard-int-1") || !strings.Contains(got, "HTTPS_PROXY=") {
		t.Errorf("allowlist: got %v", got)
	}
}

func TestBuildArgsNetworkNone(t *testing.T) {
	base := Options{Runtime: rt.Runtime{Name: "docker"}, Config: config.Defaults(), RepoDir: "/repo"}

	none := base
	none.Config.Network = config.NetworkNone
	if args := buildArgs(none, "linux", nil, nil); !slices.Contains(args, "none") ||
		!strings.Contains(strings.Join(args, " "), "--network none") {
		t.Errorf("network=none should add `--network none`, got %v", args)
	}

	full := base // Defaults() is full
	if strings.Contains(strings.Join(buildArgs(full, "linux", nil, nil), " "), "--network") {
		t.Errorf("network=full should add no --network flag, got %v", buildArgs(full, "linux", nil, nil))
	}
}

func TestBuildArgsAllowlist(t *testing.T) {
	opts := Options{
		Runtime: rt.Runtime{Name: "docker"},
		Config:  config.Config{Network: config.NetworkAllowlist, Allow: []string{"github.com"}},
		RepoDir: "/repo",
	}
	proxy := planProxySidecar(opts)
	joined := strings.Join(buildArgs(opts, "linux", proxy, nil), " ")

	if !strings.Contains(joined, "--network "+proxy.internalNet) {
		t.Errorf("allowlist should attach the internal net %q, got %v", proxy.internalNet, joined)
	}
	if !strings.Contains(joined, "HTTPS_PROXY="+proxy.proxyURL()) {
		t.Errorf("allowlist should inject HTTPS_PROXY, got %v", joined)
	}
	if strings.Contains(joined, "--network none") {
		t.Errorf("allowlist must not also pass --network none")
	}
}

// `TestBuildArgsAgentForwarding` checks that the correct forwarding flags are
// passed to the container invocation, per platform.
func TestBuildArgsAgentForwarding(t *testing.T) {
	tests := []struct {
		name          string
		goos, runtime string
		wantForwarded bool
	}{
		{"linux docker forwards", "linux", "docker", true},
		{"linux podman forwards", "linux", "podman", true},
		{"darwin docker forwards", "darwin", "docker", true},
		{"darwin podman does not", "darwin", "podman", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{
				Runtime: rt.Runtime{Name: tc.runtime, Path: tc.runtime},
				Config:  config.Defaults(),
				RepoDir: "/repo",
				SSHSock: "/host/agent.sock",
			}
			args := buildArgs(opts, tc.goos, nil, nil)
			hasEnv := slices.Contains(args, "SSH_AUTH_SOCK="+containerSSHSock)
			hasMount := strings.Contains(strings.Join(args, " "), ":"+containerSSHSock)
			if got := hasEnv && hasMount; got != tc.wantForwarded {
				t.Errorf("forwarded=%v, want %v (args: %v)", got, tc.wantForwarded, args)
			}
		})
	}
}
