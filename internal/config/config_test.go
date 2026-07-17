package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("missing file: got %+v, want defaults %+v", cfg, Defaults())
	}
}

func TestLoadOverridesAndKeepsDefaults(t *testing.T) {
	dir := writeConfig(t, `
# a comment
startup = "claude"   # trailing comment
allow   = ["github.com", "api.anthropic.com"]
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Startup != "claude" {
		t.Errorf("startup: got %q, want claude", cfg.Startup)
	}
	if cfg.Image != Defaults().Image {
		t.Errorf("image should fall back to default, got %q", cfg.Image)
	}
	if want := []string{"github.com", "api.anthropic.com"}; !reflect.DeepEqual(cfg.Allow, want) {
		t.Errorf("allow: got %v, want %v", cfg.Allow, want)
	}
}

func TestLoadUnknownKeyErrors(t *testing.T) {
	dir := writeConfig(t, "bogus = 1\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestParseLinePreservesHashInQuotes(t *testing.T) {
	key, val, ok := parseLine(`startup = "a#b"  # real comment`)
	if !ok || key != "startup" {
		t.Fatalf("parseLine failed: key=%q ok=%v", key, ok)
	}
	if got := str(val); got != "a#b" {
		t.Errorf("hash inside quotes should survive, got %q", got)
	}
}
