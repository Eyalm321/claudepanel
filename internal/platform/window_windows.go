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
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetWindowRgn               = user32.NewProc("SetWindowRgn")

	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	procCreateRectRgn = gdi32.NewProc("CreateRectRgn")

	shell32         = syscall.NewLazyDLL("shell32.dll")
	procSHAppBarMsg = shell32.NewProc("SHAppBarMessage")

	dwmapi                           = syscall.NewLazyDLL("dwmapi.dll")
	procDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	dwmwaNcRenderingPolicy = uintptr(2)
	dwmncrpDisabled        = uint32(1)
)

type dwmMargins struct {
	cxLeftWidth, cxRightWidth, cyTopHeight, cyBottomHeight int32
}

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
	swpNozorder       = uintptr(0x0004)
	lwaAlpha          = uintptr(0x2)

	swHide = uintptr(0)
	swShow = uintptr(5)

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

// MoveWindow shifts the window to (x, y) without changing its size or
// Z-order. Used by the slide animation — the OS window itself moves up off
// the top of the screen so the dark window background goes with it rather
// than being left behind after the bar slides out.
func MoveWindow(hwnd uintptr, x, y int) {
	procSetWindowPos.Call(
		hwnd, 0,
		uintptr(x), uintptr(y),
		0, 0,
		swpNoactivate|swpNosize|swpNozorder|swpNosendchanging,
	)
}

func SetWindowHeight(hwnd uintptr, physHeight int) {
	var wr rect32
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	w := int(wr.Right - wr.Left)
	procSetWindowPos.Call(
		hwnd, hwndTopmost,
		uintptr(wr.Left), uintptr(wr.Top),
		uintptr(w), uintptr(physHeight),
		swpNoactivate|swpShowwindow|swpNosendchanging,
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

// HideWindow / ShowWindow are used by the auto-hide path to make the window
// completely disappear from the desktop once the slide-up animation
// finishes — eliminates any residual chrome that transparency tricks leave.
func HideWindow(hwnd uintptr) { procShowWindow.Call(hwnd, swHide) }
func ShowWindow(hwnd uintptr) { procShowWindow.Call(hwnd, swShow) }

// SetWindowClipTop installs a region that masks out the top `topClip` pixels
// of the window. As the slide animation moves the window up past mon.Top, we
// raise topClip in lock-step so the portion that would otherwise spill onto a
// monitor above stays invisible. (SetWindowRgn takes ownership of the region
// handle; do not delete it.)
func SetWindowClipTop(hwnd uintptr, width, height, topClip int) {
	if topClip < 0 {
		topClip = 0
	}
	if topClip > height {
		topClip = height
	}
	r, _, _ := procCreateRectRgn.Call(0, uintptr(topClip), uintptr(width), uintptr(height))
	procSetWindowRgn.Call(hwnd, r, 1) // 1 = redraw
}

// ResetDwmFrame collapses the DWM-extended client-area frame to zero on every
// edge AND disables DWM non-client rendering entirely. Wails enables frame
// extension for transparent windows, which keeps re-rendering a chrome strip
// when the window loses focus even if you collapse the margins. Disabling
// NC rendering is the only way to suppress it for good.
func ResetDwmFrame(hwnd uintptr) {
	m := dwmMargins{}
	procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&m)))

	policy := dwmncrpDisabled
	procDwmSetWindowAttribute.Call(
		hwnd,
		dwmwaNcRenderingPolicy,
		uintptr(unsafe.Pointer(&policy)),
		unsafe.Sizeof(policy),
	)
}

// GetCursorPos returns the current cursor position in virtual screen coords
// (top-left of the primary monitor is 0,0). Used for hover detection because
// WebView2's mouseleave is unreliable when the cursor exits a small window.
func GetCursorPos() (int, int) {
	var p struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return int(p.X), int(p.Y)
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
