package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rhizomatous/jardiniere/internal/config"
)

// writeSettings creates an agent's user settings file under a fake home.
func writeSettings(t *testing.T, home, rel string) string {
	t.Helper()
	p := filepath.Join(home, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSettingsMount(t *testing.T) {
	home := t.TempDir()
	src := writeSettings(t, home, config.AgentSettings(config.AgentClaudeCode))

	// read-only, and landing at the same path under the container home.
	want := src + ":" + containerHome + "/.claude/settings.json:ro"
	if got := settingsMount(config.AgentClaudeCode, home); !slices.Contains(got, want) {
		t.Errorf("got %v, want one of them %q", got, want)
	}
}

func TestSettingsMountSkips(t *testing.T) {
	const agent = config.AgentClaudeCode
	rel := config.AgentSettings(agent)

	tests := []struct {
		name  string
		agent string
		home  func(t *testing.T) string
	}{
		{
			// no agent configured
			name:  "no agent configured",
			agent: config.AgentNone,
			home: func(t *testing.T) string {
				home := t.TempDir()
				writeSettings(t, home, rel)
				return home
			},
		}, {
			// os.UserHomeDir failed, e.g. HOME unset under systemd or cron
			name:  "host home unknown",
			agent: agent,
			home:  func(*testing.T) string { return "" },
		}, {
			// settings file missing
			name:  "settings never written",
			agent: agent,
			home:  func(t *testing.T) string { return t.TempDir() },
		}, {
			// found a directory instead of a file, e.g. if you bind-mounted a file
			// in docker that doesn't exist
			name:  "directory at the settings path",
			agent: agent,
			home: func(t *testing.T) string {
				home := t.TempDir()
				if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(rel)), 0o755); err != nil {
					t.Fatal(err)
				}
				return home
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := settingsMount(tc.agent, tc.home(t)); got != nil {
				t.Errorf("expected no mount, got %v", got)
			}
		})
	}
}

// these files are commonly symlinked into a dotfiles repo.
func TestSettingsMountFollowsSymlink(t *testing.T) {
	home := t.TempDir()
	real := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(real, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, filepath.FromSlash(config.AgentSettings(config.AgentClaudeCode)))
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if settingsMount(config.AgentClaudeCode, home) == nil {
		t.Error("a symlinked settings file should still be mounted")
	}
}

func TestAgentSettingsPathsAreRelative(t *testing.T) {
	for _, agent := range []string{config.AgentOpencode, config.AgentClaudeCode, config.AgentCodex} {
		rel := config.AgentSettings(agent)
		if rel == "" {
			t.Errorf("%s: no settings path", agent)
		}
		if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "~") {
			t.Errorf("%s: settings path %q must be home-relative", agent, rel)
		}
	}
}
