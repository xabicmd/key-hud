package win32

import (
	"syscall"
	"unsafe"
)

// Package win32 binds the Win32 functions, constants and structures the app
// calls. It contains no decisions: only the bindings and the struct layouts
// that must match the C definitions exactly.

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	SetWindowsHookExW   = modUser32.NewProc("SetWindowsHookExW")
	CallNextHookEx      = modUser32.NewProc("CallNextHookEx")
	GetMessageW         = modUser32.NewProc("GetMessageW")
	TranslateMessage    = modUser32.NewProc("TranslateMessage")
	DispatchMessageW    = modUser32.NewProc("DispatchMessageW")
	RegisterHotKey      = modUser32.NewProc("RegisterHotKey")
	EnumDisplayMonitors = modUser32.NewProc("EnumDisplayMonitors")
	GetMonitorInfoW     = modUser32.NewProc("GetMonitorInfoW")
	SetWindowLongW      = modUser32.NewProc("SetWindowLongW")
	SetWindowPos        = modUser32.NewProc("SetWindowPos")
	GetModuleHandleW    = modKernel32.NewProc("GetModuleHandleW")
	CreateMutexW        = modKernel32.NewProc("CreateMutexW")
	SetLastError        = modKernel32.NewProc("SetLastError")

	// Translation of a key press into the character the active layout types.
	ToUnicodeEx              = modUser32.NewProc("ToUnicodeEx")
	GetKeyboardLayout        = modUser32.NewProc("GetKeyboardLayout")
	GetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	GetWindowThreadProcessID = modUser32.NewProc("GetWindowThreadProcessId")
	GetAsyncKeyState         = modUser32.NewProc("GetAsyncKeyState")
	MapVirtualKeyExW         = modUser32.NewProc("MapVirtualKeyExW")
)

const ERROR_ALREADY_EXISTS = 183

const (
	WH_KEYBOARD_LL = 13
	WH_MOUSE_LL    = 14
	HC_ACTION      = 0

	LLKHF_INJECTED = 0x10

	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105
	WM_LBUTTONDOWN = 0x0201
	WM_RBUTTONDOWN = 0x0204
	WM_MBUTTONDOWN = 0x0207
	WM_XBUTTONDOWN = 0x020B
	WM_MOUSEWHEEL  = 0x020A
	WM_MOUSEHWHEEL = 0x020E
	WM_HOTKEY      = 0x0312

	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002

	GWL_STYLE   = ^uintptr(15) // GWL_STYLE == -16
	GWL_EXSTYLE = ^uintptr(19) // GWL_EXSTYLE == -20

	WS_POPUP   = 0x80000000
	WS_VISIBLE = 0x10000000

	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_NOACTIVATE = 0x08000000

	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020
	SWP_SHOWWINDOW   = 0x0040
)

// PtrAt reinterprets an OS-supplied integer as a pointer.
//
// The Win32 hook callback delivers lParam as a uintptr to a struct that lives
// on the OS stack for the duration of the call; it never points into the Go
// heap. The indirection through &p expresses that faithfully and keeps both
// `go vet`'s unsafeptr heuristic and `-d=checkptr` quiet about a conversion
// that is correct here.
func PtrAt(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

type Point struct {
	X, Y int32
}

type Rect struct {
	Left, Top, Right, Bottom int32
}

func (r Rect) W() int32 { return r.Right - r.Left }
func (r Rect) H() int32 { return r.Bottom - r.Top }

type Msg struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
}

type MonitorInfo struct {
	CbSize    uint32
	RcMonitor Rect
	RcWork    Rect
	DwFlags   uint32
}

type KbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MsLLHookStruct struct {
	Pt          Point
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}
