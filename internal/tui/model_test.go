package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// sandbox builds a listing entry.
func sandbox(name string, status api.Status) api.Sandbox {
	return api.Sandbox{
		Spec: api.Spec{
			Name:       name,
			Agent:      "claude",
			Image:      "base:1",
			Workspaces: []api.Workspace{{Host: "/home/viv/" + name}},
		},
		State: api.State{Status: status},
	}
}

// press sends a keypress and returns the updated model.
func press(t *testing.T, m *Model, key string) *Model {
	t.Helper()
	next, _ := m.Update(tea.KeyPressMsg{Code: keyCode(key), Text: key})
	got, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *Model", next)
	}
	return got
}

// keyCode maps the keys these tests use onto their rune, which is what
// KeyPressMsg.String reports for printable keys.
func keyCode(key string) rune {
	switch key {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "enter":
		return tea.KeyEnter
	default:
		return []rune(key)[0]
	}
}

// loaded returns a model already holding a listing.
func loaded(t *testing.T, svc api.Service, sandboxes ...api.Sandbox) *Model {
	t.Helper()
	m := New(svc)
	next, _ := m.Update(sandboxesMsg(sandboxes))
	return next.(*Model)
}

// view renders without styling, for assertions on content.
func view(m *Model) string { return ansi.Strip(m.render()) }

func TestEmptyStateSaysHowToMakeOne(t *testing.T) {
	m := loaded(t, api.NewFake())
	if !strings.Contains(view(m), "jard run") {
		t.Errorf("empty dashboard should say how to make a sandbox:\n%s", view(m))
	}
}

func TestListingRendersEachSandbox(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("alpha", api.StatusRunning), sandbox("beta", api.StatusStopped))
	out := view(m)
	for _, want := range []string{"alpha", "running", "beta", "stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}

func TestCursorMovesAndStopsAtTheEnds(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped), sandbox("b", api.StatusStopped))

	if m.cursor != 0 {
		t.Fatalf("cursor starts at %d, want 0", m.cursor)
	}
	m = press(t, m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want it held at the top", m.cursor)
	}
	m = press(t, m, "down")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m = press(t, m, "down")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want it held at the bottom", m.cursor)
	}
	// up from the bottom must actually move, or the earlier assertion only
	// proved that an unrecognised key does nothing.
	m = press(t, m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want up to have moved it back to 0", m.cursor)
	}
}

func TestVimKeysMatchTheArrows(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped), sandbox("b", api.StatusStopped))
	if m = press(t, m, "j"); m.cursor != 1 {
		t.Errorf("j: cursor = %d, want 1", m.cursor)
	}
	if m = press(t, m, "k"); m.cursor != 0 {
		t.Errorf("k: cursor = %d, want 0", m.cursor)
	}
}

func TestRunningSandboxShowsItsLoad(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusRunning))
	next, _ := m.Update(statsMsg{name: "a", sample: api.Stats{
		CPUPercent: 42, MemoryBytes: 2 << 30, MemoryLimit: 8 << 30,
	}})
	out := view(next.(*Model))

	for _, want := range []string{"42% cpu", "2GiB", "8GiB"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}

func TestStatsAreDroppedWhenASandboxStops(t *testing.T) {
	// otherwise a stopped sandbox keeps displaying the load it had when it
	// stopped, which reads as though it were still working.
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusRunning))
	next, _ := m.Update(statsMsg{name: "a", sample: api.Stats{CPUPercent: 42, MemoryBytes: 1 << 30, MemoryLimit: 8 << 30}})
	m = next.(*Model)
	if !strings.Contains(view(m), "42% cpu") {
		t.Fatal("expected the sample to render while running")
	}

	next, _ = m.Update(sandboxesMsg([]api.Sandbox{sandbox("a", api.StatusStopped)}))
	m = next.(*Model)
	if strings.Contains(view(m), "42% cpu") {
		t.Errorf("a stopped sandbox should not still show its old load:\n%s", view(m))
	}
}

func TestGaugeSaturatesAboveOneHundredPercent(t *testing.T) {
	// a multi-core sandbox reports well over 100%; the bar must not overflow
	// its column and wreck the layout.
	wide := gauge(450)
	if got := lipglossWidth(wide); got != gaugeWidth {
		t.Errorf("gauge(450) is %d cells wide, want %d", got, gaugeWidth)
	}
	if got := lipglossWidth(gauge(-5)); got != gaugeWidth {
		t.Errorf("gauge(-5) is %d cells wide, want %d", got, gaugeWidth)
	}
}

func TestCursorStaysOnTheSameSandboxAcrossRefreshes(t *testing.T) {
	// the list re-reads every couple of seconds; the cursor jumping because a
	// sandbox above it disappeared would make the dashboard unusable.
	m := loaded(t, api.NewFake(),
		sandbox("a", api.StatusStopped),
		sandbox("b", api.StatusStopped),
		sandbox("c", api.StatusStopped))
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.selectedName() != "c" {
		t.Fatalf("selected %q, want c", m.selectedName())
	}

	// "a" goes away; the cursor should follow "c", not the index.
	next, _ := m.Update(sandboxesMsg([]api.Sandbox{
		sandbox("b", api.StatusStopped),
		sandbox("c", api.StatusStopped),
	}))
	m = next.(*Model)
	if m.selectedName() != "c" {
		t.Errorf("selected %q after refresh, want c", m.selectedName())
	}
}

func TestCursorClampsWhenTheSelectedSandboxDisappears(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped), sandbox("b", api.StatusStopped))
	m = press(t, m, "down")

	next, _ := m.Update(sandboxesMsg([]api.Sandbox{sandbox("a", api.StatusStopped)}))
	m = next.(*Model)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped into range", m.cursor)
	}
	if _, ok := m.selected(); !ok {
		t.Error("a non-empty listing should always have a selection")
	}
}

func TestSelectionOnAnEmptyListingIsSafe(t *testing.T) {
	m := loaded(t, api.NewFake())
	if _, ok := m.selected(); ok {
		t.Error("an empty listing has nothing selected")
	}
	// none of these should panic with nothing to act on.
	for _, key := range []string{"up", "down", "enter", "x", "s", "r"} {
		m = press(t, m, key)
	}
}

func TestEnterAsksToAttachTheSandboxsOwnAgent(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusRunning), sandbox("b", api.StatusStopped))
	m = press(t, m, "down")
	m = press(t, m, "enter")

	got := m.Attach()
	if got == nil {
		t.Fatal("enter should have asked for a session")
	}
	if got.Sandbox != "b" {
		t.Errorf("sandbox = %q, want the selected one", got.Sandbox)
	}
	if len(got.Cmd) == 0 || got.Cmd[0] != "claude" {
		t.Errorf("cmd = %v, want the sandbox's own agent", got.Cmd)
	}
	if got.Workdir != "/home/viv/b" {
		t.Errorf("workdir = %q, want the primary workspace", got.Workdir)
	}
}

func TestEnterOnAStoppedSandboxStillAttaches(t *testing.T) {
	// starting it is the caller's job; refusing here would make Enter work on
	// some rows and not others for no reason the user can see.
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped))
	m = press(t, m, "enter")
	if m.Attach() == nil {
		t.Error("enter should attach a stopped sandbox too")
	}
}

func TestShellKeyAsksForAShell(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusRunning))
	m = press(t, m, "x")
	got := m.Attach()
	if got == nil || got.Cmd[0] != "bash" {
		t.Errorf("attach = %+v, want a shell", got)
	}
}

func TestToggleStartsAStoppedSandbox(t *testing.T) {
	fake := api.NewFake(sandbox("a", api.StatusStopped))
	m := loaded(t, fake, fake.Sandboxes...)

	m2, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = m2.(*Model)
	if m.pending != "a" {
		t.Errorf("pending = %q, want the row to show work in flight", m.pending)
	}
	if cmd == nil {
		t.Fatal("pressing s should have produced a command")
	}
	msg := cmd()
	if got, ok := msg.(actionMsg); !ok || got.err != nil || got.verb != "started" {
		t.Fatalf("action = %+v, want a successful start", msg)
	}
	if fake.Sandboxes[0].State.Status != api.StatusRunning {
		t.Errorf("status = %q, want running", fake.Sandboxes[0].State.Status)
	}
}

func TestToggleStopsARunningSandbox(t *testing.T) {
	fake := api.NewFake(sandbox("a", api.StatusRunning))
	m := loaded(t, fake, fake.Sandboxes...)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if got, ok := cmd().(actionMsg); !ok || got.verb != "stopped" {
		t.Fatalf("action = %+v, want a stop", got)
	}
	if fake.Sandboxes[0].State.Status != api.StatusStopped {
		t.Errorf("status = %q, want stopped", fake.Sandboxes[0].State.Status)
	}
}

func TestRemoveRefusesARunningSandboxRatherThanForcing(t *testing.T) {
	// the service's guard exists so one keystroke cannot destroy a live
	// session. Passing force from the dashboard would defeat it.
	fake := api.NewFake(sandbox("a", api.StatusRunning))
	m := loaded(t, fake, fake.Sandboxes...)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got, ok := cmd().(actionMsg)
	if !ok || got.err == nil {
		t.Fatalf("action = %+v, want a refusal", got)
	}
	if !strings.Contains(got.err.Error(), "stop it first") {
		t.Errorf("err = %v, want it to say what to do instead", got.err)
	}
	if len(fake.Sandboxes) != 1 {
		t.Error("the sandbox should still exist")
	}
}

func TestRemoveDeletesAStoppedSandbox(t *testing.T) {
	fake := api.NewFake(sandbox("a", api.StatusStopped))
	m := loaded(t, fake, fake.Sandboxes...)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if got, ok := cmd().(actionMsg); !ok || got.err != nil {
		t.Fatalf("action = %+v, want a successful removal", got)
	}
	if len(fake.Sandboxes) != 0 {
		t.Error("the sandbox should be gone")
	}
}

func TestKeysAreIgnoredWhileAnActionIsInFlight(t *testing.T) {
	// a second keypress must not race the first against the same sandbox.
	fake := api.NewFake(sandbox("a", api.StatusStopped), sandbox("b", api.StatusStopped))
	m := loaded(t, fake, fake.Sandboxes...)

	m2, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = m2.(*Model)

	before := m.cursor
	m = press(t, m, "down")
	if m.cursor != before {
		t.Error("the cursor should not move while an action is pending")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}); cmd != nil {
		t.Error("a second action should not start while one is pending")
	}
}

func TestQuitAndHelpWorkEvenWhilePending(t *testing.T) {
	fake := api.NewFake(sandbox("a", api.StatusStopped))
	m := loaded(t, fake, fake.Sandboxes...)
	m2, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = m2.(*Model)

	m = press(t, m, "?")
	if !m.showHelp {
		t.Error("help should toggle even with an action pending")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd == nil {
		t.Error("q should quit even with an action pending")
	}
}

func TestActionFailureIsShownRatherThanSwallowed(t *testing.T) {
	fake := api.NewFake(sandbox("a", api.StatusStopped))
	fake.Err = errors.New("daemon unreachable")
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped))
	m.svc = fake

	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	next, _ := m.Update(cmd())
	m = next.(*Model)

	if !strings.Contains(view(m), "daemon unreachable") {
		t.Errorf("a failed action should be visible:\n%s", view(m))
	}
	if m.pending != "" {
		t.Error("a failed action should clear the pending marker")
	}
}

func TestListingErrorIsShown(t *testing.T) {
	m := New(api.NewFake())
	next, _ := m.Update(errMsg{errors.New("no runtime")})
	if !strings.Contains(view(next.(*Model)), "no runtime") {
		t.Errorf("a listing failure should be visible:\n%s", view(next.(*Model)))
	}
}

func TestHelpListsEveryBinding(t *testing.T) {
	m := loaded(t, api.NewFake(), sandbox("a", api.StatusStopped))
	m = press(t, m, "?")
	out := view(m)
	for _, k := range Keys {
		if !strings.Contains(out, k.Help) {
			t.Errorf("help missing %q:\n%s", k.Help, out)
		}
	}
}

// lipglossWidth measures a styled string in terminal cells.
func lipglossWidth(s string) int { return lipgloss.Width(s) }
