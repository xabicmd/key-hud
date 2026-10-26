package main

import (
	"strings"
	"time"
)

// This file turns a decoded Windows message into one instruction for the UI:
// what the key types, whether it is a combination, or whether it is a repeated
// gesture that must stay silent.

var (
	// ignoreInjected drops synthetic input (macros, remote sessions). Off by
	// default: a key you did not physically press is still a key press.
	ignoreInjected bool

	// All of the state below is touched only by the hook thread, which is
	// locked to its OS thread, so no synchronization is needed.
	pressed      = map[uint32]bool{}
	heldMods     = map[uint32]bool{}
	rightAlt     bool // AltGr is right alt plus the ctrl Windows adds for it
	lastAction   uint32
	lastActionAt time.Time
	lastWheel    time.Time

	// The wheel has no virtual-key code, so its repeat run is keyed by the
	// direction that was shown last: spinning the wheel up ten notches is one
	// gesture and shows one label, exactly like holding an action key.
	lastWheelRun   string
	lastWheelRunAt time.Time
)

func onKey(lParam uintptr, down bool) {
	kb := (*kbdLLHookStruct)(ptrAt(lParam))
	if ignoreInjected && kb.Flags&llkhfInjected != 0 {
		return
	}
	scan := kb.ScanCode
	if scan == 0 {
		scan = scanCode(kb.VkCode)
	}
	handleKey(kb.VkCode, scan, down)
}

// handleKey applies one key transition and returns the instruction to publish,
// if any. Auto-repeats, modifier bookkeeping and key releases produce none.
//
// What a key types is asked to the active layout, so the strip shows the
// character that really appears: "@" instead of "shift+2", "á" instead of a
// dead accent followed by "a". Only presses that type nothing (ctrl+c,
// alt+tab, backspace, f5) are shown as keys.
//
// Only real modifiers become a combination prefix. A letter that is still held
// down because the next key was pressed a few milliseconds later is just fast
// typing, not "a+b"; treating it as a modifier is what turned "hola" into
// "ho" + "l+a".
func handleKey(vk, scan uint32, down bool) (Event, bool) {
	if !down {
		delete(pressed, vk)
		if isModifier(vk) {
			delete(heldMods, canonMod(vk))
			if vk == vkRAlt {
				rightAlt = false
			}
		}
		return Event{}, false
	}

	// Auto-repeat: Windows keeps sending keydowns while a key is held. Show the
	// press once, not sixty times a second.
	if pressed[vk] {
		return Event{}, false
	}
	pressed[vk] = true

	if isModifier(vk) {
		heldMods[canonMod(vk)] = true
		if vk == vkRAlt {
			rightAlt = true
		}
		return Event{}, false
	}

	if typingModifiers() {
		switch typed := readKeyText(vk, scan); {
		case typed.dead:
			// The accent waits for the next key. Showing a label now would
			// claim a character that is not on screen yet.
			return Event{}, false
		case typed.text != "":
			clearActionRun()
			if typed.text == spaceRune {
				// A bare space ends the word and leaves the blank behind it,
				// but it never takes a lane: an invisible label would make the
				// visible words look like they skip a line.
				return emit(kindBreak, spaceRune), true
			}
			return emit(kindKey, typed.text), true
		}
	}

	if len(heldMods) > 0 {
		clearActionRun()
		return emit(kindCombo, buildCombo(vk)), true
	}

	// Backspace, tab, an arrow, page up/down, home/end: pressed again and
	// again, it is one gesture and it shows once.
	if repeatsAction(vk) {
		return Event{}, false
	}
	// An action key ends the wheel run: a scroll after backspace is not the
	// same gesture as the scroll before it.
	lastWheelRun, lastWheelRunAt = "", time.Time{}
	return emit(kindAction, keyLabel(vk)), true
}

// repeatsAction reports whether this action key continues the run of the same
// key that is already on screen. Holding a key repeats it by itself, and
// deleting ten characters is ten presses of one key: showing ten BACKSPACE
// labels describes the implementation, not what the user did.
func repeatsAction(vk uint32) bool {
	now := time.Now()
	same := vk == lastAction && now.Sub(lastActionAt) <= grouperRef.window
	lastAction, lastActionAt = vk, now
	return same
}

// clearActionRun ends the run, so the same action key after a typed character
// or a combination shows again. It also ends the wheel run: a scroll is only
// one gesture while nothing else happens in between.
func clearActionRun() {
	lastAction, lastActionAt = 0, time.Time{}
	lastWheelRun, lastWheelRunAt = "", time.Time{}
}

// onButton publishes one mouse button or wheel movement. The vertical wheel
// repeats like an action key: scrolling up several notches is one gesture and
// shows one label, because a fast wheel would otherwise push everything else
// off the strip. The sideways wheel and the buttons keep their own handling.
func onButton(name string) (Event, bool) {
	now := time.Now()
	if name == "wheel up" || name == "wheel down" {
		same := name == lastWheelRun && now.Sub(lastWheelRunAt) <= grouperRef.window
		lastWheelRun, lastWheelRunAt = name, now
		if same {
			return Event{}, false
		}
		// The scroll ends a run of action keys: backspace after it shows
		// again.
		lastAction, lastActionAt = 0, time.Time{}
		return emit(groupMouse, buildComboLabel(name)), true
	}
	// A fast sideways wheel can fire dozens of events per second and would
	// push everything else off the strip.
	if strings.HasPrefix(name, "wheel") {
		if now.Sub(lastWheel) < 70*time.Millisecond {
			return Event{}, false
		}
		lastWheel = now
	}
	clearActionRun()
	return emit(groupMouse, buildComboLabel(name)), true
}

func buildCombo(vk uint32) string {
	return buildComboLabel(keyLabel(vk))
}

// typingModifiers reports whether the modifiers held only shift which
// character a key types: shift alone, or AltGr. Right alt is AltGr on Windows
// (the system adds the ctrl that the layouts expect for it). Ctrl, left alt,
// win or any other combination is a command, and a command never types a
// character.
func typingModifiers() bool {
	for m := range heldMods {
		switch m {
		case vkShift:
		case vkControl, vkAlt:
			if !rightAlt {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// buildComboLabel prefixes the active modifiers to the given key name:
// "ctrl+shift+p". Other keys held at the same time are ignored on purpose.
func buildComboLabel(key string) string {
	parts := make([]string, 0, 5)
	for _, m := range modOrder {
		if heldMods[m.vk] {
			parts = append(parts, m.name)
		}
	}
	parts = append(parts, key)
	return strings.Join(parts, "+")
}

// emit asks the grouper what to do with this input and hands the resulting
// instruction to the UI. It never blocks: it must not stall the hook callback.
// It returns the instruction so callers and tests can see it.
func emit(kind, label string) Event {
	ev := grouperRef.feed(time.Now(), kind, label)
	select {
	case hookEvents <- ev:
	default:
	}
	return ev
}
