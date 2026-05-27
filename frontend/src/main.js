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
  return { fill: '█'.repeat(filled), empty: '░'.repeat(empty) };
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

    // Messages this billing period
    const msgs = data.periodMessages || 0;
    document.getElementById('val-msgs').textContent = fmtMsgs(msgs);

    // Progress bar — only when a limit is configured
    const progWrap = document.getElementById('prog-wrap');
    if (data.periodMsgLimit > 0) {
      const p = renderProgress(data.periodPercent || 0);
      document.getElementById('prog-fill').textContent  = p.fill;
      document.getElementById('prog-empty').textContent = p.empty;
      progWrap.style.display = '';
    } else {
      progWrap.style.display = 'none';
    }

    // Warning at >90% or limit already exceeded
    const warn = data.limitExceeded || (data.periodMsgLimit > 0 && (data.periodPercent || 0) >= 0.9);
    document.getElementById('seg-msgs').classList.toggle('warn', warn);

    // Reset countdown
    document.getElementById('val-reset').textContent = data.resetIn || '---';

    // Model
    document.getElementById('val-model').textContent = data.primaryModel || '---';

    // Last data day
    const lbl = data.lastDataLabel || '---';
    document.getElementById('lbl-last').textContent = lbl + ':';
    document.getElementById('val-last').textContent =
      data.lastDataMsgs ? fmtMsgs(data.lastDataMsgs) : '0';

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
    cfg      = await GetConfig();
    monitors = await GetMonitors();

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

// Also allow clicking the account name itself to cycle forward
document.getElementById('val-acct').style.cursor = 'pointer';
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

// ── Boot ─────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);
