package main

import (
	"log"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// This file installs the global low-level hooks and dispatches raw Windows
// messages. Interpreting what those messages mean is input.go's job.

var (
	hookEvents   chan Event
	hookCallback uintptr // keeps the Windows callback alive for the GC
	grouperRef   *grouper
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
