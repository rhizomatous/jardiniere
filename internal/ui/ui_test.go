package ui

import (
	"strings"
	"testing"
)

func TestRenderSummaryHappyPath(t *testing.T) {
	out := RenderSummary(Summary{
		Runtime:      "docker",
		Image:        "nixos/nix:latest",
		Startup:      "claude",
		SSHForwarded: true,
		Identity:     "viv shaw <hey@vivsha.ws>",
	})
	for _, want := range []string{"docker", "nixos/nix:latest", "claude", "forwarded", "viv shaw"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSummaryFlagsMissingBits(t *testing.T) {
	out := RenderSummary(Summary{
		Runtime:      "podman",
		SSHForwarded: false,
		SSHDetail:    "no host agent running",
		Identity:     "",
	})
	if !strings.Contains(out, "no host agent running") {
		t.Errorf("expected ssh-off detail, got:\n%s", out)
	}
	if !strings.Contains(out, "unset") {
		t.Errorf("expected unset-identity warning, got:\n%s", out)
	}
}
