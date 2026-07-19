// Package config reads the per-repo jardiniere.toml that tells jard how to
// enter a sandbox for a given repository.
//
// schema:
//
//	startup = "claude"            # command run inside nix develop. default "bash"
//	image   = "nixos/nix:latest"  # base runner image override. default "nixos/nix:latest"
//	mounts  = ["~/.foo:ro"]       # extra host mounts, source[:target][:ro|rw]
//
//	[network]
//	mode  = "full"                # "none" | "allowlist" | "full"
//	allow = ["github.com"]        # allowlist hosts
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FileName is the config file jard looks for at the root of a target repo.
const FileName = "jardiniere.toml"

// network policy modes for the [network] mode key.
const (
	NetworkFull      = "full"      // unrestricted network (default)
	NetworkNone      = "none"      // no network at all
	NetworkAllowlist = "allowlist" // only hosts listed in Allow
)

// Config is the parsed jardiniere.toml.
type Config struct {
	Startup string        `toml:"startup"` // command to run inside the dev env
	Image   string        `toml:"image"`   // base runner image
	Mounts  []string      `toml:"mounts"`  // extra host mounts, each "source[:target][:ro|rw]"
	Network NetworkConfig `toml:"network"` // egress policy
}

// NetworkConfig is the [network] table: the egress policy for the sandbox.
type NetworkConfig struct {
	Mode  string   `toml:"mode"`  // "full" | "none" | "allowlist"
	Allow []string `toml:"allow"` // allowlisted hosts (allowlist mode)
}

// Defaults returns the config used when a repo has no jardiniere.toml.
func Defaults() Config {
	return Config{
		Startup: "bash",
		Image:   "nixos/nix:latest",
		Network: NetworkConfig{Mode: NetworkFull},
	}
}

// Load reads jardiniere.toml from dir, falling back to Defaults for any omitted key.
func Load(dir string) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Defaults(), nil
		}
		return Defaults(), err
	}

	// decode over the defaults so omitted keys keep their default values.
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Defaults(), fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Defaults(), fmt.Errorf("%s: unknown key %q", path, undecoded[0].String())
	}

	if err := validateNetwork(cfg.Network.Mode); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Network.Mode == NetworkAllowlist && len(cfg.Network.Allow) == 0 {
		return Defaults(), fmt.Errorf(`%s: network = "allowlist" requires a non-empty `+"`allow`"+` list of hosts`, path)
	}
	return cfg, nil
}

// validateNetwork rejects unknown network values up front.
func validateNetwork(mode string) error {
	switch mode {
	case NetworkFull, NetworkNone, NetworkAllowlist:
		return nil
	default:
		return fmt.Errorf("invalid network %q: must be %q, %q, or %q",
			mode, NetworkNone, NetworkAllowlist, NetworkFull)
	}
}
