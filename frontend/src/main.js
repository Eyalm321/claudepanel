import './style.css';
import {
  GetBarData, GetConfig, SetActiveAccount,
  GetMonitors, ToggleClickThrough, GetVersion
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

const BAR_CHARS = 10;

function renderProgress(pct) {
  const filled = Math.min(BAR_CHARS, Math.round(pct * BAR_CHARS));
  const empty = BAR_CHARS - filled;
  return {
    fill:  '█'.repeat(filled),
    empty: '░'.repeat(empty),
  };
}

function shortSub(sub) {
  if (!sub) return '';
  const s = sub.toUpperCase();
  if (s === 'MAX') return 'MAX';
  if (s === 'PRO') return 'PRO';
  return s.slice(0, 4);
}

let config = null;
let accountNames = [];
let numMonitors = 1;

async function refresh() {
  try {
    const data = await GetBarData();
    if (!data) return;

    document.getElementById('val-account').textContent =
      (data.accountName || '---').toUpperCase();

    const pct = data.weeklyPercent || 0;
    document.getElementById('val-weekly-pct').textContent =
      Math.round(pct * 100) + '%';

    const p = renderProgress(pct);
    document.getElementById('prog-fill').textContent  = p.fill;
    document.getElementById('prog-empty').textContent = p.empty;

    document.getElementById('val-reset').textContent  = data.resetIn || '---';
    document.getElementById('val-model').textContent  = data.primaryModel || '---';
    document.getElementById('val-today').textContent  = String(data.todayMessages || 0);
    document.getElementById('val-status').textContent = data.status || 'OFFLINE';
    document.getElementById('val-acct-sub').textContent = shortSub(data.subscriptionType);

    // Warning class at >90%
    document.getElementById('seg-weekly').classList.toggle('warn', pct >= 0.9);

    // Status class
    const segSt = document.getElementById('seg-status');
    segSt.className = 'segment ' + (data.status || 'offline').toLowerCase();

  } catch (err) {
    console.error('refresh error:', err);
  }
}

async function init() {
  try {
    config = await GetConfig();
    const monitors = await GetMonitors();
    numMonitors = monitors.length;

    if (config && config.accounts) {
      accountNames = config.accounts.map(a => a.name);
    }

    const intervalMs = ((config && config.refreshSeconds) || 15) * 1000;
    await refresh();
    setInterval(refresh, intervalMs);

    // Wails events from Go
    EventsOn('config:changed', refresh);
    EventsOn('account:changed', refresh);
    EventsOn('monitor:changed', () => {});

  } catch (err) {
    console.error('init error:', err);
  }
}

// Account cycling buttons
document.getElementById('btn-prev').addEventListener('click', async () => {
  try {
    config = await GetConfig();
    const total = (config.accounts || []).length;
    const next = ((config.activeAccount || 0) - 1 + total) % total;
    await SetActiveAccount(next);
    config.activeAccount = next;
    await refresh();
  } catch (e) { console.error(e); }
});

document.getElementById('btn-next').addEventListener('click', async () => {
  try {
    config = await GetConfig();
    const total = (config.accounts || []).length;
    const next = ((config.activeAccount || 0) + 1) % total;
    await SetActiveAccount(next);
    config.activeAccount = next;
    await refresh();
  } catch (e) { console.error(e); }
});

document.addEventListener('DOMContentLoaded', init);
