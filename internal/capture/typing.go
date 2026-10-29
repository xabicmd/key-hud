package capture

import (
	"unicode/utf16"
	"unsafe"

	"keyhud/internal/win32"
)

// What a key press types is decided by the active keyboard layout, not by the
// key name. On the layout this app was written for, shift+2 types "@", shift+6
// starts a dead circumflex ("^" then "a" gives "â"), and "'" then "a" gives
// "á". ToUnicodeEx is the API that answers exactly that question.
//
// ToUnicodeEx reads, and by default also writes, the kernel-mode keyboard
// buffer. That buffer is shared with every application: a translation that
// stores a dead key consumes the accent the user is typing right now, and the
// whole desktop stops producing accented characters while this app runs. That
// was a real bug here: it ate accents and "ñ". Bit 2 of wFlags makes the
// translation read-only, and because a pending accent is still visible to the
// caller, a dead key followed by a letter still shows the composed character.

const (
	vkCapital  = 0x14 // VK_CAPITAL
	vkNumLock  = 0x90 // VK_NUMLOCK
	mapVKToVSC = 0

	// flagNoStateChange is bit 2 of the ToUnicodeEx wFlags parameter: translate
	// without changing the keyboard state (Windows 10, version 1607 and newer).
	flagNoStateChange = 0x04
)

// keyText is what the layout produces for one key press.
type keyText struct {
	text string // printable characters, empty when this press types nothing
	dead bool   // a dead key is waiting for the next press to compose
}

// layoutKeyText builds the modifier state the hook has seen and asks the layout
// to translate the press.
//
// It is the pipeline's default for readKeyText, and it reads the pipeline's own
// held-modifier state: what a key types depends on what is held right now, so it
// has no meaning without that state.
func (p *Pipeline) layoutKeyText(vk, scan uint32) keyText {
	var state [256]byte
	if p.heldMods[vkShift] {
		state[vkShift] = 0x80
	}
	if p.heldMods[vkControl] || p.rightAlt {
		// AltGr is reported as right alt plus ctrl; supplying the ctrl even
		// when the hook has not seen it yet keeps the translation correct.
		state[vkControl] = 0x80
	}
	if p.heldMods[vkAlt] {
		state[vkAlt] = 0x80
	}
	if keyToggled(vkCapital) {
		state[vkCapital] = 0x01
	}
	if keyToggled(vkNumLock) {
		state[vkNumLock] = 0x01
	}

	var buf [8]uint16
	n, _, _ := win32.ToUnicodeEx.Call(
		uintptr(vk),
		uintptr(scan),
		uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		flagNoStateChange,
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
	if fg, _, _ := win32.GetForegroundWindow.Call(); fg != 0 {
		var processID uint32
		if tid, _, _ := win32.GetWindowThreadProcessID.Call(fg, uintptr(unsafe.Pointer(&processID))); tid != 0 {
			if hkl, _, _ := win32.GetKeyboardLayout.Call(tid); hkl != 0 {
				return hkl
			}
		}
	}
	hkl, _, _ := win32.GetKeyboardLayout.Call(0)
	return hkl
}

// keyToggled reports a lock key (caps, num) that is currently on. The async
// state is used because the hook thread never receives those key messages.
func keyToggled(vk uint32) bool {
	r, _, _ := win32.GetAsyncKeyState.Call(uintptr(vk))
	return r&0x0001 != 0
}

// scanCode recovers the scan code for a virtual key. The hook already carries
// it for real keystrokes; this is the fallback and what tests use.
func scanCode(vk uint32) uint32 {
	r, _, _ := win32.MapVirtualKeyExW.Call(uintptr(vk), mapVKToVSC, activeLayout())
	return uint32(r)
}
