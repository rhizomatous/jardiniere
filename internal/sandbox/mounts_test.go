package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMount(t *testing.T) {
	const home = "/home/viv"
	const base = "/home/viv/proj"
	tests := []struct {
		spec             string
		wantSrc, wantDst string
		wantMode         string
		wantErr          bool
	}{
		{spec: "~/.aws", wantSrc: "/home/viv/.aws", wantDst: "/root/.aws", wantMode: "ro"},
		{spec: "~/.aws:rw", wantSrc: "/home/viv/.aws", wantDst: "/root/.aws", wantMode: "rw"},
		{spec: "/etc/hosts:/etc/hosts:ro", wantSrc: "/etc/hosts", wantDst: "/etc/hosts", wantMode: "ro"},
		{spec: "~/.cfg:~/.config/app", wantSrc: "/home/viv/.cfg", wantDst: "/root/.config/app", wantMode: "ro"},
		// relative source resolves against baseDir; e.g. a sibling repo.
		{spec: "../backend:/work/backend:rw", wantSrc: "/home/viv/backend", wantDst: "/work/backend", wantMode: "rw"},
		{spec: "sub:/work/sub", wantSrc: "/home/viv/proj/sub", wantDst: "/work/sub", wantMode: "ro"},
		{spec: "malformed/path", wantErr: true}, // must be either absolute or relative
		{spec: "~/a:relative", wantErr: true},   // target not absolute
		{spec: "a:b:c:d", wantErr: true},        // too many segments
		{spec: "", wantErr: true},               // empty
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			src, dst, mode, err := parseMount(tc.spec, home, base)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got (%q,%q,%q)", tc.spec, src, dst, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if src != tc.wantSrc || dst != tc.wantDst || mode != tc.wantMode {
				t.Errorf("got (%q,%q,%q), want (%q,%q,%q)", src, dst, mode, tc.wantSrc, tc.wantDst, tc.wantMode)
			}
		})
	}
}

func TestResolveMountsChecksExistence(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "present")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}

	// existing source resolves to a -v flag.
	args, err := resolveMounts([]string{real + ":ro"}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 || args[0] != "-v" || args[1] != real+":"+real+":ro" {
		t.Errorf("got %v", args)
	}

	// a relative source resolves against baseDir before the existence check.
	args, err = resolveMounts([]string{"present:/work/present:rw"}, "", dir)
	if err != nil {
		t.Fatalf("unexpected error for relative source: %v", err)
	}
	if len(args) != 2 || args[1] != real+":/work/present:rw" {
		t.Errorf("relative source: got %v", args)
	}

	// missing source is a hard error. don't let docker create it on the host.
	if _, err := resolveMounts([]string{filepath.Join(dir, "nope") + ":ro"}, "", ""); err == nil {
		t.Error("expected error for non-existent source, got nil")
	}
}
