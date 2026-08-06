package tui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// Key is one binding, as the help panel lists it.
type Key struct {
	Keys []string
	Help string
}

// Keys are the dashboard's bindings, in the order `?` shows them.
var Keys = []Key{
	{Keys: []string{"↑/k", "↓/j"}, Help: "move"},
	{Keys: []string{"c"}, Help: "create a sandbox"},
	{Keys: []string{"enter"}, Help: "attach the agent"},
	{Keys: []string{"x"}, Help: "shell"},
	{Keys: []string{"s"}, Help: "start / stop"},
	{Keys: []string{"r"}, Help: "remove"},
	{Keys: []string{"?"}, Help: "help"},
	{Keys: []string{"q"}, Help: "quit"},
}

// handleKey maps a keypress onto an action.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// while an action is in flight, only quitting and help are allowed, so a
	// second keypress cannot race the first against the same sandbox.
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	}
	if m.pending != "" {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		m.cursor--
		m.clampCursor()
	case "down", "j":
		m.cursor++
		m.clampCursor()
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.sandboxes) - 1
		m.clampCursor()
	case "c":
		return m, m.startCreate()
	case "enter":
		return m, m.attachSelected(agentCommand)
	case "x":
		return m, m.attachSelected(shellCommand)
	case "s":
		return m, m.toggleSelected()
	case "r":
		return m, m.removeSelected()
	}
	return m, nil
}

// sessionCommand decides what a sandbox is attached with.
type sessionCommand func(api.Sandbox) ([]string, error)

// agentCommand runs the sandbox's own agent, which is the binary its image
// actually has.
func agentCommand(sb api.Sandbox) ([]string, error) {
	def, err := api.LookupAgent(sb.Spec.Agent)
	if err != nil {
		return nil, err
	}
	return def.Command, nil
}

// shellCommand opens a login shell, for poking at a sandbox without the agent.
func shellCommand(api.Sandbox) ([]string, error) {
	return []string{"bash", "-l"}, nil
}

// attachSelected leaves the dashboard so the terminal can go to the session.
// Starting a stopped sandbox first is what makes Enter work from any row.
func (m *Model) attachSelected(command sessionCommand) tea.Cmd {
	sb, ok := m.selected()
	if !ok {
		return nil
	}
	cmd, err := command(sb)
	if err != nil {
		m.status = err.Error()
		return nil
	}

	m.attach = &AttachRequest{
		Sandbox: sb.Spec.Name,
		Cmd:     cmd,
		Workdir: sb.Spec.Primary().Host,
	}
	m.quitting = true
	return tea.Quit
}

// toggleSelected starts a stopped sandbox and stops a running one.
func (m *Model) toggleSelected() tea.Cmd {
	sb, ok := m.selected()
	if !ok {
		return nil
	}
	name := sb.Spec.Name
	m.pending = name

	if sb.State.Status == api.StatusRunning {
		return func() tea.Msg {
			err := m.svc.Stop(context.Background(), api.ByName(name))
			return actionMsg{verb: "stopped", name: name, err: err}
		}
	}
	return func() tea.Msg {
		err := m.svc.Start(context.Background(), api.ByName(name))
		return actionMsg{verb: "started", name: name, err: err}
	}
}

// removeSelected deletes the selected sandbox.
//
// A running one is refused rather than force-removed: the service's guard
// exists so a live session is not destroyed by a single keystroke, and the
// dashboard would defeat it by passing force.
func (m *Model) removeSelected() tea.Cmd {
	sb, ok := m.selected()
	if !ok {
		return nil
	}
	name := sb.Spec.Name
	m.pending = name

	return func() tea.Msg {
		err := m.svc.Remove(context.Background(), api.ByName(name), false)
		if errors.Is(err, api.ErrRunning) {
			return actionMsg{verb: "not removed", name: name, err: errors.New("stop it first")}
		}
		return actionMsg{verb: "removed", name: name, err: err}
	}
}
