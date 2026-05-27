package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type AccountConfig struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type HotkeyConfig struct {
	CycleMonitor       string `json:"cycleMonitor"`
	ToggleClickThrough string `json:"toggleClickThrough"`
}

type Config struct {
	Monitor          int             `json:"monitor"`
	Theme            string          `json:"theme"`
	Opacity          float64         `json:"opacity"`
	RefreshSeconds   int             `json:"refreshSeconds"`
	WeeklyMsgLimit   int64           `json:"weeklyMsgLimit"` // 0 = show raw count only
	BillingResetDay  int             `json:"billingResetDay"`
	BarHeight        int             `json:"barHeight"`
	ActiveAccount    int             `json:"activeAccount"`
	Accounts         []AccountConfig `json:"accounts"`
	Hotkeys          HotkeyConfig    `json:"hotkeys"`
	StartWithWindows bool            `json:"startWithWindows"`
	ClickThrough     bool            `json:"clickThrough"`
	AppBarMode       bool            `json:"appBarMode"` // push apps down (AppBar API)
}

func AppDataDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, "ClaudePanel")
		}
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "Library", "Application Support", "ClaudePanel")
		}
	default:
		if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
			return filepath.Join(v, "ClaudePanel")
		}
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".config", "ClaudePanel")
		}
	}
	return filepath.Join(os.TempDir(), "ClaudePanel")
}

func configPath() string {
	return filepath.Join(AppDataDir(), "config.json")
}

func Defaults() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return Config{
		Monitor:         0,
		Theme:           "terminal-green",
		Opacity:         0.92,
		RefreshSeconds:  15,
		WeeklyMsgLimit:  0, // 0 = show raw count; set e.g. 150000 for Max plan
		BillingResetDay: 1,
		BarHeight:       28,
		ActiveAccount:   0,
		AppBarMode:      true,
		Accounts: []AccountConfig{
			{Name: "main", Path: filepath.Join(home, ".claude")},
			{Name: "alt", Path: filepath.Join(home, ".claude-alt")},
		},
		Hotkeys: HotkeyConfig{
			CycleMonitor:       "Ctrl+Alt+M",
			ToggleClickThrough: "Ctrl+Alt+T",
		},
		StartWithWindows: false,
		ClickThrough:     false,
	}
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		cfg := Defaults()
		return &cfg, nil
	}
	if err != nil {
		return nil, err
	}
	// Unmarshal over defaults so new fields get zero values
	cfg := Defaults()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir := AppDataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: temp file + rename
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}
