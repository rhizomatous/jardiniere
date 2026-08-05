package api

import (
	"fmt"
	"strconv"
	"strings"
)

// byteUnits maps a memory suffix to its multiplier. Both the IEC spelling and
// docker's single-letter shorthand are accepted, and both mean powers of 1024,
// which is what docker's -m does too.
var byteUnits = []struct {
	suffix string
	mult   int64
}{
	{"gib", 1 << 30},
	{"mib", 1 << 20},
	{"kib", 1 << 10},
	{"gb", 1 << 30},
	{"mb", 1 << 20},
	{"kb", 1 << 10},
	{"g", 1 << 30},
	{"m", 1 << 20},
	{"k", 1 << 10},
	{"b", 1},
}

// ParseBytes reads a memory size like "8GiB", "512m", or a bare byte count into
// the form [Resources] stores. An empty string is zero, meaning unlimited.
func ParseBytes(s string) (int64, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, nil
	}
	lower := strings.ToLower(raw)

	num, mult := lower, int64(1)
	for _, u := range byteUnits {
		if strings.HasSuffix(lower, u.suffix) {
			num, mult = strings.TrimSpace(strings.TrimSuffix(lower, u.suffix)), u.mult
			break
		}
	}

	val, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory size %q: want a number with an optional unit, e.g. \"8GiB\"", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("invalid memory size %q: must not be negative", s)
	}
	return int64(val * float64(mult)), nil
}

// FormatBytes renders a byte count in the largest unit that divides it cleanly,
// for display. Zero renders as "unlimited", matching what [Resources] means by it.
func FormatBytes(n int64) string {
	if n <= 0 {
		return "unlimited"
	}
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if n >= u.mult {
			return strconv.FormatFloat(float64(n)/float64(u.mult), 'g', 4, 64) + u.suffix
		}
	}
	return strconv.FormatInt(n, 10) + "B"
}
