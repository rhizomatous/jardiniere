package api

import "testing"

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1024", 1024},
		{"512b", 512},
		{"1k", 1 << 10},
		{"1KiB", 1 << 10},
		{"2m", 2 << 20},
		{"2MiB", 2 << 20},
		{"8GiB", 8 << 30},
		{"8g", 8 << 30},
		{" 4 GiB ", 4 << 30},
		{"1.5GiB", 1536 << 20},
	}
	for _, tc := range cases {
		got, err := ParseBytes(tc.in)
		if err != nil {
			t.Errorf("ParseBytes(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseBytes(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseBytesRejectsGarbage(t *testing.T) {
	for _, in := range []string{"eight", "8 gigs", "-1GiB", "GiB", "1..5g"} {
		if _, err := ParseBytes(in); err == nil {
			t.Errorf("ParseBytes(%q) succeeded, want an error", in)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "unlimited"},
		{-1, "unlimited"},
		{512, "512B"},
		{2 << 10, "2KiB"},
		{2 << 20, "2MiB"},
		{8 << 30, "8GiB"},
	}
	for _, tc := range cases {
		if got := FormatBytes(tc.in); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBytesRoundTrip(t *testing.T) {
	// what FormatBytes prints must parse back to the same count, or a limit
	// read off a display no longer means what it says.
	for _, want := range []int64{512, 2 << 10, 2 << 20, 8 << 30} {
		got, err := ParseBytes(FormatBytes(want))
		if err != nil {
			t.Errorf("ParseBytes(FormatBytes(%d)): %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("round trip of %d = %d", want, got)
		}
	}
}
