package sandbox

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// containerHome is where "~" resolves for mount targets inside the sandbox.
// The nixos/nix base image runs as root, so HOME is /root.
const containerHome = "/root"

// resolveMounts turns config mount specs into docker run -v arguments,
// validating each and confirming the host source exists.
func resolveMounts(specs []string, hostHome string) ([]string, error) {
	var args []string
	for _, spec := range specs {
		src, dst, mode, err := parseMount(spec, hostHome)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("mount %q: source %q is not accessible: %w", spec, src, err)
		}
		args = append(args, "-v", src+":"+dst+":"+mode)
	}
	return args, nil
}

// parseMount parses "source[:target][:mode]", where mode is ro|rw.
// ~ in source expands to the host home; ~ in target to the container home.
// target defaults to source (so "~/.aws" mounts the host's ~/.aws at the
// container's ~/.aws).
func parseMount(spec, hostHome string) (src, dst, mode string, err error) {
	parts := strings.Split(strings.TrimSpace(spec), ":")

	// a trailing ro/rw is the mode; otherwise default to read-only.
	mode = "ro"
	if n := len(parts); n > 1 {
		if last := parts[n-1]; last == "ro" || last == "rw" {
			mode = last
			parts = parts[:n-1]
		}
	}

	var rawSrc, rawDst string
	switch len(parts) {
	case 1:
		rawSrc, rawDst = parts[0], parts[0]
	case 2:
		rawSrc, rawDst = parts[0], parts[1]
	default:
		return "", "", "", fmt.Errorf("invalid mount %q: expected source[:target][:ro|rw]", spec)
	}
	if strings.TrimSpace(rawSrc) == "" || strings.TrimSpace(rawDst) == "" {
		return "", "", "", fmt.Errorf("invalid mount %q: empty source or target", spec)
	}

	src = expandHome(rawSrc, hostHome)
	dst = expandHome(rawDst, containerHome)
	if !filepath.IsAbs(src) {
		return "", "", "", fmt.Errorf("invalid mount %q: source must be an absolute or ~ path", spec)
	}
	if !path.IsAbs(dst) {
		return "", "", "", fmt.Errorf("invalid mount %q: target must be an absolute or ~ path", spec)
	}
	return src, dst, mode, nil
}

// expandHome replaces a leading ~ with home.
func expandHome(p, home string) string {
	if home == "" {
		return p
	}
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return home + p[1:]
	default:
		return p
	}
}
