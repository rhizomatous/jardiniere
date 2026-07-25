package sandbox

import (
	"strings"
	"testing"
)

func TestNameExplicit(t *testing.T) {
	name, err := Name("feature-x")
	if err != nil || name != "feature-x" {
		t.Errorf("got (%q, %v), want (feature-x, nil)", name, err)
	}
}

func TestNameTrims(t *testing.T) {
	name, err := Name("  feature-x  ")
	if err != nil || name != "feature-x" {
		t.Errorf("got (%q, %v), want (feature-x, nil)", name, err)
	}
}

func TestNameGenerated(t *testing.T) {
	// a generated name must itself be a valid sandbox name.
	name, err := Name("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, serr := sanitizeName(name); serr != nil {
		t.Errorf("generated name %q is not valid: %v", name, serr)
	}
}

func TestNameInvalid(t *testing.T) {
	for _, bad := range []string{"-leading", ".dot", "has space", "slash/name", "..", "weird*char"} {
		if _, err := Name(bad); err == nil {
			t.Errorf("Name(%q) expected an error, got none", bad)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	// names must be valid hostnames: alphanumeric start, then letters/digits/'-'.
	good := []string{"feature-x", "fix123", "a", "Z9", "brisk-otter"}
	for _, n := range good {
		if _, err := sanitizeName(n); err != nil {
			t.Errorf("sanitizeName(%q) unexpected error: %v", n, err)
		}
	}
	bad := []string{"", "  ", "-leading", ".dot", "fix.123", "Z_9", "has space", "slash/name", "..", "weird*char"}
	for _, n := range bad {
		if _, err := sanitizeName(n); err == nil {
			t.Errorf("sanitizeName(%q) expected an error, got none", n)
		}
	}
}

func TestPickName(t *testing.T) {
	name := pickName(func(int) int { return 0 })
	if want := adjectives[0] + "-" + plants[0]; name != want {
		t.Errorf("got %q, want %q", name, want)
	}
	if parts := strings.Split(name, "-"); len(parts) != 2 {
		t.Errorf("name %q is not adjective-plant", name)
	}
}
