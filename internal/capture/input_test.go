package capture

import (
	"testing"
	"time"

	"keyhud/internal/config"
)

// The tests in this file pin how a raw key press becomes one label: what it
// types, when it is a combination, and what it deliberately swallows.

// p is the pipeline under test. Every test builds a fresh one, so no state
// leaks from one case into the next.
var p *Pipeline

func resetComboState() {
	p = New(Options{
		GroupWindow: 500 * time.Millisecond,
		Lanes:       3,
	})
	p.readKeyText = fakeLayout
}

// fakeLayout answers like a plain US layout with dead quotes, so the key tests
// do not depend on which keyboard layout the machine has installed. The real
// translation is covered by TestLayoutKeyTextComposesDeadKeys.
func fakeLayout(vk, scan uint32) keyText {
	if p.rightAlt {
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
		if p.heldMods[vkShift] {
			r = rune(vk) // "A"
		}
		return keyText{text: string(r)}
	}
	if vk >= 0x30 && vk <= 0x39 {
		if p.heldMods[vkShift] {
			return keyText{text: string(")!@#$%^&*("[vk-0x30])}
		}
		return keyText{text: string(rune(vk))}
	}
	switch vk {
	case 0xBA: // ;
		return keyText{text: ";"}
	case 0xBB: // =
		if p.heldMods[vkShift] {
			return keyText{text: "+"}
		}
		return keyText{text: "="}
	case 0xDE: // ' starts a dead accent
		return keyText{dead: true}
	}
	return keyText{}
}

// newTestHooks gives every key test a fresh pipeline, so lane numbers,
// grouping state and repeat runs start from zero.
func newTestHooks(t *testing.T) {
	t.Helper()
	resetComboState()
}

func press(t *testing.T, vks ...uint32) {
	t.Helper()
	for _, vk := range vks {
		p.handleKey(vk, 0, true)
		p.handleKey(vk, 0, false)
	}
}

// Fast typing overlaps keystrokes: the next key goes down before the previous
// one comes up. A held letter must never be mistaken for a modifier, otherwise
// "hola" turns into "ho" + "l+a".
func TestHeldLetterIsNotACombination(t *testing.T) {
	newTestHooks(t)

	if ev, ok := p.handleKey(0x41, 0, true); !ok || ev.Kind != "new" || ev.Text != "a" {
		t.Fatalf("first key = %+v, %v", ev, ok)
	}

	// "b" arrives while "a" is still down.
	ev, ok := p.handleKey(0x42, 0, true)
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

	if _, ok := p.handleKey(vkLShift, 0, true); ok {
		t.Fatal("a modifier alone must not publish a label")
	}

	ev, ok := p.handleKey(0x32, 0, true) // shift+2 => @
	if !ok {
		t.Fatal("shift+2 produced no event")
	}
	if ev.Kind != "new" || ev.Text != "@" || ev.Group != config.GroupSymbols || ev.Lane != 0 {
		t.Fatalf("shift+2 = %+v, want a @ label", ev)
	}

	// It is text, so it joins the word like any other character.
	p.handleKey(vkLShift, 0, false)
	next, ok := p.handleKey(0x33, 0, true)
	if !ok || next.Kind != "append" || next.Text != "3" {
		t.Fatalf("key after @ = %+v, %v", next, ok)
	}
}

func TestShiftedLetterStaysAWord(t *testing.T) {
	newTestHooks(t)

	if _, ok := p.handleKey(vkLShift, 0, true); ok {
		t.Fatal("a modifier alone must not publish a label")
	}

	// Shift+H must read as H, not SHIFT+H: an uppercase word is still a word.
	ev, ok := p.handleKey(0x48, 0, true)
	if !ok {
		t.Fatal("shift+h produced no event")
	}
	if ev.Kind != "new" || ev.Text != "H" || ev.Group != config.GroupLetters {
		t.Fatalf("shift+h = %+v", ev)
	}

	next, ok := p.handleKey(0x4F, 0, true)
	if !ok || next.Kind != "append" || next.Text != "O" {
		t.Fatalf("shift+o = %+v, %v, want append O", next, ok)
	}
}

func TestShiftWithAnotherModifierIsStillACombination(t *testing.T) {
	newTestHooks(t)

	p.handleKey(vkLControl, 0, true)
	p.handleKey(vkLShift, 0, true)
	ev, ok := p.handleKey(0x41, 0, true) // ctrl+shift+a
	if !ok {
		t.Fatal("ctrl+shift+a produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+shift+a" || ev.Group != config.GroupCombo {
		t.Fatalf("ctrl+shift+a = %+v", ev)
	}
}

// AltGr is right alt, and it types characters.
func TestAltGrTypesTheCharacter(t *testing.T) {
	newTestHooks(t)

	p.handleKey(vkRAlt, 0, true)
	ev, ok := p.handleKey(0x45, 0, true) // AltGr+E types é
	if !ok {
		t.Fatal("AltGr+E produced no event")
	}
	if ev.Kind != "new" || ev.Text != "é" || ev.Group != config.GroupLetters {
		t.Fatalf("AltGr+E = %+v, want the letter it types", ev)
	}
}

// Left alt with ctrl is a command, not AltGr: it types nothing and stays a
// combination, exactly like the user asked.
func TestCtrlAltIsACombination(t *testing.T) {
	newTestHooks(t)

	p.handleKey(vkLControl, 0, true)
	p.handleKey(vkLAlt, 0, true)
	ev, ok := p.handleKey(0x2E, 0, true) // ctrl+alt+delete
	if !ok {
		t.Fatal("ctrl+alt+delete produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+alt+delete" || ev.Group != config.GroupCombo {
		t.Fatalf("ctrl+alt+delete = %+v", ev)
	}
}

func TestAltTabIsACombination(t *testing.T) {
	newTestHooks(t)

	p.handleKey(vkLAlt, 0, true)
	ev, ok := p.handleKey(0x09, 0, true) // alt+tab
	if !ok {
		t.Fatal("alt+tab produced no event")
	}
	if ev.Kind != "new" || ev.Text != "alt+tab" || ev.Group != config.GroupCombo {
		t.Fatalf("alt+tab = %+v", ev)
	}
}

// A dead key shows nothing on screen when it is pressed; the character appears
// with the key that composes it.
func TestDeadKeyPublishesNothing(t *testing.T) {
	newTestHooks(t)

	press(t, 0x41)
	if ev, ok := p.handleKey(0xDE, 0, true); ok {
		t.Fatalf("dead key = %+v, want no label until it composes", ev)
	}
}

func TestSpaceEndsTheWordWithoutTakingALane(t *testing.T) {
	newTestHooks(t)

	press(t, 0x48, 0x4F, 0x4C, 0x41) // hola on lane 0

	ev, ok := p.handleKey(0x20, 0, true) // space
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
		got, ok := p.handleKey(vk, 0, true)
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
		p.handleKey(vk, 0, false)
	}
}

func TestControlSpaceKeepsItsName(t *testing.T) {
	newTestHooks(t)

	p.handleKey(vkLControl, 0, true)
	ev, ok := p.handleKey(0x20, 0, true)
	if !ok {
		t.Fatal("ctrl+space produced no event")
	}
	if ev.Kind != "new" || ev.Text != "ctrl+space" || ev.Group != config.GroupCombo {
		t.Fatalf("ctrl+space = %+v", ev)
	}
}

func TestBackspaceIsItsOwnLabel(t *testing.T) {
	newTestHooks(t)

	press(t, 0x41, 0x42) // ab
	ev, ok := p.handleKey(0x08, 0, true)
	if !ok {
		t.Fatal("backspace produced no event")
	}
	if ev.Kind != "new" || ev.Text != "backspace" || ev.Group != config.GroupSymbols || ev.Lane != 1 {
		t.Fatalf("backspace = %+v", ev)
	}
}

// Pressing an action key again and again is one gesture: it shows once.
func TestRepeatedActionKeyShowsOnce(t *testing.T) {
	newTestHooks(t)

	first, ok := p.handleKey(0x08, 0, true) // backspace
	if !ok || first.Text != "backspace" {
		t.Fatalf("first backspace = %+v, %v", first, ok)
	}
	for i := 0; i < 4; i++ {
		p.handleKey(0x08, 0, false)
		if ev, ok := p.handleKey(0x08, 0, true); ok {
			t.Fatalf("backspace repeat %d = %+v, want nothing", i+2, ev)
		}
	}

	// Each different action key is its own gesture.
	p.handleKey(0x09, 0, true) // tab
	p.handleKey(0x09, 0, false)
	p.handleKey(0x25, 0, true) // left arrow
	p.handleKey(0x25, 0, false)

	// A typed character ends the run: backspace after it shows again.
	p.handleKey(0x41, 0, true)
	p.handleKey(0x41, 0, false)
	if ev, ok := p.handleKey(0x25, 0, true); !ok || ev.Text != "←" {
		t.Fatalf("action key after typing = %+v, %v, want it to show again", ev, ok)
	}
}

// A spin of the wheel fires one event per notch, dozens per second. It is one
// gesture per direction, exactly like an action key held down.
func TestRepeatedWheelDirectionShowsOnce(t *testing.T) {
	newTestHooks(t)

	first, ok := p.onButton("wheel up")
	if !ok || first.Kind != "new" || first.Text != "wheel up" || first.Group != config.GroupMouse {
		t.Fatalf("first wheel up = %+v, %v", first, ok)
	}
	for i := 0; i < 4; i++ {
		if ev, ok := p.onButton("wheel up"); ok {
			t.Fatalf("wheel up notch %d = %+v, want nothing", i+2, ev)
		}
	}

	// The other direction is a different gesture and shows on its own.
	if ev, ok := p.onButton("wheel down"); !ok || ev.Text != "wheel down" {
		t.Fatalf("wheel down after wheel up = %+v, %v", ev, ok)
	}
}

// The wheel run is a run of one gesture: anything in between ends it.
func TestWheelDirectionShowsAgainAfterAnotherInput(t *testing.T) {
	newTestHooks(t)

	p.onButton("wheel up")
	if ev, ok := p.onButton("click"); !ok || ev.Text != "click" {
		t.Fatalf("click after wheel up = %+v, %v", ev, ok)
	}
	if ev, ok := p.onButton("wheel up"); !ok || ev.Text != "wheel up" {
		t.Fatalf("wheel up after a click = %+v, %v, want it to show again", ev, ok)
	}

	p.onButton("wheel up")
	press(t, 0x41) // typing
	if ev, ok := p.onButton("wheel up"); !ok || ev.Text != "wheel up" {
		t.Fatalf("wheel up after typing = %+v, %v, want it to show again", ev, ok)
	}

	p.onButton("wheel up")
	press(t, 0x08) // backspace
	if ev, ok := p.onButton("wheel up"); !ok || ev.Text != "wheel up" {
		t.Fatalf("wheel up after an action key = %+v, %v, want it to show again", ev, ok)
	}
}

// The sideways wheel keeps its own rate limit: a tilt is deliberate, but a
// fling still must not flood the strip.
func TestSidewaysWheelKeepsRateLimit(t *testing.T) {
	newTestHooks(t)

	if ev, ok := p.onButton("wheel right"); !ok || ev.Text != "wheel right" {
		t.Fatalf("first wheel right = %+v, %v", ev, ok)
	}
	if ev, ok := p.onButton("wheel right"); ok {
		t.Fatalf("second wheel right = %+v, want nothing inside the rate limit", ev)
	}
}

func TestAutoRepeatPublishesOnce(t *testing.T) {
	newTestHooks(t)

	if _, ok := p.handleKey(0x41, 0, true); !ok {
		t.Fatal("first press produced no event")
	}
	if _, ok := p.handleKey(0x41, 0, true); ok {
		t.Fatal("auto-repeat must not publish a second label")
	}
}
