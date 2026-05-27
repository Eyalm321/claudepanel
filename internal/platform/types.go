package platform

// MonitorInfo is the app-facing monitor descriptor, exposed to the frontend.
// Fields are identical across platforms; per-OS implementations populate them
// from their native display APIs.
type MonitorInfo struct {
	Index     int     `json:"index"`
	Left      int32   `json:"left"`      // physical pixels
	Top       int32   `json:"top"`       // physical pixels
	Width     int     `json:"width"`     // logical pixels
	Height    int     `json:"height"`    // logical pixels
	PhysWidth int     `json:"physWidth"` // physical pixels (use for OS-native sizing calls)
	DpiScale  float64 `json:"dpiScale"`  // e.g. 1.25 at 125%
	IsPrimary bool    `json:"isPrimary"`
	Name      string  `json:"name"`
}
