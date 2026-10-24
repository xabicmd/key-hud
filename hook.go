package main

import (
	"log"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Event is what the browser side receives for every captured input.
type Event struct {
	Kind  string `json:"kind"`  // "key" or "mouse"
	Label string `json:"label"` // "ctrl+c", "click", "wheel up"
}

var (
	hookEvents   chan Event
	hookCallback uintptr // keeps the Windows callback alive for the GC

	// ignoreInjected drops synthetic input (macros, remote sessions). Off by
	// default: a key you did not physically press is still a key press.
	ignoreInjected bool

	// All of the state below is touched only by the hook thread, which is
	// locked to its OS thread, so no synchronization is needed.
	pressed   = map[uint32]bool{}
	heldMods  = map[uint32]bool{}
	heldOrder []uint32
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
	vk := kb.VkCode

	if ignoreInjected && kb.Flags&llkhfInjected != 0 {
		return
	}

	if !down {
		delete(pressed, vk)
		if isModifier(vk) {
			delete(heldMods, canonMod(vk))
		} else {
			removeHeld(vk)
		}
		return
	}

	// Auto-repeat: Windows keeps sending keydowns while a key is held. Show the
	// press once, not sixty times a second.
	if pressed[vk] {
		return
	}
	pressed[vk] = true

	if isModifier(vk) {
		heldMods[canonMod(vk)] = true
		return
	}

	label := buildCombo(vk)
	heldOrder = append(heldOrder, vk)
	emit(Event{Kind: "key", Label: label})
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
	emit(Event{Kind: "mouse", Label: buildComboLabel(name)})
}

func buildCombo(vk uint32) string {
	return buildComboLabel(keyLabel(vk))
}

// buildComboLabel prefixes the active modifiers (and any other keys currently
// held down) to the given key name: "ctrl+shift+p".
func buildComboLabel(key string) string {
	parts := make([]string, 0, 5)
	for _, m := range modOrder {
		if heldMods[m.vk] {
			parts = append(parts, m.name)
		}
	}
	for _, k := range heldOrder {
		parts = append(parts, keyLabel(k))
	}
	parts = append(parts, key)
	return strings.Join(parts, "+")
}

func removeHeld(vk uint32) {
	for i, k := range heldOrder {
		if k == vk {
			heldOrder = append(heldOrder[:i], heldOrder[i+1:]...)
			return
		}
	}
}

// emit never blocks: it must not stall the hook callback.
func emit(ev Event) {
	select {
	case hookEvents <- ev:
	default:
	}
}
