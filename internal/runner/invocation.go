package runner

import (
	"strings"
)

// Invocation is one container-runtime command string. Every runner
// renders its invocations this way so --dry-run can show exactly what
// would execute.
type Invocation struct {
	Path string
	Args []string
}

// String renders the invocation as a line you can paste into a shell.
func (i Invocation) String() string {
	parts := make([]string, 0, len(i.Args)+1)
	parts = append(parts, quote(i.Path))
	for _, a := range i.Args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

// shellSafe are the characters a bare word may contain without quoting.
const shellSafe = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"0123456789" +
	"@%+=:,./-_"

// quote wraps s in single quotes when it holds anything a shell would treat
// specially. An empty string always quotes, so it stays a distinct argument.
func quote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool { return !strings.ContainsRune(shellSafe, r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
