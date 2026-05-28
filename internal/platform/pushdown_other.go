//go:build !darwin

package platform

// PushdownEnable is a no-op on non-darwin platforms.
func PushdownEnable(mon MonitorInfo, barHeight int) error {
	return nil
}

// PushdownDisable is a no-op on non-darwin platforms.
func PushdownDisable() {
}

// PushdownReconfigure is a no-op on non-darwin platforms.
func PushdownReconfigure(mon MonitorInfo, barHeight int) {
}

// AXTrusted always returns true on non-darwin platforms as accessibility
// permission is not required.
func AXTrusted() bool {
	return true
}

// AXRequestTrust always returns true on non-darwin platforms.
func AXRequestTrust() bool {
	return true
}

// GetPushdownStats returns empty diagnostics on non-darwin platforms.
func GetPushdownStats() PushdownStats {
	return PushdownStats{
		Enabled:           false,
		Trusted:           true,
		ObservedApps:      0,
		PushesThisSession: 0,
		LastError:         "",
	}
}
