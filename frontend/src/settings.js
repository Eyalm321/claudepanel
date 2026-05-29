// Entry point for the reusable settings popup window (settings.html).
//
// Go opens this window (frameless) from the tray "Configure…" items and tells
// it which panel to show — via the URL on first load, and via the
// "settings:show" event for subsequent opens / panel switches. The shell renders
// the ClaudePanel chrome and mounts the requested panel module.
import './style.css';
import { Events } from '@wailsio/runtime';
import { createShell, applyTheme } from './settings/shell.js';
import accounts from './settings/panel-accounts.js';
import terminals from './settings/panel-terminals.js';
import stations from './settings/panel-stations.js';

const PANELS = {
  [accounts.id]: accounts,
  [terminals.id]: terminals,
  [stations.id]: stations,
};

function panelFromURL() {
  const p = new URLSearchParams(location.search).get('panel');
  return p && PANELS[p] ? p : 'accounts';
}

function boot() {
  applyTheme();
  const shell = createShell(PANELS);

  // First load: the panel is encoded in the URL (the event may fire before this
  // listener is registered).
  shell.show(panelFromURL());

  // Subsequent opens / panel switches on the already-loaded page.
  Events.On('settings:show', (e) => {
    const panel = e && e.data;
    const id = (Array.isArray(panel) ? panel[0] : panel);
    if (id && PANELS[id]) shell.show(id);
  });

  // Track the bar's theme. localStorage is shared across the app's windows, and
  // the `storage` event fires here when the bar changes the theme.
  window.addEventListener('storage', (e) => {
    if (e.key === 'claudepanel-theme') applyTheme();
  });
}

document.addEventListener('DOMContentLoaded', boot);
