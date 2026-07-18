// `ui` holds jard's terminal presentation: a shared logger and the run summary.
//
// styling degrades to plain text automatically when output is piped
// or redirected.
package ui

import (
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

	lines := []string{
		titleStyle.Render("🪴 jardinière"),
		row("runtime", s.Runtime),
		row("image", s.Image),
		row("startup", s.Startup),
		"  " + labelStyle.Render("ssh-agent") + ssh,
		"  " + labelStyle.Render("commits") + identity,
		arrowStyle.Render("  › entering sandbox…"),
	}
	return strings.Join(lines, "\n")
}
