package platform

import (
	"syscall"
	"unsafe"

	"keyhud/internal/win32"
)

// This file makes the webview window behave like an overlay instead of a
// normal application window, and refuses to run twice.

// singleInstanceMutex is kept for the whole process lifetime. Releasing it
// early would defeat the check.
var singleInstanceMutex uintptr

// AcquireSingleInstance reports whether this process is the only key-hud
// running. A second copy would install a second pair of global hooks and paint
// every keystroke twice — which looks exactly like labels overlapping, so it is
// worth refusing outright rather than debugging twice.
//
// The mutex lives in the Local namespace so it is per-session: two users logged
// in at once can each run their own HUD.
func AcquireSingleInstance() bool {
	name, err := syscall.UTF16PtrFromString(`Local\key-hud-single-instance`)
	if err != nil {
		return true
	}

	// CreateMutexW only sets ERROR_ALREADY_EXISTS; it does not clear it. Zero it
	// first so a leftover value cannot be mistaken for "already running".
	win32.SetLastError.Call(0)

	h, _, callErr := win32.CreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		// Could not create the mutex at all. Running anyway is friendlier than
		// refusing to start for a reason the user cannot act on.
		return true
	}
	singleInstanceMutex = h

	if errno, ok := callErr.(syscall.Errno); ok && errno == win32.ERROR_ALREADY_EXISTS {
		return false
	}
	return true
}

// StyleAsOverlay turns the freshly created webview window into a borderless,
// topmost overlay that exactly covers the target monitor. A WS_POPUP window
// sized to the monitor win32.Rect is what "fullscreen" means here: no caption, no
// resize grip, no taskbar entry, no F11 needed.
func StyleAsOverlay(hwnd uintptr, r win32.Rect, topmost, noActivate bool) {
	win32.SetWindowLongW.Call(hwnd, win32.GWL_STYLE, uintptr(win32.WS_POPUP|win32.WS_VISIBLE))

	ex := uintptr(win32.WS_EX_TOOLWINDOW)
	if topmost {
		ex |= win32.WS_EX_TOPMOST
	}
	if noActivate {
		ex |= win32.WS_EX_NOACTIVATE
	}
	win32.SetWindowLongW.Call(hwnd, win32.GWL_EXSTYLE, ex)

	insertAfter := uintptr(0) // HWND_TOP
	if topmost {
		insertAfter = ^uintptr(0) // HWND_TOPMOST
	}

	win32.SetWindowPos.Call(
		hwnd,
		insertAfter,
		uintptr(r.Left),
		uintptr(r.Top),
		uintptr(r.W()),
		uintptr(r.H()),
		win32.SWP_FRAMECHANGED|win32.SWP_SHOWWINDOW|win32.SWP_NOACTIVATE,
	)
}
