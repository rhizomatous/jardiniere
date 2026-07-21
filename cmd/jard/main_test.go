package main

import (
	"reflect"
	"testing"

	"github.com/rhizomatous/jardiniere/internal/config"
)

func TestOverrideEmptyFlagsKeepConfig(t *testing.T) {
	cfg := config.Config{
		Startup: "claude",
		Image:   "custom:latest",
		Agent:   config.AgentOpencode,
		Mounts:  []string{"~/.aws:ro"},
		Network: config.NetworkConfig{Mode: config.NetworkAllowlist, Allow: []string{"github.com"}},
	}
	if got := override(cfg, cli{}); !reflect.DeepEqual(got, cfg) {
		t.Errorf("empty flags should leave config untouched, got %+v", got)
	}
}

func TestOverrideScalarsReplace(t *testing.T) {
	agent, network := config.AgentCodex, config.NetworkNone
	cfg := config.Defaults()
	got := override(cfg, cli{
		Startup: "zsh",
		Image:   "img:2",
		Agent:   &agent,
		Network: &network,
	})
	if got.Startup != "zsh" || got.Image != "img:2" || got.Agent != config.AgentCodex || got.Network.Mode != config.NetworkNone {
		t.Errorf("scalar flags should replace config values, got %+v", got)
	}
}

func TestOverrideListsReplaceNotAppend(t *testing.T) {
	cfg := config.Config{
		Mounts:  []string{"~/.aws:ro"},
		Network: config.NetworkConfig{Mode: config.NetworkAllowlist, Allow: []string{"github.com"}},
	}
	got := override(cfg, cli{
		Mount: []string{"~/.cache:rw"},
		Allow: []string{"example.com"},
	})
	if want := []string{"~/.cache:rw"}; !reflect.DeepEqual(got.Mounts, want) {
		t.Errorf("mounts: got %v, want %v", got.Mounts, want)
	}
	if want := []string{"example.com"}; !reflect.DeepEqual(got.Network.Allow, want) {
		t.Errorf("allow: got %v, want %v", got.Network.Allow, want)
	}
}
