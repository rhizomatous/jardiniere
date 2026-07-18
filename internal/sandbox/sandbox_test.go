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
		name              string
		goos, rt, hostSk  string
		wantPath          string
		wantOK            bool
		wantReasonSubstr  string // "" means reason must be empty
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
			args := buildArgs(opts, tc.goos)
			hasEnv := slices.Contains(args, "SSH_AUTH_SOCK="+containerSSHSock)
			hasMount := strings.Contains(strings.Join(args, " "), ":"+containerSSHSock)
			if got := hasEnv && hasMount; got != tc.wantForwarded {
				t.Errorf("forwarded=%v, want %v (args: %v)", got, tc.wantForwarded, args)
			}
		})
	}
}
