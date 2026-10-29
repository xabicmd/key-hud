package capture

import (
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unsafe"

	"keyhud/internal/win32"
)

// The machine's real layout decides the characters, and Windows composes dead
// accents. These tests find whichever dead key the layout has instead of
// assuming an accent, and they skip when the layout has none.

// deadKeyOf returns a dead key on the punctuation row, or skips the test.
func deadKeyOf(t *testing.T) uint32 {
	t.Helper()
	for vk := uint32(0xBA); vk <= 0xDE; vk++ {
		if p.layoutKeyText(vk, scanCode(vk)).dead {
			return vk
		}
	}
	t.Skip("the active layout has no dead key on the punctuation row")
	return 0
}

// rawTranslate calls ToUnicodeEx without the guard, so a test can move the
// shared kernel keyboard buffer into the state it needs.
func rawTranslate(vk, flags uint32) int32 {
	var state [256]byte
	var buf [8]uint16
	n, _, _ := win32.ToUnicodeEx.Call(
		uintptr(vk),
		uintptr(scanCode(vk)),
		uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(flags),
		activeLayout(),
	)
	return int32(n)
}

// clearPendingDeadKey consumes whatever accent the shared kernel keyboard buffer
// is holding, so a test starts from a known state.
func clearPendingDeadKey() {
	for i := 0; i < 2; i++ {
		rawTranslate(0x20, 0) // a space never composes: it drops the accent
	}
}

// pendingDeadKey stores a dead accent the way Windows does when that accent is
// delivered to the window in front.
func pendingDeadKey(t *testing.T, vk uint32) {
	t.Helper()
	if n := rawTranslate(vk, 0); n != -1 {
		t.Fatalf("dead key 0x%02X returned %d, want -1", vk, n)
	}
}

// Translating a press must not touch the kernel keyboard buffer. That buffer is
// shared with every application: a translation that stores a dead key eats the
// accent the user is typing right now, and the whole system stops producing
// accented characters while this app runs.
func TestLayoutKeyTextLeavesTheKeyboardStateUntouched(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	resetComboState()
	clearPendingDeadKey()
	defer clearPendingDeadKey()

	deadKeyOf(t) // translating the accent is what used to arm the corruption

	if next := p.layoutKeyText(0x41, scanCode(0x41)); !strings.EqualFold(next.text, "a") {
		t.Fatalf("letter after translating a dead key = %+v, want a plain a", next)
	}
}

// Reading the composed character must still work: when the accent is already
// pending, the translation reports the letter it will become.
func TestLayoutKeyTextComposesFromAPendingDeadKey(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	resetComboState()
	clearPendingDeadKey()
	defer clearPendingDeadKey()

	deadVK := deadKeyOf(t)
	pendingDeadKey(t, deadVK)

	composed := p.layoutKeyText(0x41, scanCode(0x41))
	if composed.dead || composed.text == "" || strings.EqualFold(composed.text, "a") {
		t.Fatalf("dead key 0x%02X then a = %+v, want one composed letter", deadVK, composed)
	}
	if r, ok := loneRune(composed.text); !ok || !unicode.IsLetter(r) {
		t.Fatalf("composed text = %q, want a single letter", composed.text)
	}

	// Reading it for the strip must not consume it: the window in front still
	// has to compose the very same accent.
	if again := p.layoutKeyText(0x41, scanCode(0x41)); !strings.EqualFold(again.text, composed.text) {
		t.Fatalf("second read = %+v, want the same composed letter", again)
	}
}
