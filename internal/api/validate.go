package api

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// nameRE constrains sandbox names to what is safe as both a filename and a
// container name.
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)

// ValidName reports whether name is usable as a sandbox name.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// Validate checks a spec before anything acts on it. Every field here reaches a
// container runtime as a command-line argument, where a malformed value is
// silently misread rather than rejected — a workspace path containing a colon
// is the worst of them, since it mounts a different directory than the one
// asked for.
func (s Spec) Validate() error {
	if !ValidName(s.Name) {
		return fmt.Errorf("invalid sandbox name %q: use letters, digits, dot, dash, or underscore, starting with a letter or digit", s.Name)
	}
	if strings.TrimSpace(s.Image) == "" {
		return fmt.Errorf("sandbox %q has no image", s.Name)
	}
	if s.Agent != "" {
		if _, err := LookupAgent(s.Agent); err != nil {
			return err
		}
	}
	if err := validateWorkspaces(s.Workspaces); err != nil {
		return err
	}
	if err := validateEnv(s.Env); err != nil {
		return err
	}
	if err := validatePorts(s.Ports); err != nil {
		return err
	}
	return validateResources(s.Resources)
}

func validateWorkspaces(workspaces []Workspace) error {
	seen := make(map[string]bool, len(workspaces))
	for _, ws := range workspaces {
		switch {
		case ws.Host == "":
			return fmt.Errorf("workspace has no path")
		// a mount spec is colon-delimited, so a colon in the path silently
		// shifts where the bind lands.
		case strings.Contains(ws.Host, ":"):
			return fmt.Errorf("workspace %q contains a colon, which a mount spec cannot represent", ws.Host)
		case !path.IsAbs(ws.Host):
			return fmt.Errorf("workspace %q is not an absolute path", ws.Host)
		case seen[ws.Host]:
			return fmt.Errorf("workspace %q is listed twice", ws.Host)
		}
		seen[ws.Host] = true
	}
	return nil
}

func validateEnv(env map[string]string) error {
	for k := range env {
		switch {
		case k == "":
			return fmt.Errorf("environment variable has no name")
		// "K=V=x" would bind K to "V=x" rather than erroring.
		case strings.ContainsAny(k, "=\x00"):
			return fmt.Errorf("environment variable name %q contains an equals sign or null byte", k)
		}
	}
	return nil
}

func validatePorts(ports []Port) error {
	for _, p := range ports {
		if !validPort(p.Host) || !validPort(p.Sandbox) {
			return fmt.Errorf("port %d:%d is out of range 1-65535", p.Host, p.Sandbox)
		}
		switch p.Proto {
		case "", "tcp", "udp":
		default:
			return fmt.Errorf("port protocol %q must be tcp or udp", p.Proto)
		}
	}
	return nil
}

func validPort(n int) bool { return n >= 1 && n <= 65535 }

func validateResources(r Resources) error {
	if r.CPUs < 0 {
		return fmt.Errorf("cpus %v is negative", r.CPUs)
	}
	if r.Memory < 0 {
		return fmt.Errorf("memory %d is negative", r.Memory)
	}
	return nil
}
