package tray

import (
	"fmt"

	"github.com/getlantern/systray"
)

// Event types emitted by the tray to the app.
type EventType int

const (
	EventSetAccount EventType = iota // Index int
	EventSetMonitor                  // Index int
	EventToggleStartup
	EventManageAccounts
	EventQuit
)

// Event carries a tray menu action back to the App.
type Event struct {
	Type  EventType
	Index int // used by EventSetAccount and EventSetMonitor
}

// Manager owns the tray lifecycle.
type Manager struct {
	events chan Event

	// updated at runtime
	accountItems []*systray.MenuItem
	monitorItems []*systray.MenuItem
	startupItem  *systray.MenuItem
}

func New() *Manager {
	return &Manager{events: make(chan Event, 8)}
}

// Events returns the channel the app should read from.
func (m *Manager) Events() <-chan Event { return m.events }

// Run starts the systray blocking loop. Call in a dedicated goroutine.
func (m *Manager) Run(iconBytes []byte, version string, accountNames []string, numMonitors int) {
	systray.Run(func() { m.onReady(iconBytes, version, accountNames, numMonitors) }, m.onExit)
}

// Quit tears down the tray icon.
func (m *Manager) Quit() { systray.Quit() }

// SetAccountChecked updates the checkmark on the account submenu.
func (m *Manager) SetAccountChecked(index int) {
	for i, item := range m.accountItems {
		if i == index {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}

// SetMonitorChecked updates the checkmark on the monitor submenu.
func (m *Manager) SetMonitorChecked(index int) {
	for i, item := range m.monitorItems {
		if i == index {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}

// SetStartup updates the startup menu item label.
func (m *Manager) SetStartup(enabled bool) {
	if m.startupItem == nil {
		return
	}
	if enabled {
		m.startupItem.SetTitle("Start on login: ON")
		m.startupItem.Check()
	} else {
		m.startupItem.SetTitle("Start on login: OFF")
		m.startupItem.Uncheck()
	}
}

func (m *Manager) onReady(iconBytes []byte, version string, accountNames []string, numMonitors int) {
	systray.SetIcon(iconBytes)
	systray.SetTooltip("Claude Panel")

	// Title (disabled)
	title := systray.AddMenuItem(fmt.Sprintf("Claude Panel %s", version), "")
	title.Disable()
	systray.AddSeparator()

	// Account items
	for i, name := range accountNames {
		item := systray.AddMenuItem(fmt.Sprintf("Account: %s", name), "")
		if i == 0 {
			item.Check()
		}
		idx := i
		m.accountItems = append(m.accountItems, item)
		go func(mi *systray.MenuItem, index int) {
			for range mi.ClickedCh {
				m.events <- Event{Type: EventSetAccount, Index: index}
			}
		}(item, idx)
	}
	systray.AddSeparator()

	// Monitor items
	for i := 0; i < numMonitors; i++ {
		label := fmt.Sprintf("Monitor %d", i+1)
		item := systray.AddMenuItem(label, "")
		if i == 0 {
			item.Check()
		}
		idx := i
		m.monitorItems = append(m.monitorItems, item)
		go func(mi *systray.MenuItem, index int) {
			for range mi.ClickedCh {
				m.events <- Event{Type: EventSetMonitor, Index: index}
			}
		}(item, idx)
	}
	systray.AddSeparator()

	// Start-on-login toggle
	m.startupItem = systray.AddMenuItem("Start on login: OFF", "Launch on login")
	go func() {
		for range m.startupItem.ClickedCh {
			m.events <- Event{Type: EventToggleStartup}
		}
	}()

	// Configure Accounts item
	manageAccts := systray.AddMenuItem("Configure Accounts...", "Add, edit, or delete Claude accounts")
	go func() {
		for range manageAccts.ClickedCh {
			m.events <- Event{Type: EventManageAccounts}
		}
	}()

	systray.AddSeparator()

	// Quit
	quit := systray.AddMenuItem("Quit", "Exit Claude Panel")
	go func() {
		<-quit.ClickedCh
		m.events <- Event{Type: EventQuit}
	}()
}

func (m *Manager) onExit() {}
