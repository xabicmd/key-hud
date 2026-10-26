package main

import (
	"log"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	hookEvents   chan Event
	hookCallback uintptr // keeps the Windows callback alive for the GC
	grouperRef   *grouper

	// ignoreInjected drops synthetic input (macros, remote sessions). Off by
	// default: a key you did not physically press is still a key press.
	ignoreInjected bool

	// All of the state below is touched only by the hook thread, which is
	// locked to its OS thread, so no synchronization is needed.
	pressed   = map[uint32]bool{}
	heldMods  = map[uint32]bool{}
	rightAlt  bool // AltGr is right alt plus the ctrl Windows adds for it
	lastWheel time.Time
)

// startHooks installs the global low-level keyboard and mouse hooks and
// returns the channel they publish into.
//
// Two hard constraints drive the shape of this function:
//  1. A low-level hook only fires while the installing thread pumps Win32
//     messages, so this goroutine locks itself to its OS thread and blocks in
//     GetMessage forever. It never does anything else.
//  2. The hook callback runs inside a Windows timeout window, so it must never
//     block. It does the minimum work and drops events on a full channel.
func startHooks(quit chan struct{}) <-chan Event {
	hookEvents = make(chan Event, 512)
	if grouperRef == nil {
		grouperRef = newGrouper(500*time.Millisecond, 3)
	}
	ready := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		cb := syscall.NewCallback(hookProc)
		hookCallback = cb

		hInst, _, _ := procGetModuleHandleW.Call(0)

		if hk, _, err := procSetWindowsHookExW.Call(whKeyboardLL, cb, hInst, 0); hk == 0 {
			log.Printf("keyboard hook failed: %v", err)
		}
		if hk, _, err := procSetWindowsHookExW.Call(whMouseLL, cb, hInst, 0); hk == 0 {
			log.Printf("mouse hook failed: %v", err)
		}

		// Ctrl+Alt+Q is the only way out: the window has no close button.
		procRegisterHotKey.Call(0, 1, modControl|modAlt, 'Q')

		close(ready)

		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.Message == wmHotkey {
				select {
				case <-quit: // already closing
				default:
					close(quit)
				}
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()

	<-ready
	return hookEvents
}

func hookProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode == hcAction {
		switch uint32(wParam) {
		case wmKeyDown, wmSysKeyDown:
			onKey(lParam, true)
		case wmKeyUp, wmSysKeyUp:
			onKey(lParam, false)
		case wmLButtonDown:
			onButton("click")
		case wmRButtonDown:
			onButton("right click")
		case wmMButtonDown:
			onButton("middle click")
		case wmXButtonDown:
			ms := (*msLLHookStruct)(ptrAt(lParam))
			if ms.MouseData>>16 == 1 {
				onButton("mouse back")
			} else {
				onButton("mouse forward")
			}
		case wmMouseWheel:
			ms := (*msLLHookStruct)(ptrAt(lParam))
			if int16(ms.MouseData>>16) > 0 {
				onButton("wheel up")
			} else {
				onButton("wheel down")
			}
		case wmMouseHWheel:
			ms := (*msLLHookStruct)(ptrAt(lParam))
			if int16(ms.MouseData>>16) > 0 {
				onButton("wheel right")
			} else {
				onButton("wheel left")
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}

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
		return emit(kindCombo, buildCombo(vk)), true
	}
	return emit(kindAction, keyLabel(vk)), true
}

func onButton(name string) {
	// A fast scroll wheel can fire dozens of events per second and would push
	// everything else off the strip.
	if strings.HasPrefix(name, "wheel") {
		now := time.Now()
		if now.Sub(lastWheel) < 70*time.Millisecond {
			return
		}
		lastWheel = now
	}
	emit(groupMouse, buildComboLabel(name))
}

func buildCombo(vk uint32) string {
	return buildComboLabel(keyLabel(vk))
}

// typingModifiers reports whether the modifiers held only shift which
// character a key types: shift alone, or AltGr, which Windows reports as right
// alt plus the ctrl it adds for it. Ctrl, alt, win or any other combination is
// a command, and a command never types a character.
func typingModifiers() bool {
	altGr := rightAlt && heldMods[vkControl]
	for m := range heldMods {
		switch m {
		case vkShift:
		case vkControl, vkAlt:
			if !altGr {
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
