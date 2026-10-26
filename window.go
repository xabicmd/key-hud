package main

import (
	"syscall"
	"unsafe"
)

// This file makes the webview window behave like an overlay instead of a
// normal application window, and refuses to run twice.

// singleInstanceMutex is kept for the whole process lifetime. Releasing it
// early would defeat the check.
var singleInstanceMutex uintptr

// acquireSingleInstance reports whether this process is the only key-hud
// running. A second copy would install a second pair of global hooks and paint
// every keystroke twice — which looks exactly like labels overlapping, so it is
// worth refusing outright rather than debugging twice.
//
// The mutex lives in the Local namespace so it is per-session: two users logged
// in at once can each run their own HUD.
func acquireSingleInstance() bool {
	name, err := syscall.UTF16PtrFromString(`Local\key-hud-single-instance`)
	if err != nil {
		return true
	}

	// CreateMutexW only sets ERROR_ALREADY_EXISTS; it does not clear it. Zero it
	// first so a leftover value cannot be mistaken for "already running".
	procSetLastError.Call(0)

	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		// Could not create the mutex at all. Running anyway is friendlier than
		// refusing to start for a reason the user cannot act on.
		return true
	}
	singleInstanceMutex = h

	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		return false
	}
	return true
}

// styleAsOverlay turns the freshly created webview window into a borderless,
// topmost overlay that exactly covers the target monitor. A WS_POPUP window
// sized to the monitor rect is what "fullscreen" means here: no caption, no
// resize grip, no taskbar entry, no F11 needed.
func styleAsOverlay(hwnd uintptr, r rect, topmost, noActivate bool) {
	procSetWindowLongW.Call(hwnd, gwlStyle, uintptr(wsPopup|wsVisible))

	ex := uintptr(wsExToolWindow)
	if topmost {
		ex |= wsExTopmost
	}
	if noActivate {
		ex |= wsExNoActivate
	}
	procSetWindowLongW.Call(hwnd, gwlExStyle, ex)

	insertAfter := uintptr(0) // HWND_TOP
	if topmost {
		insertAfter = ^uintptr(0) // HWND_TOPMOST
	}

	procSetWindowPos.Call(
		hwnd,
		insertAfter,
		uintptr(r.Left),
		uintptr(r.Top),
		uintptr(r.W()),
		uintptr(r.H()),
		swpFrameChanged|swpShowWindow|swpNoActivate,
	)
}
