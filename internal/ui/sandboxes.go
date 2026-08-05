package ui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// SandboxColumns are the headers of the `jard ls` table, in order.
var SandboxColumns = []string{"NAME", "STATUS", "AGENT", "IMAGE", "WORKSPACE", "CREATED"}

// RenderSandboxes returns the `jard ls` table. An empty list renders a single
// hint line rather than a bare header, which reads better on a fresh install.
func RenderSandboxes(sandboxes []api.Sandbox, now time.Time) string {
	if len(sandboxes) == 0 {
		return Faint.Render("no sandboxes yet")
	}

	rows := make([][]string, 0, len(sandboxes))
	for _, sb := range sandboxes {
		rows = append(rows, []string{
			sb.Spec.Name,
			string(sb.State.Status),
			dash(sb.Spec.Agent),
			dash(sb.Spec.Image),
			dash(sb.Spec.Primary().Host),
			Age(sb.Spec.CreatedAt, now),
		})
	}

	widths := columnWidths(SandboxColumns, rows)
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, renderRow(SandboxColumns, widths, headerStyle))
	for _, row := range rows {
		lines = append(lines, renderRow(row, widths, cellStyle))
	}
	return strings.Join(lines, "\n")
}

// headerStyle renders every header cell the same way.
func headerStyle(_ int, cell string) string { return Header.Render(cell) }

// cellStyle picks the style for one cell by column, so status reads as a
// posture rather than a word.
func cellStyle(col int, cell string) string {
	switch col {
	case 0:
		return Value.Render(cell)
	case 1:
		return StatusStyle(api.Status(cell)).Render(cell)
	default:
		return cell
	}
}

// StatusStyle colors a status by how much attention it wants.
func StatusStyle(s api.Status) lipgloss.Style {
	switch s {
	case api.StatusRunning:
		return OK
	case api.StatusStopped, api.StatusCreated:
		return Faint
	case api.StatusMissing:
		return Bad
	default:
		return Warn
	}
}

// Age renders how long ago t was, in the coarsest unit that still says
// something. A zero time renders as "-".
func Age(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	default:
		return plural(int(d.Hours()/24), "day")
	}
}

func plural(n int, unit string) string {
	s := strconv.Itoa(n) + " " + unit
	if n != 1 {
		s += "s"
	}
	return s + " ago"
}

// columnWidths measures each column against its header and every cell.
func columnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

// renderRow pads each cell to its column width and styles it. The last column
// is left unpadded so there's no trailing whitespace.
func renderRow(cells []string, widths []int, style func(col int, cell string) string) string {
	var b strings.Builder
	for i, cell := range cells {
		b.WriteString(style(i, cell))
		if i == len(cells)-1 {
			break
		}
		b.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)+2))
	}
	return b.String()
}

// dash renders an empty value as a placeholder, so columns never look skipped.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
