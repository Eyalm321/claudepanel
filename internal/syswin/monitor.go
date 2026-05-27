package syswin

import (
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	shcore                  = syscall.NewLazyDLL("shcore.dll")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetDpiForMonitor    = shcore.NewProc("GetDpiForMonitor")
)

const (
	monitorinfofPrimary = 0x00000001
	mdtEffectiveDpi     = 0
)

type rect32 struct {
	Left, Top, Right, Bottom int32
}

// monitorInfoEx is the MONITORINFOEX Win32 struct.
type monitorInfoEx struct {
	cbSize    uint32
	rcMonitor rect32
	rcWork    rect32
	dwFlags   uint32
	szDevice  [32]uint16
}

// MonitorInfo is the app-facing monitor descriptor.
type MonitorInfo struct {
	Index     int    `json:"index"`
	Left      int32  `json:"left"`
	Top       int32  `json:"top"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"isPrimary"`
	Name      string `json:"name"`
}

var enumResults []MonitorInfo

func enumMonitorProc(hMonitor, _ uintptr, _ uintptr, _ uintptr) uintptr {
	var info monitorInfoEx
	info.cbSize = uint32(unsafe.Sizeof(info))
	procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&info)))

	var dpiX, dpiY uint32
	procGetDpiForMonitor.Call(hMonitor, mdtEffectiveDpi,
		uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	if dpiX == 0 {
		dpiX = 96
	}
	scale := float64(dpiX) / 96.0
	physW := int(info.rcMonitor.Right - info.rcMonitor.Left)
	physH := int(info.rcMonitor.Bottom - info.rcMonitor.Top)

	m := MonitorInfo{
		Index:     len(enumResults),
		Left:      info.rcMonitor.Left,
		Top:       info.rcMonitor.Top,
		Width:     int(float64(physW) / scale),
		Height:    int(float64(physH) / scale),
		IsPrimary: info.dwFlags&monitorinfofPrimary != 0,
		Name:      syscall.UTF16ToString(info.szDevice[:]),
	}
	enumResults = append(enumResults, m)
	return 1
}

// GetMonitors returns all connected monitors ordered by index.
func GetMonitors() []MonitorInfo {
	enumResults = nil
	cb := syscall.NewCallback(enumMonitorProc)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return enumResults
}
