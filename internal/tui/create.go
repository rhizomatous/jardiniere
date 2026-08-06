package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/rhizomatous/jardiniere/internal/api"
)

// createForm collects what a new sandbox needs. Only the three fields with no
// sensible default are asked for; resource limits and ports stay on the CLI,
// where a flag is quicker than a form field.
type createForm struct {
	form      *huh.Form
	workspace string
	agent     string
	name      string
}

// newCreateForm builds the form, defaulting the workspace to the directory jard
// was started in.
func newCreateForm(cwd string) *createForm {
	c := &createForm{workspace: cwd, agent: api.DefaultAgent}

	c.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("workspace").
				Description("the directory to mount, at this same path inside").
				Value(&c.workspace).
				Validate(validWorkspace),

			huh.NewSelect[string]().
				Title("agent").
				Options(huh.NewOptions(api.AgentNames()...)...).
				Value(&c.agent),

			huh.NewInput().
				Title("name").
				Description("leave blank to name it after the directory").
				Value(&c.name).
				Validate(validOptionalName),
		),
	)
	return c
}

// validWorkspace rejects a path that is not a directory jard can mount, so the
// form says so while the user is still in it.
func validWorkspace(path string) error {
	if path == "" {
		return errors.New("a workspace is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return errors.New("no such directory")
	}
	if !fi.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

// validOptionalName accepts an empty name, which means "derive one".
func validOptionalName(name string) error {
	if name == "" || api.ValidName(name) {
		return nil
	}
	return errors.New("letters, digits, dot, dash, or underscore, starting with a letter or digit")
}

// spec turns the answers into a Spec. Called only once the form completes, so
// the workspace has already been validated.
func (c *createForm) spec() (api.Spec, error) {
	abs, err := filepath.Abs(c.workspace)
	if err != nil {
		return api.Spec{}, err
	}
	def, err := api.LookupAgent(c.agent)
	if err != nil {
		return api.Spec{}, err
	}

	name := c.name
	if name == "" {
		name = api.SandboxName(abs)
	}
	return api.Spec{
		Name:       name,
		Agent:      c.agent,
		Image:      def.Image,
		Workspaces: []api.Workspace{{Host: abs}},
	}, nil
}

// startCreate opens the form.
func (m *Model) startCreate() tea.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		m.status = err.Error()
		return nil
	}
	m.create = newCreateForm(cwd)
	return m.create.form.Init()
}

// updateCreate feeds a message to the open form and acts on the result.
func (m *Model) updateCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.create.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.create.form = f
	}

	switch m.create.form.State {
	case huh.StateCompleted:
		spec, err := m.create.spec()
		m.create = nil
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.pending = spec.Name
		return m, m.submitCreate(spec)

	case huh.StateAborted:
		m.create = nil
		return m, nil
	}
	return m, cmd
}

// submitCreate builds the sandbox.
func (m *Model) submitCreate(spec api.Spec) tea.Cmd {
	return func() tea.Msg {
		_, err := m.svc.Create(context.Background(), spec)
		return actionMsg{verb: "created", name: spec.Name, err: err}
	}
}
