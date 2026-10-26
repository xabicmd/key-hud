package main

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		w, h int
		ok   bool
	}{
		{"1920x480", 1920, 480, true},
		{" 1920 x 480 ", 1920, 480, true},
		{"1920", 0, 0, false},
		{"1920x", 0, 0, false},
		{"axb", 0, 0, false},
		{"0x480", 0, 0, false},
	}

	for _, c := range cases {
		w, h, ok := parseSize(c.in)
		if ok != c.ok || w != c.w || h != c.h {
			t.Errorf("parseSize(%q) = (%d,%d,%v), want (%d,%d,%v)", c.in, w, h, ok, c.w, c.h, c.ok)
		}
	}
}
