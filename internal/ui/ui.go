// `ui` holds jard's terminal presentation: a shared logger and the run summary.
//
// styling degrades to plain text automatically when output is piped
// or redirected.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// `Log` is the shared stderr logger.
var Log = log.NewWithOptions(os.Stderr, log.Options{ReportTimestamp: false})

// colors chosen to read on both light and dark terminals.
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	labelStyle = lipgloss.NewStyle().Faint(true).Width(11)
	valueStyle = lipgloss.NewStyle().Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	arrowStyle = lipgloss.NewStyle().Faint(true)
)

// `Summary` is the data shown before a sandbox starts.
type Summary struct {
	Runtime      string // e.g. "docker"
	Image        string // e.g. "nixos/nix:latest"
	Startup      string // e.g. "claude"
	Network      string // "none" | "full" | "allowlist"
	AllowCount   int    // number of allowlisted hosts (allowlist mode)
	MountCount   int    // number of extra host mounts (row hidden when 0)
	SSHForwarded bool
	SSHDetail    string // shown when SSHForwarded is false
	Identity     string // "viv shaw <hey@vivsha.ws>", or "" when unset
}

// `RenderSummary` returns the multi-line block printed to stderr before entering
// the sandbox.
func RenderSummary(s Summary) string {
	row := func(label, val string) string {
		return "  " + labelStyle.Render(label) + valueStyle.Render(val)
	}

	ssh := okStyle.Render("forwarded ✓")
	if !s.SSHForwarded {
		ssh = warnStyle.Render("off — " + s.SSHDetail)
	}

	identity := s.Identity
	if identity == "" {
		identity = warnStyle.Render("unset — commits won't be attributed")
	} else {
		identity = valueStyle.Render(identity)
	}

	// color-code the security posture
	var network string
	switch s.Network {
	case "none":
		network = okStyle.Render("none — no network access")
	case "full":
		network = warnStyle.Render("full — no isolation")
	case "allowlist":
		network = okStyle.Render(fmt.Sprintf("allowlist — %d host(s) permitted", s.AllowCount))
	default:
		network = valueStyle.Render(s.Network)
	}

	lines := []string{
		titleStyle.Render("🪴 jardinière"),
		row("runtime", s.Runtime),
		row("image", s.Image),
		row("startup", s.Startup),
		"  " + labelStyle.Render("network") + network,
		"  " + labelStyle.Render("ssh-agent") + ssh,
		"  " + labelStyle.Render("commits") + identity,
	}
	if s.MountCount > 0 {
		lines = append(lines, row("mounts", fmt.Sprintf("%d extra path(s)", s.MountCount)))
	}
	lines = append(lines, arrowStyle.Render("  › entering sandbox…"))
	return strings.Join(lines, "\n")
}
