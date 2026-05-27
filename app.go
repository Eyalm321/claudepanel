package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"claudebar/internal/claude"
	"claudebar/internal/config"
	"claudebar/internal/syswin"
	"claudebar/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set via -ldflags "-X main.Version=x.y.z" at build time.
var Version = "dev"

type App struct {
	ctx      context.Context
	cfg      *config.Config
	monitors []syswin.MonitorInfo
	hwnd     uintptr
	trayMgr  *tray.Manager
}

func NewApp() *App {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load error: %v — using defaults", err)
		def := config.Defaults()
		cfg = &def
	}
	return &App{cfg: cfg}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(ctx context.Context) {
	time.Sleep(300 * time.Millisecond)

	hwnd, err := syswin.FindWindowByPID()
	if err != nil {
		log.Printf("HWND lookup failed: %v", err)
	} else {
		a.hwnd = hwnd
		syswin.ApplyBarStyles(hwnd)
	}

	a.monitors = syswin.GetMonitors()
	if a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}

	if a.hwnd != 0 && len(a.monitors) > 0 {
		syswin.DockToMonitor(a.hwnd, a.monitors[a.cfg.Monitor], a.cfg.BarHeight, a.cfg.AppBarMode)
		syswin.SetOpacity(a.hwnd, a.cfg.Opacity)
		if a.cfg.ClickThrough {
			syswin.SetClickThrough(a.hwnd, true)
		}
	}

	go a.runTray()
}

func (a *App) shutdown(ctx context.Context) {
	if a.hwnd != 0 {
		syswin.RemoveAppBar(a.hwnd)
	}
	if a.trayMgr != nil {
		a.trayMgr.Quit()
	}
}

func (a *App) runTray() {
	names := make([]string, len(a.cfg.Accounts))
	for i, acc := range a.cfg.Accounts {
		names[i] = acc.Name
	}
	a.trayMgr = tray.New()
	go a.handleTrayEvents()
	a.trayMgr.Run(trayIconBytes, Version, names, len(a.monitors))
}

func (a *App) handleTrayEvents() {
	for event := range a.trayMgr.Events() {
		switch event.Type {
		case tray.EventSetAccount:
			_ = a.SetActiveAccount(event.Index)
		case tray.EventSetMonitor:
			_ = a.SetMonitor(event.Index)
		case tray.EventToggleClickThrough:
			enabled := a.ToggleClickThrough()
			a.trayMgr.SetClickThrough(enabled)
		case tray.EventToggleStartup:
			a.cfg.StartWithWindows = !a.cfg.StartWithWindows
			exePath, _ := os.Executable()
			_ = config.SetStartWithWindows(a.cfg.StartWithWindows, exePath)
			_ = config.Save(a.cfg)
			a.trayMgr.SetStartup(a.cfg.StartWithWindows)
		case tray.EventQuit:
			a.trayMgr.Quit()
			runtime.Quit(a.ctx)
		}
	}
}

// ── Wails-exported bindings ──────────────────────────────────────────────────

func (a *App) GetBarData() (*claude.BarData, error) {
	if len(a.cfg.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured")
	}
	idx := a.cfg.ActiveAccount
	if idx >= len(a.cfg.Accounts) {
		idx = 0
	}
	acc := a.cfg.Accounts[idx]

	sc, _ := claude.ReadStatsCache(acc.Path)
	creds, _ := claude.ReadCredentials(acc.Path)
	sessions := claude.ReadSessions(acc.Path)
	notifs := claude.ReadNotifications(acc.Path)

	return claude.ComputeBarData(
		acc.Name,
		sc, creds, sessions, notifs,
		a.cfg.WeeklyMsgLimit,
		a.cfg.BillingResetDay,
	), nil
}

func (a *App) GetConfig() config.Config {
	return *a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	prevMonitor := a.cfg.Monitor
	prevAppBar := a.cfg.AppBarMode
	a.cfg = &cfg
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.hwnd != 0 {
		if cfg.Monitor != prevMonitor || cfg.AppBarMode != prevAppBar {
			if prevAppBar {
				syswin.RemoveAppBar(a.hwnd)
			}
			a.monitors = syswin.GetMonitors()
			if cfg.Monitor < len(a.monitors) {
				syswin.DockToMonitor(a.hwnd, a.monitors[cfg.Monitor], cfg.BarHeight, cfg.AppBarMode)
			}
		}
		syswin.SetOpacity(a.hwnd, cfg.Opacity)
	}
	runtime.EventsEmit(a.ctx, "config:changed")
	return nil
}

func (a *App) GetMonitors() []syswin.MonitorInfo {
	a.monitors = syswin.GetMonitors()
	return a.monitors
}

func (a *App) SetActiveAccount(index int) error {
	if index < 0 || index >= len(a.cfg.Accounts) {
		return fmt.Errorf("account index %d out of range", index)
	}
	a.cfg.ActiveAccount = index
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.trayMgr != nil {
		a.trayMgr.SetAccountChecked(index)
	}
	runtime.EventsEmit(a.ctx, "account:changed", index)
	return nil
}

func (a *App) SetMonitor(index int) error {
	a.monitors = syswin.GetMonitors()
	if index < 0 || index >= len(a.monitors) {
		return fmt.Errorf("monitor index %d out of range", index)
	}
	if a.hwnd != 0 && a.cfg.AppBarMode {
		syswin.RemoveAppBar(a.hwnd)
	}
	a.cfg.Monitor = index
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.hwnd != 0 {
		syswin.DockToMonitor(a.hwnd, a.monitors[index], a.cfg.BarHeight, a.cfg.AppBarMode)
	}
	if a.trayMgr != nil {
		a.trayMgr.SetMonitorChecked(index)
	}
	runtime.EventsEmit(a.ctx, "monitor:changed", index)
	return nil
}

func (a *App) ToggleClickThrough() bool {
	a.cfg.ClickThrough = !a.cfg.ClickThrough
	if a.hwnd != 0 {
		syswin.SetClickThrough(a.hwnd, a.cfg.ClickThrough)
	}
	_ = config.Save(a.cfg)
	return a.cfg.ClickThrough
}

func (a *App) SetOpacity(opacity float64) error {
	a.cfg.Opacity = opacity
	if a.hwnd != 0 {
		syswin.SetOpacity(a.hwnd, opacity)
	}
	return config.Save(a.cfg)
}

func (a *App) GetVersion() string {
	return Version
}
