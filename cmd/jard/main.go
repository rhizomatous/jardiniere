// jard manages persistent, isolated container sandboxes for coding agents.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/charmbracelet/fang"
)

// version is exactly what it says on the tin.
var version = "dev"

func main() {
	os.Exit(run())
}

// run returns an exit code rather than calling os.Exit, which would skip the
// deferred stop.
func run() int {
	// Ctrl-C should unwind cleanly rather than orphan a container.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := fang.Execute(ctx, newRootCmd(), fang.WithVersion(buildVersion())); err != nil {
		return 1
	}
	return 0
}

// buildVersion resolves the string shown by --version. It prefers a value
// injected via -ldflags, then falls back to VCS build info if absent.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	// set when installed via `go install ...@version`
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	// otherwise synthesize from the embedded git revision
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev := s.Value
			if len(rev) > 12 {
				rev = rev[:12]
			}
			return "dev-" + rev
		}
	}
	return version
}
