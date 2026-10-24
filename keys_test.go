package main

import (
	"testing"
	"time"
)

func resetComboState() {
	pressed = map[uint32]bool{}
	heldMods = map[uint32]bool{}
	rightAlt = false
}

// fakeLayout answers like a plain US layout with dead quotes, so the key tests
// do not depend on which keyboard layout the machine has installed. The real
// translation is covered by TestLayoutKeyTextComposesDeadKeys.
func fakeLayout(vk, scan uint32) keyText {
	if heldMods[vkControl] && heldMods[vkAlt] {
		if vk == 0x45 { // AltGr+E
			return keyText{text: "é"}
		}
		return keyText{}
	}
	if vk == vkSpace {
		return keyText{text: " "}
	}
	if vk >= 0x41 && vk <= 0x5A {
		r := rune(vk + 0x20) // "a"
		if heldMods[vkShift] {
			r = rune(vk) // "A"
		}
		return keyText{text: string(r)}
	}
	if vk >= 0x30 && vk <= 0x39 {
		if heldMods[vkShift] {
			return keyText{text: string(")!@#$%^&*("[vk-0x30])}
		}
		return keyText{text: string(rune(vk))}
	}
	switch vk {
	case 0xBA: // ;
		return keyText{text: ";"}
	case 0xBB: // =
		if heldMods[vkShift] {
			return keyText{text: "+"}
		}
		return keyText{text: "="}
	case 0xDE: // ' starts a dead accent
		return keyText{dead: true}
	}
	return keyText{}
}

// newTestHooks gives every key test a fresh grouper, so lane numbers and
// grouping state start from zero.
func newTestHooks(t *testing.T) {
	t.Helper()
	resetComboState()
	readKeyText = fakeLayout
	t.Cleanup(func() {
		readKeyText = layoutKeyText
		resetComboState()
	})
	grouperRef = newGrouper(500*time.Millisecond, 3)
	hookEvents = nil // emit drops into the default branch, no reader needed
}

func press(t *testing.T, vks ...uint32) {
	t.Helper()
	for _, vk := range vks {
		handleKey(vk, 0, true)
		handleKey(vk, 0, false)
	}
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

// Fast typing overlaps keystrokes: the next key goes down before the previous
// one comes up. A held letter must never be mistaken for a modifier, otherwise
// "hola" turns into "ho" + "l+a".
func TestHeldLetterIsNotACombination(t *testing.T) {
	newTestHooks(t)

	if ev, ok := handleKey(0x41, 0, true); !ok || ev.Kind != "new" || ev.Text != "a" {
		t.Fatalf("first key = %+v, %v", ev, ok)
	}

	// "b" arrives while "a" is still down.
	ev, ok := handleKey(0x42, 0, true)
	if !ok {
		t.Fatal("second key produced no event")
	}
	if ev.Kind != "append" || ev.Text != "b" {
		t.Fatalf("while a is held: %+v, want append b", ev)
	}
}

// The user asked for the character that really appears: shift+2 types "@", so
// the strip shows "@" and not "shift+2".
func TestShiftCharacterShowsTheTypedCharacter(t *testing.T) {
	newTestHooks(t)

	if _, ok := handleKey(vkLShift, 0, true); ok {
		t.Fatal("a modifier alone must not publish a label")
	}

	ev, ok := handleKey(0x32, 0, true) // shift+2 => @
	if !ok {
		t.Fatal("shift+2 produced no event")
	}
	if ev.Kind != "new" || ev.Text != "@" || ev.Group != groupSymbols || ev.Lane != 0 {
		t.Fatalf("shift+2 = %+v, want a @ label", ev)
	}

	// It is text, so it joins the word like any other character.
	next, ok := handleKey(0x23, 0, true)
	if !ok || next.Kind != "append" || next.Text != "3" {
		t.Fatalf("key after @ = %+v, %v", next, ok)
	}
}

func TestShiftedLetterStaysAWord(t *testing.T) {
	newTestHooks(t)

	if _, ok := handleKey(vkLShift, 0, true); ok {
		t.Fatal("a modifier alone must not publish a label")
	}

	// Shift+H must read as H, not SHIFT+H: an uppercase word is still a word.
	ev, ok := handleKey(0x48, 0, true)
	if !ok {
		t.Fatal("shift+h produced no event")
	}
	if ev.Kind != "new" || ev.Text != "H" || ev.Group != groupLetters {
		t.Fatalf("shift+h = %+v", ev)
	}

	next, ok := handleKey(0x4F, 0, true)
	if !ok || next.Kind != "append" || next.Text != "O" {
		t.Fatalf("shift+o = %+v, %v, want append O", next, ok)
	}
}

func TestShiftWithAnotherModifierIsStillACombination(t *testing.T) {
	newTestHooks(t)

	handleKey(vkLControl, true)
	handleKey(vkLShift, true)
	ev, ok := handleKey(0x41, true) // ctrl+shift+a
	if !ok {
		t.Fatal("ctrl+shift+a produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+shift+a" || ev.Group != groupCombo {
		t.Fatalf("ctrl+shift+a = %+v", ev)
	}
}

func TestSpaceEndsTheWordWithoutTakingALane(t *testing.T) {
	newTestHooks(t)

	press(t, 0x48, 0x4F, 0x4C, 0x41) // hola on lane 0

	ev, ok := handleKey(0x20, true) // space
	if !ok {
		t.Fatal("space produced no event")
	}
	// The blank belongs to HOLA; it is not a label and it does not consume the
	// next lane, so the words keep 1 -> 2 -> 3 rotation.
	if ev.Kind != "space" || ev.Text != spaceRune {
		t.Fatalf("space = %q (%+v), want a blank on the current label", ev.Text, ev)
	}

	// The next word takes the lane right after HOLA, not one after a space.
	for i, vk := range []uint32{0x58, 0x41, 0x42, 0x49} { // xabi
		got, ok := handleKey(vk, true)
		if !ok {
			t.Fatalf("letter %d produced no event", i)
		}
		if i == 0 {
			if got.Kind != "new" || got.Text != "x" || got.Lane != 1 {
				t.Fatalf("first letter of the new word = %+v, want new x on lane 1", got)
			}
		} else if got.Kind != "append" {
			t.Fatalf("letter %d = %+v, want append", i, got)
		}
		handleKey(vk, false)
	}
}

func TestControlSpaceKeepsItsName(t *testing.T) {
	newTestHooks(t)

	handleKey(vkLControl, true)
	ev, ok := handleKey(0x20, true)
	if !ok {
		t.Fatal("ctrl+space produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+space" || ev.Group != groupCombo {
		t.Fatalf("ctrl+space = %+v", ev)
	}
}

func TestBackspaceIsItsOwnLabel(t *testing.T) {
	newTestHooks(t)

	press(t, 0x41, 0x42) // ab
	ev, ok := handleKey(0x08, true)
	if !ok {
		t.Fatal("backspace produced no event")
	}
	if ev.Kind != "new" || ev.Text != "backspace" || ev.Group != groupSymbols || ev.Lane != 1 {
		t.Fatalf("backspace = %+v", ev)
	}
}

func TestAutoRepeatPublishesOnce(t *testing.T) {
	newTestHooks(t)

	if _, ok := handleKey(0x41, true); !ok {
		t.Fatal("first press produced no event")
	}
	if _, ok := handleKey(0x41, true); ok {
		t.Fatal("auto-repeat must not publish a second label")
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
