package main

import (
	"unicode/utf16"
	"unsafe"
)

// What a key press types is decided by the active keyboard layout, not by the
// key name. On the layout this app was written for, shift+2 types "@", shift+6
// starts a dead circumflex ("^" then "a" gives "â"), and "'" then "a" gives
// "á". ToUnicodeEx is the API that answers exactly that question.
//
// ToUnicodeEx also owns the dead-key state, which Windows keeps per thread. The
// hook thread is locked to one OS thread, so letting it translate every press
// mirrors what the foreground application does with the same key sequence:
// nothing has to be composed by hand.

const (
	vkMenu    = 0x12 // VK_MENU enters the key-state array as "alt"
	vkCapital = 0x14 // VK_CAPITAL
	vkNumLock = 0x90 // VK_NUMLOCK

	mapVKToVSC = 0
)

// keyText is what the layout produces for one key press.
type keyText struct {
	text string // printable characters, empty when this press types nothing
	dead bool   // a dead key is waiting for the next press to compose
}

// readKeyText asks the active layout what a key press types. It is a variable so
// tests can answer without depending on the layout installed on the machine.
var readKeyText = layoutKeyText

// layoutKeyText builds the modifier state the hook has seen and asks the layout
// to translate the press.
func layoutKeyText(vk, scan uint32) keyText {
	var state [256]byte
	if heldMods[vkShift] {
		state[vkShift] = 0x80
	}
	if heldMods[vkControl] {
		state[vkControl] = 0x80
	}
	if heldMods[vkAlt] {
		state[vkMenu] = 0x80
	}
	if keyToggled(vkCapital) {
		state[vkCapital] = 0x01
	}
	if keyToggled(vkNumLock) {
		state[vkNumLock] = 0x01
	}

	var buf [8]uint16
	n, _, _ := procToUnicodeEx.Call(
		uintptr(vk),
		uintptr(scan),
		uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
		activeLayout(),
	)

	switch {
	case int32(n) < 0:
		// Dead key: the accent is held back until the next press decides which
		// character it becomes. Nothing appeared on screen yet.
		return keyText{dead: true}
	case int32(n) == 0:
		return keyText{}
	}

	// Some dead keys produce two characters at once: "´" followed by a letter
	// that cannot compose types both, and that is exactly what the user sees.
	text := string(utf16.Decode(buf[:int32(n)]))
	if !isPrintable(text) {
		return keyText{}
	}
	return keyText{text: text}
}

// isPrintable rejects the control characters the layout reports for keys that
// type nothing visible, such as enter (0x0d) or ctrl+c (0x03).
func isPrintable(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// activeLayout is the layout of the window that will receive the keystroke: an
// application may run its own keyboard layout, and the character shown must be
// the one it types. Without a foreground window the thread's own layout is the
// best available answer.
func activeLayout() uintptr {
	if fg, _, _ := procGetForegroundWindow.Call(); fg != 0 {
		var pid uint32
		if tid, _, _ := procGetWindowThreadProcessID.Call(fg, uintptr(unsafe.Pointer(&pid))); tid != 0 {
			if hkl, _, _ := procGetKeyboardLayout.Call(tid); hkl != 0 {
				return hkl
			}
		}
	}
	hkl, _, _ := procGetKeyboardLayout.Call(0)
	return hkl
}

// keyToggled reports a lock key (caps, num) that is currently on. The async
// state is used because the hook thread never receives those key messages.
func keyToggled(vk uint32) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x0001 != 0
}

// scanCode recovers the scan code for a virtual key. The hook already carries
// it for real keystrokes; this is the fallback and what tests use.
func scanCode(vk uint32) uint32 {
	r, _, _ := procMapVirtualKeyExW.Call(uintptr(vk), mapVKToVSC, activeLayout())
	return uint32(r)
}
