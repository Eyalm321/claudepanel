import './style.css';
import {
  GetBarData, GetConfig, SetActiveAccount,
  GetMonitors, SetMonitor, ToggleClickThrough, GetVersion
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

const BAR_CHARS = 9;

// Format message count: 90543 → "90.5K", 1234 → "1.2K", 150 → "150"
function fmtMsgs(n) {
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
  return String(n);
}

function renderProgress(pct) {
  const filled = Math.min(BAR_CHARS, Math.round(pct * BAR_CHARS));
  const empty  = BAR_CHARS - filled;
  let char = '░';
  if (pct >= 0.25 && pct < 0.55) {
    char = '▒';
  } else if (pct >= 0.55 && pct < 0.85) {
    char = '▓';
  } else if (pct >= 0.85) {
    char = '█';
  }
  return { fill: char.repeat(filled), empty: '·'.repeat(empty) };
}

// State
let cfg        = null;
let monitors   = [];
let refreshId  = null;

async function refresh() {
  try {
    const data = await GetBarData();
    if (!data) return;

    // Account + subscription
    document.getElementById('val-acct').textContent =
      (data.accountName || '---').toUpperCase();
    const sub = data.subscriptionType || '';
    document.getElementById('val-sub').textContent  = sub ? `[${sub}]` : '';

    // Weekly Usage/Messages this billing period
    const msgs = data.periodMessages || 0;
    const limit = data.periodMsgLimit || 0;
    const lblMsgs = document.querySelector('#seg-msgs .lbl');

    if (limit > 0) {
      if (lblMsgs) lblMsgs.textContent = 'WEEKLY:';
      const pct = data.periodPercent || 0;
      document.getElementById('val-msgs').textContent = (pct * 100).toFixed(0) + '%';
    } else {
      if (lblMsgs) lblMsgs.textContent = 'MSGS:';
      document.getElementById('val-msgs').textContent = fmtMsgs(msgs);
    }

    // Progress bar — only when a limit is configured
    const progWrap = document.getElementById('prog-wrap');
    if (limit > 0) {
      const p = renderProgress(data.periodPercent || 0);
      document.getElementById('prog-fill').textContent  = p.fill;
      document.getElementById('prog-empty').textContent = p.empty;
      progWrap.style.display = '';
    } else {
      progWrap.style.display = 'none';
    }

    // Dynamic warning classes
    const pct = data.periodPercent || 0;
    const warnMed = data.periodMsgLimit > 0 && pct >= 0.85 && pct < 0.95;
    const warnHigh = data.limitExceeded || (data.periodMsgLimit > 0 && pct >= 0.95);
    document.getElementById('seg-msgs').classList.toggle('warn-medium', warnMed);
    document.getElementById('seg-msgs').classList.toggle('warn-high', warnHigh);

    // 5-hour rolling usage and reset
    const sepHourly = document.getElementById('sep-hourly');
    const segHourly = document.getElementById('seg-hourly');
    const sepHourlyReset = document.getElementById('sep-hourly-reset');
    const segHourlyReset = document.getElementById('seg-hourly-reset');
    if (data.hourlyPercent >= 0) {
      document.getElementById('val-hourly').textContent = (data.hourlyPercent * 100).toFixed(0) + '%';
      
      const p = renderProgress(data.hourlyPercent || 0);
      document.getElementById('prog-fill-hourly').textContent  = p.fill;
      document.getElementById('prog-empty-hourly').textContent = p.empty;
      
      document.getElementById('val-hourly-reset').textContent = data.hourlyResetIn || '---';
      
      if (sepHourly) sepHourly.style.display = '';
      if (segHourly) segHourly.style.display = '';
      if (sepHourlyReset) sepHourlyReset.style.display = '';
      if (segHourlyReset) segHourlyReset.style.display = '';
      
      // Dynamic hourly warnings
      const hpct = data.hourlyPercent || 0;
      const hwarnMed = hpct >= 0.85 && hpct < 0.95;
      const hwarnHigh = hpct >= 0.95;
      segHourly.classList.toggle('warn-medium', hwarnMed);
      segHourly.classList.toggle('warn-high', hwarnHigh);
    } else {
      if (sepHourly) sepHourly.style.display = 'none';
      if (segHourly) segHourly.style.display = 'none';
      if (sepHourlyReset) sepHourlyReset.style.display = 'none';
      if (segHourlyReset) segHourlyReset.style.display = 'none';
    }

    // Reset countdown
    document.getElementById('val-reset').textContent = data.resetIn || '---';

    // Model
    document.getElementById('val-model').textContent = data.primaryModel || '---';

    // Status
    const status = (data.status || 'OFFLINE').toLowerCase();
    document.getElementById('val-status').textContent = (data.status || 'OFFLINE');
    const segSt = document.getElementById('seg-status');
    segSt.className = 'seg ' + status;

  } catch (err) {
    console.error('refresh error:', err);
  }
}

async function updateMonitorDisplay() {
  try {
    cfg      = await GetConfig();
    monitors = await GetMonitors();
    document.getElementById('val-mon').textContent =
      String((cfg.monitor || 0) + 1);
  } catch (e) { /* ignore */ }
}

async function init() {
  try {
    initTheme();
    cfg      = await GetConfig();
    monitors = await GetMonitors();

    // Hide account cycling arrows if there is only one account configured
    const accounts = (cfg && cfg.accounts) || [];
    if (accounts.length < 2) {
      document.getElementById('btn-acct-prev').style.display = 'none';
      document.getElementById('btn-acct-next').style.display = 'none';
      document.getElementById('val-acct').style.cursor = 'default';
    } else {
      document.getElementById('btn-acct-prev').style.display = '';
      document.getElementById('btn-acct-next').style.display = '';
      document.getElementById('val-acct').style.cursor = 'pointer';
    }

    // Hide monitor cycling arrows if there is only one monitor detected
    const totalMonitors = monitors.length;
    if (totalMonitors < 2) {
      document.getElementById('btn-mon-prev').style.display = 'none';
      document.getElementById('btn-mon-next').style.display = 'none';
    } else {
      document.getElementById('btn-mon-prev').style.display = '';
      document.getElementById('btn-mon-next').style.display = '';
    }

    const intervalMs = ((cfg && cfg.refreshSeconds) || 15) * 1000;
    await refresh();
    await updateMonitorDisplay();

    refreshId = setInterval(refresh, intervalMs);

    EventsOn('config:changed',  refresh);
    EventsOn('account:changed', refresh);
    EventsOn('monitor:changed', updateMonitorDisplay);

  } catch (err) {
    console.error('init error:', err);
  }
}

// ── Account cycling ──────────────────────────────────────────────────────────

async function cycleAccount(dir) {
  try {
    cfg = await GetConfig();
    const total = (cfg.accounts || []).length;
    if (total < 2) return;
    const next = ((cfg.activeAccount || 0) + dir + total) % total;
    await SetActiveAccount(next);
    cfg.activeAccount = next;
    await refresh();
  } catch (e) { console.error(e); }
}

document.getElementById('btn-acct-prev').addEventListener('click', () => cycleAccount(-1));
document.getElementById('btn-acct-next').addEventListener('click', () => cycleAccount(+1));

// Also allow clicking the account name itself to cycle forward if multiple are configured
document.getElementById('val-acct').addEventListener('click', () => cycleAccount(+1));

// ── Monitor cycling ──────────────────────────────────────────────────────────

async function cycleMon(dir) {
  try {
    cfg      = await GetConfig();
    monitors = await GetMonitors();
    const total = monitors.length;
    if (total < 2) return;
    const next = ((cfg.monitor || 0) + dir + total) % total;
    await SetMonitor(next);
    cfg.monitor = next;
    document.getElementById('val-mon').textContent = String(next + 1);
  } catch (e) { console.error(e); }
}

document.getElementById('btn-mon-prev').addEventListener('click', () => cycleMon(-1));
document.getElementById('btn-mon-next').addEventListener('click', () => cycleMon(+1));

// ── Theme cycling ───────────────────────────────────────────────────────────
const THEMES = ['CLAUDE', 'FALLOUT', 'AMBER', 'MATRIX', 'DRACULA'];
let activeThemeIdx = 0;

function applyTheme(idx) {
  const bar = document.getElementById('bar');
  // Remove old theme classes
  THEMES.forEach(t => bar.classList.remove(`theme-${t.toLowerCase()}`));
  
  const themeName = THEMES[idx];
  bar.classList.add(`theme-${themeName.toLowerCase()}`);
  document.getElementById('val-theme').textContent = themeName;
  localStorage.setItem('claudebar-theme', themeName);
}

function cycleTheme(dir) {
  activeThemeIdx = (activeThemeIdx + dir + THEMES.length) % THEMES.length;
  applyTheme(activeThemeIdx);
}

// Set up listeners for theme cycler
document.getElementById('btn-theme-prev').addEventListener('click', () => cycleTheme(-1));
document.getElementById('btn-theme-next').addEventListener('click', () => cycleTheme(+1));
document.getElementById('val-theme').addEventListener('click', () => cycleTheme(+1));

function initTheme() {
  const savedTheme = localStorage.getItem('claudebar-theme') || 'CLAUDE';
  let idx = THEMES.indexOf(savedTheme);
  if (idx === -1) idx = 0;
  activeThemeIdx = idx;
  applyTheme(idx);
}

// ── Boot ─────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);
