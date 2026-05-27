package syswin

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
)

const (
	gwlStyle        = uintptr(0xFFFFFFF0) // -16 as unsigned for syscall
	gwlExStyle      = uintptr(0xFFFFFFEC) // -20 as unsigned for syscall
	wsCaption       = uintptr(0x00C00000)
	wsThickframe    = uintptr(0x00040000)
	wsMinimizebox   = uintptr(0x00020000)
	wsMaximizebox   = uintptr(0x00010000)
	wsSysmenu       = uintptr(0x00080000)
	wsExToolwindow  = uintptr(0x00000080)
	wsExLayered     = uintptr(0x00080000)
	wsExTransparent = uintptr(0x00000020)
	hwndTopmost     = ^uintptr(0) // (HWND)(-1)
	swpNoactivate   = uintptr(0x0010)
	swpShowwindow   = uintptr(0x0040)
	swpNosize       = uintptr(0x0001)
	swpNomove       = uintptr(0x0002)
	swpFramechanged = uintptr(0x0020)
	lwaAlpha        = uintptr(0x2)
)

// FindWindowByPID finds a visible window belonging to this process.
func FindWindowByPID() (uintptr, error) {
	pid := uint32(os.Getpid())
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var wPid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wPid)))
		if wPid == pid {
			vis, _, _ := procIsWindowVisible.Call(hwnd)
			if vis != 0 {
				// Skip the Wails hidden helper window by checking it has non-trivial size
				// (helper windows tend to be 0x0)
				found = hwnd
				return 0 // stop enumeration
			}
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return 0, fmt.Errorf("window not found for PID %d", pid)
	}
	return found, nil
}

// ApplyBarStyles strips chrome, marks as tool window, and enables layered compositing.
func ApplyBarStyles(hwnd uintptr) {
	// Remove frame chrome
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= wsCaption | wsThickframe | wsMinimizebox | wsMaximizebox | wsSysmenu
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	// Tool window (hides from taskbar + alt-tab) + layered (enables opacity)
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	exStyle |= wsExToolwindow | wsExLayered
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)

	// Force frame change so Windows recomputes chrome
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoactivate|swpNosize|swpNomove|swpFramechanged)
}

// DockToMonitor positions the bar at the top edge of the given monitor.
func DockToMonitor(hwnd uintptr, mon MonitorInfo, barHeight int) {
	procSetWindowPos.Call(
		hwnd,
		hwndTopmost,
		uintptr(mon.Left),
		uintptr(mon.Top),
		uintptr(mon.Width),
		uintptr(barHeight),
		swpNoactivate|swpShowwindow,
	)
}

// SetOpacity controls window transparency via WS_EX_LAYERED (0.0–1.0).
func SetOpacity(hwnd uintptr, opacity float64) {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	alpha := uintptr(opacity * 255)
	procSetLayeredWindowAttributes.Call(hwnd, 0, alpha, lwaAlpha)
}

// SetClickThrough toggles WS_EX_TRANSPARENT so mouse events pass through the window.
func SetClickThrough(hwnd uintptr, enabled bool) {
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	if enabled {
		exStyle |= wsExTransparent
	} else {
		exStyle &^= wsExTransparent
	}
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)
}
