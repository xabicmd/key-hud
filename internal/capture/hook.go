package capture

import (
	"log"
	"runtime"
	"syscall"
	"unsafe"

	"keyhud/internal/win32"
)

// This file installs the global low-level hooks and dispatches raw Windows
// messages. Interpreting what those messages mean is input.go's job.

// hookEventBuffer is the capacity of the event channel. The hook callback must
// never block, so a full channel drops events instead of stalling Windows.
const hookEventBuffer = 512

// hookCallback keeps the Windows callback alive for the garbage collector. One
// process runs one pipeline, so one slot is enough.
var hookCallback uintptr

// Start installs the global low-level keyboard and mouse hooks and returns the
// channel they publish into. It blocks until the hooks are in place, so the
// caller knows no keystroke can slip through after it returns.
//
// Two hard constraints drive the shape of this function:
//  1. A low-level hook only fires while the installing thread pumps Win32
//     messages, so this goroutine locks itself to its OS thread and blocks in
//     GetMessage forever. It never does anything else.
//  2. The hook callback runs inside a Windows timeout window, so it must never
//     block. It does the minimum work and drops events on a full channel.
func (p *Pipeline) Start(quit chan struct{}) <-chan Event {
	p.events = make(chan Event, hookEventBuffer)
	ready := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		cb := syscall.NewCallback(p.hookProc)
		hookCallback = cb

		hInst, _, _ := win32.GetModuleHandleW.Call(0)

		if hk, _, err := win32.SetWindowsHookExW.Call(win32.WH_KEYBOARD_LL, cb, hInst, 0); hk == 0 {
			log.Printf("keyboard hook failed: %v", err)
		}
		if hk, _, err := win32.SetWindowsHookExW.Call(win32.WH_MOUSE_LL, cb, hInst, 0); hk == 0 {
			log.Printf("mouse hook failed: %v", err)
		}

		// Ctrl+Alt+Q is the only way out: the window has no close button.
		win32.RegisterHotKey.Call(0, 1, win32.MOD_CONTROL|win32.MOD_ALT, 'Q')

		close(ready)

		var m win32.Msg
		for {
			r, _, _ := win32.GetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.Message == win32.WM_HOTKEY {
				select {
				case <-quit: // already closing
				default:
					close(quit)
				}
				return
			}
			win32.TranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			win32.DispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()

	<-ready
	return p.events
}

func (p *Pipeline) hookProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode == win32.HC_ACTION {
		switch uint32(wParam) {
		case win32.WM_KEYDOWN, win32.WM_SYSKEYDOWN:
			p.onKey(lParam, true)
		case win32.WM_KEYUP, win32.WM_SYSKEYUP:
			p.onKey(lParam, false)
		case win32.WM_LBUTTONDOWN:
			p.onButton("click")
		case win32.WM_RBUTTONDOWN:
			p.onButton("right click")
		case win32.WM_MBUTTONDOWN:
			p.onButton("middle click")
		case win32.WM_XBUTTONDOWN:
			ms := (*win32.MsLLHookStruct)(win32.PtrAt(lParam))
			if ms.MouseData>>16 == 1 {
				p.onButton("mouse back")
			} else {
				p.onButton("mouse forward")
			}
		case win32.WM_MOUSEWHEEL:
			ms := (*win32.MsLLHookStruct)(win32.PtrAt(lParam))
			if int16(ms.MouseData>>16) > 0 {
				p.onButton("wheel up")
			} else {
				p.onButton("wheel down")
			}
		case win32.WM_MOUSEHWHEEL:
			ms := (*win32.MsLLHookStruct)(win32.PtrAt(lParam))
			if int16(ms.MouseData>>16) > 0 {
				p.onButton("wheel right")
			} else {
				p.onButton("wheel left")
			}
		}
	}
	r, _, _ := win32.CallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return r
}
