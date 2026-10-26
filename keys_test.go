package main

import "testing"

// The key vocabulary itself: how a virtual-key code is named and how modifier
// sides collapse into one prefix.

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
