package tray

import (
	"fmt"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Controller defines the interface for App callbacks from the tray.
type Controller interface {
	SetActiveAccount(index int) error
	SetMonitor(index int) error
	ToggleStartup()
	ConfigureAccounts()
	Quit()
}

// Manager owns the tray lifecycle.
type Manager struct {
	tray         *application.SystemTray
	menu         *application.Menu
	accountItems []*application.MenuItem
	monitorItems []*application.MenuItem
	startupItem  *application.MenuItem
}

func New() *Manager {
	return &Manager{}
}

// Build creates the native system tray and its menus.
func (m *Manager) Build(
	app *application.App,
	controller Controller,
	iconBytes []byte,
	version string,
	accountNames []string,
	numMonitors int,
	startWithWindows bool,
	activeAccount int,
	activeMonitor int,
) {
	m.tray = app.SystemTray.New()

	if runtime.GOOS == "darwin" {
		m.tray.SetTemplateIcon(iconBytes)
	} else {
		m.tray.SetIcon(iconBytes)
	}

	m.menu = app.NewMenu()
	m.menu.Add(fmt.Sprintf("Claude Panel %s", version)).SetEnabled(false)
	m.menu.AddSeparator()

	// Accounts
	for i, name := range accountNames {
		idx := i
		item := m.menu.AddRadio(fmt.Sprintf("Account: %s", name), i == activeAccount)
		item.OnClick(func(ctx *application.Context) {
			_ = controller.SetActiveAccount(idx)
		})
		m.accountItems = append(m.accountItems, item)
	}
	m.menu.AddSeparator()

	// Monitors
	for i := 0; i < numMonitors; i++ {
		idx := i
		item := m.menu.AddRadio(fmt.Sprintf("Monitor %d", i+1), i == activeMonitor)
		item.OnClick(func(ctx *application.Context) {
			_ = controller.SetMonitor(idx)
		})
		m.monitorItems = append(m.monitorItems, item)
	}
	m.menu.AddSeparator()

	// Start-on-login toggle
	m.startupItem = m.menu.AddCheckbox("Start on login", startWithWindows)
	m.startupItem.OnClick(func(ctx *application.Context) {
		controller.ToggleStartup()
	})

	// Configure Accounts item
	m.menu.Add("Configure Accounts...").OnClick(func(ctx *application.Context) {
		controller.ConfigureAccounts()
	})

	m.menu.AddSeparator()

	// Quit
	m.menu.Add("Quit").OnClick(func(ctx *application.Context) {
		controller.Quit()
	})

	m.tray.SetMenu(m.menu)
}

// Quit is a no-op in Wails v3 as application teardown is managed by the Wails runtime.
func (m *Manager) Quit() {}

// SetAccountChecked updates the checkmark on the account submenu.
func (m *Manager) SetAccountChecked(index int) {
	for i, item := range m.accountItems {
		item.SetChecked(i == index)
	}
}

// SetMonitorChecked updates the checkmark on the monitor submenu.
func (m *Manager) SetMonitorChecked(index int) {
	for i, item := range m.monitorItems {
		item.SetChecked(i == index)
	}
}

// SetStartup updates the startup menu item checked state.
func (m *Manager) SetStartup(enabled bool) {
	if m.startupItem != nil {
		m.startupItem.SetChecked(enabled)
	}
}
