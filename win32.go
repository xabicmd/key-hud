package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

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

type monitor struct {
	Rect    rect
	Primary bool
}

func enumMonitors() []monitor {
	var out []monitor
	cb := syscall.NewCallback(func(hMon, hdc, lprc, data uintptr) uintptr {
		mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
		procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
		out = append(out, monitor{Rect: mi.RcMonitor, Primary: mi.DwFlags&1 != 0})
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return out
}

// pickMonitor resolves the target screen from a spec of the form "1920x480"
// (match by size), "2" (1-based index) or "primary".
func pickMonitor(spec string) (rect, error) {
	mons := enumMonitors()
	if len(mons) == 0 {
		return rect{}, fmt.Errorf("no monitors detected")
	}

	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" || spec == "primary" {
		for _, m := range mons {
			if m.Primary {
				return m.Rect, nil
			}
		}
		return mons[0].Rect, nil
	}

	if w, h, ok := parseSize(spec); ok {
		for _, m := range mons {
			if int(m.Rect.W()) == w && int(m.Rect.H()) == h {
				return m.Rect, nil
			}
		}
		return rect{}, fmt.Errorf("no monitor matches %dx%d (available: %s)", w, h, describeMonitors(mons))
	}

	if n, err := strconv.Atoi(spec); err == nil {
		if n >= 1 && n <= len(mons) {
			return mons[n-1].Rect, nil
		}
		return rect{}, fmt.Errorf("monitor index %d out of range 1..%d", n, len(mons))
	}

	return rect{}, fmt.Errorf("invalid -monitor value %q (use WxH, an index, or \"primary\")", spec)
}

func parseSize(s string) (w, h int, ok bool) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func describeMonitors(mons []monitor) string {
	parts := make([]string, 0, len(mons))
	for i, m := range mons {
		tag := ""
		if m.Primary {
			tag = " primary"
		}
		parts = append(parts, fmt.Sprintf("%d:%dx%d@%d,%d%s", i+1, m.Rect.W(), m.Rect.H(), m.Rect.Left, m.Rect.Top, tag))
	}
	return strings.Join(parts, " ")
}

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
