package main

import (
	"syscall"
	"unsafe"
)

// This file declares the Win32 functions, constants and structures the rest of
// the app calls. It contains no decisions: only the bindings and the struct
// layouts that must match the C definitions exactly.

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetWindowsHookExW   = modUser32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = modUser32.NewProc("CallNextHookEx")
	procGetMessageW         = modUser32.NewProc("GetMessageW")
	procTranslateMessage    = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = modUser32.NewProc("DispatchMessageW")
	procRegisterHotKey      = modUser32.NewProc("RegisterHotKey")
	procEnumDisplayMonitors = modUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = modUser32.NewProc("GetMonitorInfoW")
	procSetWindowLongW      = modUser32.NewProc("SetWindowLongW")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procGetModuleHandleW    = modKernel32.NewProc("GetModuleHandleW")
	procCreateMutexW        = modKernel32.NewProc("CreateMutexW")
	procSetLastError        = modKernel32.NewProc("SetLastError")

	// Translation of a key press into the character the active layout types.
	procToUnicodeEx              = modUser32.NewProc("ToUnicodeEx")
	procGetKeyboardLayout        = modUser32.NewProc("GetKeyboardLayout")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = modUser32.NewProc("GetWindowThreadProcessId")
	procGetAsyncKeyState         = modUser32.NewProc("GetAsyncKeyState")
	procMapVirtualKeyExW         = modUser32.NewProc("MapVirtualKeyExW")
)

const errorAlreadyExists = 183

const (
	whKeyboardLL = 13
	whMouseLL    = 14
	hcAction     = 0

	llkhfInjected = 0x10

	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmMButtonDown = 0x0207
	wmXButtonDown = 0x020B
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmHotkey      = 0x0312

	modAlt     = 0x0001
	modControl = 0x0002

	gwlStyle   = ^uintptr(15) // GWL_STYLE == -16
	gwlExStyle = ^uintptr(19) // GWL_EXSTYLE == -20

	wsPopup   = 0x80000000
	wsVisible = 0x10000000

	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	wsExNoActivate = 0x08000000

	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
)

// ptrAt reinterprets an OS-supplied integer as a pointer.
//
// The Win32 hook callback delivers lParam as a uintptr to a struct that lives
// on the OS stack for the duration of the call; it never points into the Go
// heap. The indirection through &p expresses that faithfully and keeps both
// `go vet`'s unsafeptr heuristic and `-d=checkptr` quiet about a conversion
// that is correct here.
func ptrAt(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

type point struct {
	X, Y int32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

func (r rect) W() int32 { return r.Right - r.Left }
func (r rect) H() int32 { return r.Bottom - r.Top }

type msg struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msLLHookStruct struct {
	Pt          point
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}
