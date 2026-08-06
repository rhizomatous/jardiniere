package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhizomatous/jardiniere/internal/api"
)

func tempDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	return dir
}

func TestCreateKeyOpensTheForm(t *testing.T) {
	m := loaded(t, api.NewFake())
	if m.create != nil {
		t.Fatal("the form should start closed")
	}
	m = press(t, m, "c")
	if m.create == nil {
		t.Fatal("c should open the form")
	}
	if !strings.Contains(view(m), "workspace") {
		t.Errorf("the open form should be visible:\n%s", view(m))
	}
}

func TestFormOwnsTheKeyboardWhileOpen(t *testing.T) {
	// typing "r" into the workspace field must not remove a sandbox, and "j"
	// must not move the cursor out from under the user.
	fake := api.NewFake(sandbox("a", api.StatusStopped), sandbox("b", api.StatusStopped))
	m := loaded(t, fake, fake.Sandboxes...)
	m = press(t, m, "c")

	for _, key := range []string{"r", "j", "x", "s"} {
		m = press(t, m, key)
	}
	if len(fake.Sandboxes) != 2 {
		t.Error("keys typed into the form acted on the list")
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want it left alone while the form is open", m.cursor)
	}
	if m.Attach() != nil {
		t.Error("typing x into the form should not have asked to attach")
	}
}

func TestFormSpecUsesTheAnswers(t *testing.T) {
	dir := tempDir(t, "myrepo")
	c := newCreateForm(dir)
	c.agent = "codex"

	spec, err := c.spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Name != "myrepo" {
		t.Errorf("name = %q, want it derived from the directory", spec.Name)
	}
	if spec.Agent != "codex" {
		t.Errorf("agent = %q, want codex", spec.Agent)
	}
	if len(spec.Workspaces) != 1 || spec.Workspaces[0].Host != dir {
		t.Errorf("workspaces = %+v, want the given directory", spec.Workspaces)
	}
	if spec.Image == "" {
		t.Error("image should come from the chosen agent")
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("the form produced a spec the service would reject: %v", err)
	}
}

func TestFormNameOverridesTheDerivedOne(t *testing.T) {
	c := newCreateForm(tempDir(t, "myrepo"))
	c.name = "custom"
	spec, err := c.spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Name != "custom" {
		t.Errorf("name = %q, want the one typed in", spec.Name)
	}
}

func TestFormResolvesARelativeWorkspace(t *testing.T) {
	// the sandbox binds the workspace at its host path, which has to be absolute.
	c := newCreateForm(".")
	spec, err := c.spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if !filepath.IsAbs(spec.Workspaces[0].Host) {
		t.Errorf("workspace = %q, want an absolute path", spec.Workspaces[0].Host)
	}
}

func TestWorkspaceValidation(t *testing.T) {
	dir := tempDir(t, "myrepo")
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatalf("writing a file: %v", err)
	}

	if err := validWorkspace(dir); err != nil {
		t.Errorf("a real directory should validate: %v", err)
	}
	for _, bad := range []string{"", "/definitely/not/here", file} {
		if err := validWorkspace(bad); err == nil {
			t.Errorf("validWorkspace(%q) should have failed", bad)
		}
	}
}

func TestOptionalNameValidation(t *testing.T) {
	// blank is legal and means "derive one"; anything else must be usable.
	if err := validOptionalName(""); err != nil {
		t.Errorf("a blank name is allowed: %v", err)
	}
	if err := validOptionalName("my-repo"); err != nil {
		t.Errorf("a valid name should pass: %v", err)
	}
	for _, bad := range []string{"has space", "-leading", "has/slash"} {
		if err := validOptionalName(bad); err == nil {
			t.Errorf("validOptionalName(%q) should have failed", bad)
		}
	}
}

func TestSubmitCreateGoesThroughTheService(t *testing.T) {
	dir := tempDir(t, "myrepo")
	fake := api.NewFake()
	m := New(fake)

	c := newCreateForm(dir)
	spec, err := c.spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	msg := m.submitCreate(spec)()

	got, ok := msg.(actionMsg)
	if !ok || got.err != nil {
		t.Fatalf("action = %+v, want a successful create", msg)
	}
	if len(fake.Sandboxes) != 1 || fake.Sandboxes[0].Spec.Name != "myrepo" {
		t.Errorf("sandboxes = %+v, want the new one", fake.Sandboxes)
	}
}

func TestSubmitCreateReportsAFailure(t *testing.T) {
	dir := tempDir(t, "myrepo")
	fake := api.NewFake(api.Sandbox{Spec: api.Spec{Name: "myrepo"}})
	m := New(fake)

	spec, err := newCreateForm(dir).spec()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	got, ok := m.submitCreate(spec)().(actionMsg)
	if !ok || got.err == nil {
		t.Fatalf("action = %+v, want a duplicate-name failure", got)
	}
}

func TestCreateKeyIsListedInHelp(t *testing.T) {
	m := loaded(t, api.NewFake())
	m = press(t, m, "?")
	if !strings.Contains(view(m), "create") {
		t.Errorf("help should list the create key:\n%s", view(m))
	}
}
