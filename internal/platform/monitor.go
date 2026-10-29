package platform

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"keyhud/internal/win32"
)

// This file answers one question: which screen should the strip cover?

type monitor struct {
	Rect    win32.Rect
	Primary bool
}

func enumMonitors() []monitor {
	var out []monitor
	cb := syscall.NewCallback(func(hMon, hdc, lprc, data uintptr) uintptr {
		mi := win32.MonitorInfo{CbSize: uint32(unsafe.Sizeof(win32.MonitorInfo{}))}
		win32.GetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
		out = append(out, monitor{Rect: mi.RcMonitor, Primary: mi.DwFlags&1 != 0})
		return 1
	})
	win32.EnumDisplayMonitors.Call(0, 0, cb, 0)
	return out
}

// PickMonitor resolves the target screen from a spec of the form "1920x480"
// (match by size), "2" (1-based index) or "primary".
func PickMonitor(spec string) (win32.Rect, error) {
	mons := enumMonitors()
	if len(mons) == 0 {
		return win32.Rect{}, fmt.Errorf("no monitors detected")
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
		return win32.Rect{}, fmt.Errorf("no monitor matches %dx%d (available: %s)", w, h, describeMonitors(mons))
	}

	if n, err := strconv.Atoi(spec); err == nil {
		if n >= 1 && n <= len(mons) {
			return mons[n-1].Rect, nil
		}
		return win32.Rect{}, fmt.Errorf("monitor index %d out of range 1..%d", n, len(mons))
	}

	return win32.Rect{}, fmt.Errorf("invalid -monitor value %q (use WxH, an index, or \"primary\")", spec)
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
