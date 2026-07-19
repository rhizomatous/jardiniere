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

[network]
mode  = "allowlist"
allow = ["github.com", "api.anthropic.com"]
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
	if want := []string{"github.com", "api.anthropic.com"}; !reflect.DeepEqual(cfg.Network.Allow, want) {
		t.Errorf("allow: got %v, want %v", cfg.Network.Allow, want)
	}
}

func TestLoadUnknownKeyErrors(t *testing.T) {
	dir := writeConfig(t, "bogus = 1\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestLoadPreservesHashInQuotes(t *testing.T) {
	dir := writeConfig(t, `startup = "a#b"  # real comment`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Startup != "a#b" {
		t.Errorf("hash inside quotes should survive, got %q", cfg.Startup)
	}
}

// multi-line arrays are valid TOML and must parse.
func TestLoadMultiLineArray(t *testing.T) {
	dir := writeConfig(t, `
[network]
mode  = "allowlist"
allow = [
  "github.com",
  "api.anthropic.com",
]
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"github.com", "api.anthropic.com"}; !reflect.DeepEqual(cfg.Network.Allow, want) {
		t.Errorf("allow: got %v, want %v", cfg.Network.Allow, want)
	}
}

func TestLoadInvalidTOMLErrors(t *testing.T) {
	dir := writeConfig(t, "startup = \"unterminated\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for malformed toml, got nil")
	}
}

func TestLoadInvalidNetworkErrors(t *testing.T) {
	dir := writeConfig(t, "[network]\nmode = \"sometimes\"\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid network, got nil")
	}
}

func TestLoadAllowlistWithoutHostsErrors(t *testing.T) {
	dir := writeConfig(t, "[network]\nmode = \"allowlist\"\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for allowlist with empty allow, got nil")
	}
}
