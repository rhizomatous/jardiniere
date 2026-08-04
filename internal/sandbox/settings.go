package sandbox

import (
	"os"
	"path"
	"path/filepath"

	"github.com/rhizomatous/jardiniere/internal/config"
)

// settingsMount is the read-only bind mount carrying the configured agent's
// user settings in from the host, so preferences like editor theme don't need
// setting up each run. it returns nil when the agent has no settings file, or
// the host has yet to write one.
func settingsMount(agent, hostHome string) []string {
	rel := config.AgentSettings(agent)
	if rel == "" || hostHome == "" {
		return nil
	}
	// Stat follows symlinks, so a path managed by a dotfile manager still works.
	src := filepath.Join(hostHome, filepath.FromSlash(rel))
	if fi, err := os.Stat(src); err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	return []string{"-v", src + ":" + path.Join(containerHome, rel) + ":ro"}
}
