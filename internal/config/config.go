// Package config reads the per-repo jardiniere.toml that tells jard how to
// enter a sandbox for a given repository.
//
// schema:
//
//	startup = "claude"            # command run inside nix develop. default "bash"
//	image   = "nixos/nix:latest"  # base runner image override. default "nixos/nix:latest"
//	agent   = "claude-code"       # coding agent to add to the env. default "none"
//	mounts  = ["~/.foo:ro"]       # extra host mounts, source[:target][:ro|rw]
//
//	[network]
//	mode  = "full"                # "none" | "allowlist" | "full"
//	allow = ["github.com"]        # allowlist hosts
package config

import (
	"errors"
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

// coding agents for the agent key. AgentNone (the default) injects nothing.
const (
	AgentNone       = "none"        // no agent injected (default)
	AgentOpencode   = "opencode"    // sst opencode
	AgentClaudeCode = "claude-code" // Anthropic Claude Code
	AgentCodex      = "codex"       // OpenAI Codex CLI
)

// agentSpec describes how an agent is provided from nixpkgs.
type agentSpec struct {
	pkg    string // nixpkgs attribute
	unfree bool   // whether that package carries an unfree license
}

// agents maps each supported agent to its nixpkgs package. the unfree flags
// track nixpkgs licensing at time of writing.
var agents = map[string]agentSpec{
	AgentOpencode:   {pkg: "opencode"},                  // MIT
	AgentClaudeCode: {pkg: "claude-code", unfree: true}, // proprietary
	AgentCodex:      {pkg: "codex"},                     // Apache-2.0
}

// AgentPackage returns the nixpkgs attribute for agent, or "" when the agent is
// AgentNone or unrecognised.
func AgentPackage(agent string) string {
	return agents[agent].pkg
}

// AgentUnfree reports whether the selected agent's nixpkgs package is unfree.
func AgentUnfree(agent string) bool {
	return agents[agent].unfree
}

// Config is the parsed jardiniere.toml.
type Config struct {
	Startup string        `toml:"startup"` // command to run inside the dev env
	Image   string        `toml:"image"`   // base runner image
	Agent   string        `toml:"agent"`   // coding agent to add to the dev env
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
		Agent:   AgentNone,
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
		return Config{}, err
	}

	// decode over the defaults so omitted keys keep their default values.
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Config{}, fmt.Errorf("%s: unknown key %q", path, undecoded[0].String())
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks that the enum-like fields hold recognized values and that an
// allowlist policy actually names hosts. it runs after file decoding and again
// after any CLI overrides.
func (c Config) Validate() error {
	if err := validateNetwork(c.Network.Mode); err != nil {
		return err
	}
	if c.Network.Mode == NetworkAllowlist && len(c.Network.Allow) == 0 {
		return errors.New(`network "allowlist" requires a non-empty allow list of hosts`)
	}
	return validateAgent(c.Agent)
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

// validateAgent rejects unknown agent values up front.
func validateAgent(agent string) error {
	if agent == AgentNone {
		return nil
	}
	if _, ok := agents[agent]; ok {
		return nil
	}
	return fmt.Errorf("invalid agent %q: must be %q, %q, %q, or %q",
		agent, AgentNone, AgentOpencode, AgentClaudeCode, AgentCodex)
}
