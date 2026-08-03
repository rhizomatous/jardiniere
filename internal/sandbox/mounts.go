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
// validating each and confirming the host source exists. relative sources are
// resolved against baseDir.
func resolveMounts(specs []string, hostHome, baseDir string) ([]string, error) {
	var args []string
	for _, spec := range specs {
		src, dst, mode, err := parseMount(spec, hostHome, baseDir)
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
// target defaults to source.
func parseMount(spec, hostHome, baseDir string) (src, dst, mode string, err error) {
	rest := strings.TrimSpace(spec)

	// peel an optional trailing ro/rw mode off the right. default to read-only.
	mode = "ro"
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		if tail := rest[i+1:]; tail == "ro" || tail == "rw" {
			mode, rest = tail, rest[:i]
		}
	}

	// the remainder is source[:target].
	rawSrc, rawDst, hasTarget := strings.Cut(rest, ":")
	if !hasTarget {
		rawDst = rawSrc
	}
	if strings.TrimSpace(rawSrc) == "" || strings.TrimSpace(rawDst) == "" {
		return "", "", "", fmt.Errorf("invalid mount %q: empty source or target", spec)
	}

	src = expandHome(rawSrc, hostHome)
	dst = expandHome(rawDst, containerHome)
	// a relative source is taken relative to the target repo dir.
	if !filepath.IsAbs(src) {
		src = filepath.Join(baseDir, src)
	}
	if !filepath.IsAbs(src) {
		return "", "", "", fmt.Errorf("invalid mount %q: source must be an absolute, ~, or relative path", spec)
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
