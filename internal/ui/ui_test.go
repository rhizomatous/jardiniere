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

func TestRenderSummaryAgentRow(t *testing.T) {
	mounted := RenderSummary(Summary{Runtime: "docker", Agent: "claude-code", Mounted: true})
	if !strings.Contains(mounted, "claude-code") || !strings.Contains(mounted, "settings from host") {
		t.Errorf("expected agent row noting mounted settings, got:\n%s", mounted)
	}
	// the agent is named whether or not its settings were found.
	bare := RenderSummary(Summary{Runtime: "docker", Agent: "codex"})
	if !strings.Contains(bare, "codex") || strings.Contains(bare, "settings from host") {
		t.Errorf("expected a bare agent row, got:\n%s", bare)
	}
	// "  agent" as a row label, not the tail of "ssh-agent".
	if none := RenderSummary(Summary{Runtime: "docker"}); strings.Contains(none, "  agent") {
		t.Errorf("expected no agent row when none is injected, got:\n%s", none)
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
