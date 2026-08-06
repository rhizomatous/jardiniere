package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rhizomatous/jardiniere/internal/api"
	"github.com/rhizomatous/jardiniere/internal/ui"
)

// gaugeWidth is how many cells the CPU and memory bars occupy.
const gaugeWidth = 12

var (
	cursorStyle   = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	gaugeFull     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	gaugeEmpty    = lipgloss.NewStyle().Faint(true)
)

// View renders the dashboard.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	// the dashboard owns the whole screen and restores what was there on exit.
	v.AltScreen = !m.quitting
	return v
}

func (m *Model) render() string {
	var b strings.Builder
	b.WriteString(ui.Title.Render("🪴 jardinière"))
	b.WriteString("\n\n")

	// the form replaces the list rather than sitting beside it: it owns the
	// keyboard while open, and showing a list you cannot move through invites
	// the user to try.
	if m.create != nil {
		b.WriteString(m.create.form.View())
		b.WriteString("\n" + ui.Faint.Render("esc cancel"))
		return b.String()
	}

	switch {
	case m.err != nil:
		b.WriteString(ui.Bad.Render("could not read sandboxes: " + m.err.Error()))
	case len(m.sandboxes) == 0:
		b.WriteString(ui.Faint.Render("no sandboxes yet — run `jard run` in a repo to make one"))
	default:
		b.WriteString(m.renderList())
	}

	if m.status != "" {
		b.WriteString("\n\n" + ui.Faint.Render(m.status))
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// renderList draws one row per sandbox.
func (m *Model) renderList() string {
	rows := make([]string, 0, len(m.sandboxes))
	for i, sb := range m.sandboxes {
		rows = append(rows, m.renderRow(sb, i == m.cursor))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) renderRow(sb api.Sandbox, selected bool) string {
	marker, name := "  ", sb.Spec.Name
	if selected {
		marker = cursorStyle.Render("▸ ")
		name = selectedStyle.Render(name)
	}

	status := string(sb.State.Status)
	if m.pending == sb.Spec.Name {
		status = "working…"
	}

	line := marker + pad(name, 20) + " " +
		ui.StatusStyle(sb.State.Status).Render(pad(status, 9)) + " " +
		ui.Faint.Render(pad(dash(sb.Spec.Agent), 10))

	if sample, ok := m.stats[sb.Spec.Name]; ok {
		line += " " + gauge(sample.CPUPercent) + " " + ui.Faint.Render(pad(cpuLabel(sample), 7))
		line += " " + gauge(sample.MemoryPercent()) + " " + ui.Faint.Render(memLabel(sample))
	} else if ws := sb.Spec.Primary().Host; ws != "" {
		line += " " + ui.Faint.Render(ws)
	}
	return line
}

// gauge draws a proportion as a bar. CPU can exceed 100% on a multi-core
// sandbox, so the bar saturates rather than overflowing its width.
func gauge(percent float64) string {
	filled := int(percent / 100 * gaugeWidth)
	if filled > gaugeWidth {
		filled = gaugeWidth
	}
	if filled < 0 {
		filled = 0
	}
	return gaugeFull.Render(strings.Repeat("█", filled)) +
		gaugeEmpty.Render(strings.Repeat("─", gaugeWidth-filled))
}

func cpuLabel(s api.Stats) string {
	return fmt.Sprintf("%.0f%% cpu", s.CPUPercent)
}

func memLabel(s api.Stats) string {
	if s.MemoryLimit <= 0 {
		return api.FormatBytes(s.MemoryBytes)
	}
	return api.FormatBytes(s.MemoryBytes) + " / " + api.FormatBytes(s.MemoryLimit)
}

// renderFooter shows either the full key list or a one-line reminder.
func (m *Model) renderFooter() string {
	if !m.showHelp {
		return ui.Faint.Render("↑/↓ move · c create · enter attach · x shell · s start/stop · r remove · ? help · q quit")
	}
	lines := make([]string, 0, len(Keys))
	for _, k := range Keys {
		lines = append(lines, "  "+ui.Value.Render(pad(strings.Join(k.Keys, " "), 12))+ui.Faint.Render(k.Help))
	}
	return strings.Join(lines, "\n")
}

// pad right-pads s to width, measured in terminal cells so styled or wide
// characters do not skew the columns.
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
