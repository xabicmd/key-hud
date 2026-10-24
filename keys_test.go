package main

import "testing"

func resetComboState() {
	pressed = map[uint32]bool{}
	heldMods = map[uint32]bool{}
	heldOrder = nil
}

func TestKeyLabel(t *testing.T) {
	cases := []struct {
		vk   uint32
		want string
	}{
		{0x41, "a"},        // A
		{0x5A, "z"},        // Z
		{0x30, "0"},        // 0
		{0x39, "9"},        // 9
		{0x70, "f1"},       // F1
		{0x87, "f24"},      // F24
		{0x1B, "esc"},      // Escape
		{0x20, "space"},    // Space
		{0x0D, "enter"},    // Return
		{0x25, "left"},     // Left arrow
		{0x60, "numpad 0"}, // Numpad 0
		{0xBA, ";"},        // OEM semicolon
		{0x0F, "vk0f"},     // unmapped code
	}

	for _, c := range cases {
		if got := keyLabel(c.vk); got != c.want {
			t.Errorf("keyLabel(0x%02X) = %q, want %q", c.vk, got, c.want)
		}
	}
}

func TestKeyLabelIsLowercaseForLetters(t *testing.T) {
	// "ctrl+c" reads far better than "CTRL+C" next to the pixel font, and the
	// font renders everything uppercase anyway.
	if got := keyLabel(0x43); got != "c" {
		t.Fatalf("keyLabel(C) = %q, want %q", got, "c")
	}
}

func TestCanonModCollapsesSides(t *testing.T) {
	pairs := [][2]uint32{
		{vkLControl, vkControl},
		{vkRControl, vkControl},
		{vkLShift, vkShift},
		{vkRShift, vkShift},
		{vkLAlt, vkAlt},
		{vkRAlt, vkAlt},
		{vkLWin, vkWin},
		{vkRWin, vkWin},
	}
	for _, p := range pairs {
		if got := canonMod(p[0]); got != p[1] {
			t.Errorf("canonMod(0x%02X) = 0x%02X, want 0x%02X", p[0], got, p[1])
		}
	}
}

func TestBuildComboLabelOrder(t *testing.T) {
	resetComboState()
	if got := buildComboLabel("c"); got != "c" {
		t.Fatalf("no modifiers: got %q, want %q", got, "c")
	}

	// Order is fixed as ctrl, shift, alt, win regardless of press order.
	heldMods[canonMod(vkLWin)] = true
	heldMods[canonMod(vkRShift)] = true
	heldMods[canonMod(vkLAlt)] = true
	heldMods[canonMod(vkRControl)] = true

	if got, want := buildComboLabel("c"), "ctrl+shift+alt+win+c"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildComboLabelIncludesOtherHeldKeys(t *testing.T) {
	resetComboState()
	heldMods[canonMod(vkLControl)] = true
	heldOrder = []uint32{0x41} // "a" still held down

	if got, want := buildComboLabel("b"), "ctrl+a+b"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildComboLabelMouse(t *testing.T) {
	resetComboState()
	if got, want := buildComboLabel("click"), "click"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	heldMods[canonMod(vkLControl)] = true
	if got, want := buildComboLabel("click"), "ctrl+click"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRemoveHeld(t *testing.T) {
	resetComboState()
	heldOrder = []uint32{0x41, 0x42, 0x43}

	removeHeld(0x42)
	if len(heldOrder) != 2 || heldOrder[0] != 0x41 || heldOrder[1] != 0x43 {
		t.Fatalf("after removing 0x42: %v", heldOrder)
	}

	// Removing something absent must not panic or corrupt the slice.
	removeHeld(0x5A)
	if len(heldOrder) != 2 {
		t.Fatalf("absent removal changed slice: %v", heldOrder)
	}
}

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
