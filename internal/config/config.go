// `config` reads the per-repo `jardiniere.toml` that tells jard how to
// enter a sandbox for a given repository.
//
// schema:
//
//	startup = "claude"          # command run inside `nix develop`. default "bash"
//	image   = "nixos/nix:latest"  # base runner image override. default "nixos/nix:latest"
//	network = "full"            # "none" | "allowlist" | "full" (v2; parsed now)
//	allow   = ["github.com"]    # allowlist hosts (v2; parsed now)
//	mounts  = ["~/.foo:ro"]     # extra host mounts (v2; parsed now)
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// the config file jard looks for at the root of a target repo.
const FileName = "jardiniere.toml"

// the parsed `jardiniere.toml`.
type Config struct {
	Startup string   // command to run inside the dev env
	Image   string   // base runner image
	Network string   // network policy (v2)
	Allow   []string // allowlisted hosts (v2)
	Mounts  []string // extra host mounts (v2)
}

// default config used when a repo has no `jardiniere.toml`.
func Defaults() Config {
	return Config{
		Startup: "bash",
		Image:   "nixos/nix:latest",
		Network: "full",
	}
}

// reads `jardiniere.toml` from dir, falling back to `Defaults()` for any omitted key.
func Load(dir string) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(dir, FileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		key, val, ok := parseLine(sc.Text())
		if !ok {
			continue
		}
		switch key {
		case "startup":
			cfg.Startup = str(val)
		case "image":
			cfg.Image = str(val)
		case "network":
			cfg.Network = str(val)
		case "allow":
			cfg.Allow = arr(val)
		case "mounts":
			cfg.Mounts = arr(val)
		default:
			return cfg, fmt.Errorf("%s:%d: unknown key %q", path, line, key)
		}
	}
	return cfg, sc.Err()
}

// `parseLine` splits a "key = value" line, skipping blanks and # comments.
// returns `ok=false` for lines that carry no assignment.
func parseLine(raw string) (key, val string, ok bool) {
	s := strings.TrimSpace(stripComment(raw))
	if s == "" {
		return "", "", false
	}
	eq := strings.IndexByte(s, '=')
	if eq < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:eq]), strings.TrimSpace(s[eq+1:]), true
}

// `stripComment` removes a trailing # comment while respecting quoted strings so
// a "#" inside a value is preserved.
func stripComment(s string) string {
	inQuote := false
	for i, r := range s {
		switch r {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote {
				return s[:i]
			}
		}
	}
	return s
}

// `str` unquotes a scalar string value.
func str(v string) string {
	return strings.Trim(strings.TrimSpace(v), `"`)
}

// `arr` parses a `["a", "b"]` inline array into a slice of unquoted strings.
func arr(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, part := range strings.Split(v, ",") {
		if s := str(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
