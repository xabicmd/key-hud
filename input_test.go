package main

import (
	"testing"
	"time"
)

// The tests in this file pin how a raw key press becomes one label: what it
// types, when it is a combination, and what it deliberately swallows.

func resetComboState() {
	pressed = map[uint32]bool{}
	heldMods = map[uint32]bool{}
	rightAlt = false
	clearActionRun()
}

// fakeLayout answers like a plain US layout with dead quotes, so the key tests
// do not depend on which keyboard layout the machine has installed. The real
// translation is covered by TestLayoutKeyTextComposesDeadKeys.
func fakeLayout(vk, scan uint32) keyText {
	if rightAlt {
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
	handleKey(vkLShift, 0, false)
	next, ok := handleKey(0x33, 0, true)
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

	handleKey(vkLControl, 0, true)
	handleKey(vkLShift, 0, true)
	ev, ok := handleKey(0x41, 0, true) // ctrl+shift+a
	if !ok {
		t.Fatal("ctrl+shift+a produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+shift+a" || ev.Group != groupCombo {
		t.Fatalf("ctrl+shift+a = %+v", ev)
	}
}

// AltGr is right alt, and it types characters.
func TestAltGrTypesTheCharacter(t *testing.T) {
	newTestHooks(t)

	handleKey(vkRAlt, 0, true)
	ev, ok := handleKey(0x45, 0, true) // AltGr+E types é
	if !ok {
		t.Fatal("AltGr+E produced no event")
	}
	if ev.Kind != "new" || ev.Text != "é" || ev.Group != groupLetters {
		t.Fatalf("AltGr+E = %+v, want the letter it types", ev)
	}
}

// Left alt with ctrl is a command, not AltGr: it types nothing and stays a
// combination, exactly like the user asked.
func TestCtrlAltIsACombination(t *testing.T) {
	newTestHooks(t)

	handleKey(vkLControl, 0, true)
	handleKey(vkLAlt, 0, true)
	ev, ok := handleKey(0x2E, 0, true) // ctrl+alt+delete
	if !ok {
		t.Fatal("ctrl+alt+delete produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+alt+delete" || ev.Group != groupCombo {
		t.Fatalf("ctrl+alt+delete = %+v", ev)
	}
}

func TestAltTabIsACombination(t *testing.T) {
	newTestHooks(t)

	handleKey(vkLAlt, 0, true)
	ev, ok := handleKey(0x09, 0, true) // alt+tab
	if !ok {
		t.Fatal("alt+tab produced no event")
	}
	if ev.Kind != "new" || ev.Text != "alt+tab" || ev.Group != groupCombo {
		t.Fatalf("alt+tab = %+v", ev)
	}
}

// A dead key shows nothing on screen when it is pressed; the character appears
// with the key that composes it.
func TestDeadKeyPublishesNothing(t *testing.T) {
	newTestHooks(t)

	press(t, 0x41)
	if ev, ok := handleKey(0xDE, 0, true); ok {
		t.Fatalf("dead key = %+v, want no label until it composes", ev)
	}
}

func TestSpaceEndsTheWordWithoutTakingALane(t *testing.T) {
	newTestHooks(t)

	press(t, 0x48, 0x4F, 0x4C, 0x41) // hola on lane 0

	ev, ok := handleKey(0x20, 0, true) // space
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
		got, ok := handleKey(vk, 0, true)
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
		handleKey(vk, 0, false)
	}
}

func TestControlSpaceKeepsItsName(t *testing.T) {
	newTestHooks(t)

	handleKey(vkLControl, 0, true)
	ev, ok := handleKey(0x20, 0, true)
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
	ev, ok := handleKey(0x08, 0, true)
	if !ok {
		t.Fatal("backspace produced no event")
	}
	if ev.Kind != "new" || ev.Text != "backspace" || ev.Group != groupSymbols || ev.Lane != 1 {
		t.Fatalf("backspace = %+v", ev)
	}
}

// Pressing an action key again and again is one gesture: it shows once.
func TestRepeatedActionKeyShowsOnce(t *testing.T) {
	newTestHooks(t)

	first, ok := handleKey(0x08, 0, true) // backspace
	if !ok || first.Text != "backspace" {
		t.Fatalf("first backspace = %+v, %v", first, ok)
	}
	for i := 0; i < 4; i++ {
		handleKey(0x08, 0, false)
		if ev, ok := handleKey(0x08, 0, true); ok {
			t.Fatalf("backspace repeat %d = %+v, want nothing", i+2, ev)
		}
	}

	// Each different action key is its own gesture.
	handleKey(0x09, 0, true) // tab
	handleKey(0x09, 0, false)
	handleKey(0x25, 0, true) // left arrow
	handleKey(0x25, 0, false)

	// A typed character ends the run: backspace after it shows again.
	handleKey(0x41, 0, true)
	handleKey(0x41, 0, false)
	if ev, ok := handleKey(0x25, 0, true); !ok || ev.Text != "left" {
		t.Fatalf("action key after typing = %+v, %v, want it to show again", ev, ok)
	}
}

func TestAutoRepeatPublishesOnce(t *testing.T) {
	newTestHooks(t)

	if _, ok := handleKey(0x41, 0, true); !ok {
		t.Fatal("first press produced no event")
	}
	if _, ok := handleKey(0x41, 0, true); ok {
		t.Fatal("auto-repeat must not publish a second label")
	}
}
