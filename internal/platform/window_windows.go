//go:build windows

package platform

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
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")

	shell32         = syscall.NewLazyDLL("shell32.dll")
	procSHAppBarMsg = shell32.NewProc("SHAppBarMessage")
)

const (
	gwlStyle          = uintptr(0xFFFFFFF0) // -16
	gwlExStyle        = uintptr(0xFFFFFFEC) // -20
	wsCaption         = uintptr(0x00C00000)
	wsThickframe      = uintptr(0x00040000)
	wsMinimizebox     = uintptr(0x00020000)
	wsMaximizebox     = uintptr(0x00010000)
	wsSysmenu         = uintptr(0x00080000)
	wsExToolwindow    = uintptr(0x00000080)
	wsExLayered       = uintptr(0x00080000)
	wsExTransparent   = uintptr(0x00000020)
	hwndTopmost       = ^uintptr(0) // (HWND)(-1)
	swpNoactivate     = uintptr(0x0010)
	swpShowwindow     = uintptr(0x0040)
	swpNosize         = uintptr(0x0001)
	swpNomove         = uintptr(0x0002)
	swpFramechanged   = uintptr(0x0020)
	swpNosendchanging = uintptr(0x0400)
	lwaAlpha          = uintptr(0x2)

	abmNew      = uintptr(0x00000000)
	abmRemove   = uintptr(0x00000001)
	abmQuerypos = uintptr(0x00000002)
	abmSetpos   = uintptr(0x00000003)
	abeTop      = uint32(1)
)

type appBarData struct {
	cbSize uint32
	hWnd   uintptr
	uMsg   uint32
	uEdge  uint32
	rc     rect32
	lParam uintptr
}

func FindWindowByPID() (uintptr, error) {
	pid := uint32(os.Getpid())
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var wPid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wPid)))
		if wPid == pid {
			vis, _, _ := procIsWindowVisible.Call(hwnd)
			if vis != 0 {
				found = hwnd
				return 0
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

func ApplyBarStyles(hwnd uintptr) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= wsCaption | wsThickframe | wsMinimizebox | wsMaximizebox | wsSysmenu
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	exStyle |= wsExToolwindow | wsExLayered
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)

	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoactivate|swpNosize|swpNomove|swpFramechanged)
}

func DockToMonitor(hwnd uintptr, mon MonitorInfo, barHeight int, appBarMode bool) {
	physW := mon.PhysWidth
	if physW == 0 {
		physW = mon.Width
	}
	if appBarMode {
		registerAppBar(hwnd, mon, barHeight, physW)
	}
	procSetWindowPos.Call(
		hwnd, hwndTopmost,
		uintptr(mon.Left), uintptr(mon.Top),
		uintptr(physW), uintptr(barHeight),
		swpNoactivate|swpShowwindow|swpNosendchanging,
	)
}

func RemoveAppBar(hwnd uintptr) {
	abd := appBarData{
		cbSize: uint32(unsafe.Sizeof(appBarData{})),
		hWnd:   hwnd,
	}
	procSHAppBarMsg.Call(abmRemove, uintptr(unsafe.Pointer(&abd)))
}

func registerAppBar(hwnd uintptr, mon MonitorInfo, barHeight, physW int) {
	abd := appBarData{
		cbSize: uint32(unsafe.Sizeof(appBarData{})),
		hWnd:   hwnd,
		uEdge:  abeTop,
		rc: rect32{
			Left:   mon.Left,
			Top:    mon.Top,
			Right:  mon.Left + int32(physW),
			Bottom: mon.Top + int32(barHeight),
		},
	}
	procSHAppBarMsg.Call(abmNew, uintptr(unsafe.Pointer(&abd)))
	procSHAppBarMsg.Call(abmQuerypos, uintptr(unsafe.Pointer(&abd)))
	abd.rc.Bottom = abd.rc.Top + int32(barHeight)
	procSHAppBarMsg.Call(abmSetpos, uintptr(unsafe.Pointer(&abd)))
}

func GetWindowSize(hwnd uintptr) (left, top, width, height int) {
	var wr rect32
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	return int(wr.Left), int(wr.Top), int(wr.Right - wr.Left), int(wr.Bottom - wr.Top)
}

func SetWindowHeight(hwnd uintptr, physHeight int) {
	var wr rect32
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	w := int(wr.Right - wr.Left)
	procSetWindowPos.Call(
		hwnd, hwndTopmost,
		uintptr(wr.Left), uintptr(wr.Top),
		uintptr(w), uintptr(physHeight),
		swpNoactivate|swpShowwindow,
	)
}

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

func SetClickThrough(hwnd uintptr, enabled bool) {
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	if enabled {
		exStyle |= wsExTransparent
	} else {
		exStyle &^= wsExTransparent
	}
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)
}
