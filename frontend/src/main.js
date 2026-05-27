import './style.css';
import {
  GetBarData, GetConfig, SetActiveAccount,
  GetMonitors, SetMonitor, ToggleClickThrough, GetVersion,
  SaveConfig, SetPinned, SetEditorOpen, PlatformGOOS
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
      const pct = data.periodPercent || 0;
      // 1. Text blocks
      const p = renderProgress(pct);
      document.getElementById('prog-fill-text').textContent = p.fill;
      document.getElementById('prog-empty-text').textContent = p.empty;
      // 2. Solid outlined bar
      document.getElementById('prog-fill-bar').style.width = Math.min(100, Math.max(0, pct * 100)) + '%';
      
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
      
      const hpct = data.hourlyPercent || 0;
      // 1. Text blocks
      const hp = renderProgress(hpct);
      document.getElementById('prog-fill-hourly-text').textContent = hp.fill;
      document.getElementById('prog-empty-hourly-text').textContent = hp.empty;
      // 2. Solid outlined bar
      document.getElementById('prog-fill-hourly-bar').style.width = Math.min(100, Math.max(0, hpct * 100)) + '%';
      
      document.getElementById('val-hourly-reset').textContent = data.hourlyResetIn || '---';
      
      if (sepHourly) sepHourly.style.display = '';
      if (segHourly) segHourly.style.display = '';
      if (sepHourlyReset) sepHourlyReset.style.display = '';
      if (segHourlyReset) segHourlyReset.style.display = '';
      
      // Dynamic hourly warnings
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
    let displayStatus = data.status || 'IDLE';
    if (displayStatus === 'OFFLINE') displayStatus = 'IDLE';
    
    const status = displayStatus.toLowerCase();
    document.getElementById('val-status').textContent = displayStatus;
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

    pinned = cfg.pinned !== false;
    applyPinUI();

    // Hide the Claude FM segment on macOS — the YouTube iframe player's
    // audio is blocked by WkWebView's autoplay policy and we can't override
    // it without patching Wails (see README "Known limitations").
    try {
      const goos = await PlatformGOOS();
      if (goos === 'darwin') {
        const radio = document.getElementById('seg-radio');
        if (radio) {
          const sep = radio.nextElementSibling;
          radio.style.display = 'none';
          if (sep && sep.classList.contains('sep')) sep.style.display = 'none';
        }
      }
    } catch (e) { /* ignore */ }

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
    EventsOn('show:accounts-editor', openAccountsEditor);
    // Auto-hide slide animation is driven from Go (window position);
    // no JS-side animation state to manage.

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
  localStorage.setItem('claudepanel-theme', themeName);
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
  const savedTheme = localStorage.getItem('claudepanel-theme') || 'CLAUDE';
  let idx = THEMES.indexOf(savedTheme);
  if (idx === -1) idx = 0;
  activeThemeIdx = idx;
  applyTheme(idx);
}

// ── Pin / Unpin (auto-hide) ─────────────────────────────────────────────────
// Auto-hide is driven entirely by a Go-side cursor-position poller — WebView2
// mouseleave is unreliable on a 28-px-tall window, so JS doesn't observe hover
// at all. The poller compares the OS cursor against the bar's screen rect.
let pinned = true;

function applyPinUI() {
  document.getElementById('seg-pin').classList.toggle('pinned', pinned);
}

async function togglePin() {
  pinned = !pinned;
  applyPinUI();
  try {
    await SetPinned(pinned);
  } catch (e) { console.error('SetPinned failed:', e); }
}

document.getElementById('seg-pin').addEventListener('click', togglePin);

// ── Radio Player (Claude FM background audio streaming) ──────────────────────
let ytPlayer = null;
let isYtApiLoaded = false;
let isRadioPlaying = false;
let currentVolume = parseInt(localStorage.getItem('claudepanel-fm-volume') || '100', 10);

function updateVolumeUI() {
  const volEl = document.getElementById('radio-vol');
  if (volEl) {
    volEl.textContent = currentVolume + '%';
  }
}

function setVolume(vol) {
  currentVolume = Math.min(200, Math.max(0, vol));
  localStorage.setItem('claudepanel-fm-volume', currentVolume);
  updateVolumeUI();
  if (ytPlayer && typeof ytPlayer.setVolume === 'function') {
    ytPlayer.setVolume(Math.round(currentVolume / 2));
  }
}

function cycleVolume() {
  let nextVol = currentVolume - 10;
  if (nextVol < 0) {
    if (currentVolume === 0) {
      nextVol = 200;
    } else {
      nextVol = 0;
    }
  }
  setVolume(nextVol);
}

function loadYtIframeApi() {
  return new Promise((resolve) => {
    if (isYtApiLoaded) {
      resolve();
      return;
    }
    const tag = document.createElement('script');
    tag.src = "https://www.youtube.com/iframe_api";
    const firstScriptTag = document.getElementsByTagName('script')[0];
    firstScriptTag.parentNode.insertBefore(tag, firstScriptTag);
    
    window.onYouTubeIframeAPIReady = () => {
      isYtApiLoaded = true;
      resolve();
    };
  });
}

async function toggleRadio() {
  const statusEl = document.getElementById('radio-status');
  const titleEl = document.getElementById('radio-title');
  try {
    if (!ytPlayer) {
      statusEl.textContent = '[LOAD]';
      statusEl.className = 'val loading';
      if (titleEl) {
        titleEl.textContent = 'CLAUDE FM';
        titleEl.classList.remove('marquee');
      }
      await loadYtIframeApi();
      ytPlayer = new YT.Player('yt-player', {
        height: '0',
        width: '0',
        videoId: 'YmQ7jRgf4f0',
        playerVars: {
          'playsinline': 1,
          'controls': 0,
          'disablekb': 1,
          'fs': 0,
          'rel': 0
        },
        events: {
          'onReady': (event) => {
            ytPlayer.setVolume(Math.round(currentVolume / 2));
            event.target.playVideo();
            isRadioPlaying = true;
            statusEl.textContent = '[ON]';
            statusEl.className = 'val playing';
            if (titleEl) {
              titleEl.textContent = 'NOW PLAYING CLAUDE FM · NOW PLAYING CLAUDE FM · ';
              titleEl.classList.add('marquee');
            }
          },
          'onStateChange': (event) => {
            if (event.data === 1) { // YT.PlayerState.PLAYING
              isRadioPlaying = true;
              statusEl.textContent = '[ON]';
              statusEl.className = 'val playing';
              if (titleEl) {
                titleEl.textContent = 'NOW PLAYING CLAUDE FM · NOW PLAYING CLAUDE FM · ';
                titleEl.classList.add('marquee');
              }
            } else {
              isRadioPlaying = false;
              statusEl.textContent = '[OFF]';
              statusEl.className = 'val';
              if (titleEl) {
                titleEl.textContent = 'CLAUDE FM';
                titleEl.classList.remove('marquee');
              }
            }
          }
        }
      });
      return;
    }

    if (isRadioPlaying) {
      ytPlayer.pauseVideo();
    } else {
      ytPlayer.playVideo();
    }
  } catch (err) {
    console.error('Radio error:', err);
    statusEl.textContent = '[ERR]';
    statusEl.className = 'val';
    if (titleEl) {
      titleEl.textContent = 'CLAUDE FM';
      titleEl.classList.remove('marquee');
    }
  }
}

// Set up listeners
const radioSeg = document.getElementById('seg-radio');
radioSeg.addEventListener('click', (e) => {
  if (e.target.id === 'radio-vol' || e.target.id === 'radio-vol-lbl') {
    cycleVolume();
    return;
  }
  toggleRadio();
});

radioSeg.addEventListener('wheel', (e) => {
  e.preventDefault();
  const diff = e.deltaY < 0 ? 5 : -5;
  setVolume(currentVolume + diff);
}, { passive: false });

// Initialize volume display on load
updateVolumeUI();

// ── Accounts Editor Controller ───────────────────────────────────────────────
let editorConfig = null;
let editorFormMode = "edit"; // "edit" or "add"

async function openAccountsEditor() {
  try {
    editorConfig = await GetConfig();

    // Hide standard bar segments
    document.getElementById('bar-main-contents').style.display = 'none';

    // Populate select dropdown
    renderEditorSelect();

    // Reset inputs & hide the form segment
    hideEditorForm();

    // Show the editor panel
    document.getElementById('bar-editor-contents').style.display = 'flex';

    // Tell the Go-side hover-watcher to keep the bar expanded (and force-expand
    // it immediately, since the editor is opened from the tray with the cursor
    // off-bar).
    SetEditorOpen(true);
  } catch (err) {
    console.error('Failed to open accounts editor:', err);
  }
}

function renderEditorSelect() {
  const select = document.getElementById('editor-acct-select');
  if (!select) return;
  select.innerHTML = '';
  
  const accounts = editorConfig.accounts || [];
  accounts.forEach((acc, index) => {
    const opt = document.createElement('option');
    opt.value = index;
    opt.textContent = `${acc.name} (${acc.path})`;
    select.appendChild(opt);
  });
  
  // Select active one by default
  const activeIdx = editorConfig.activeAccount || 0;
  if (activeIdx < accounts.length) {
    select.value = activeIdx;
  }
}

function showEditorForm(mode) {
  editorFormMode = mode;
  const form = document.getElementById('editor-form');
  const listControls = document.getElementById('editor-list-controls');
  
  if (form) form.style.display = 'flex';
  if (listControls) listControls.style.display = 'none';
  
  const nameInput = document.getElementById('input-acct-name');
  const pathInput = document.getElementById('input-acct-path');
  
  if (mode === 'add') {
    if (nameInput) nameInput.value = '';
    if (pathInput) pathInput.value = '';
    if (nameInput) nameInput.focus();
  } else {
    // edit mode, fill with currently selected
    const select = document.getElementById('editor-acct-select');
    const idx = select ? parseInt(select.value, 10) : -1;
    const accounts = editorConfig.accounts || [];
    if (idx >= 0 && idx < accounts.length) {
      if (nameInput) nameInput.value = accounts[idx].name;
      if (pathInput) pathInput.value = accounts[idx].path;
      if (nameInput) nameInput.focus();
    }
  }
}

function hideEditorForm() {
  const form = document.getElementById('editor-form');
  const listControls = document.getElementById('editor-list-controls');
  if (form) form.style.display = 'none';
  if (listControls) listControls.style.display = 'flex';
}

async function saveAccount() {
  const nameInput = document.getElementById('input-acct-name');
  const pathInput = document.getElementById('input-acct-path');
  
  const name = nameInput ? nameInput.value.trim() : '';
  const path = pathInput ? pathInput.value.trim() : '';
  
  if (!name || !path) {
    alert("Name and Path cannot be empty!");
    return;
  }
  
  const accounts = editorConfig.accounts || [];
  
  if (editorFormMode === 'add') {
    accounts.push({ name: name, path: path });
    editorConfig.activeAccount = accounts.length - 1;
  } else {
    // edit mode
    const select = document.getElementById('editor-acct-select');
    const idx = select ? parseInt(select.value, 10) : -1;
    if (idx >= 0 && idx < accounts.length) {
      accounts[idx] = { name: name, path: path };
    }
  }
  
  try {
    editorConfig.accounts = accounts;
    await SaveConfig(editorConfig);
    
    renderEditorSelect();
    hideEditorForm();
  } catch (err) {
    console.error('Failed to save config:', err);
    alert('Error saving config: ' + err);
  }
}

async function deleteAccount() {
  const accounts = editorConfig.accounts || [];
  if (accounts.length <= 1) {
    alert("At least one account is required. Cannot delete!");
    return;
  }
  
  const select = document.getElementById('editor-acct-select');
  const idx = select ? parseInt(select.value, 10) : -1;
  if (idx < 0 || idx >= accounts.length) return;
  
  if (!confirm(`Are you sure you want to delete account "${accounts[idx].name}"?`)) {
    return;
  }
  
  accounts.splice(idx, 1);
  
  let activeIdx = editorConfig.activeAccount || 0;
  if (activeIdx >= accounts.length) {
    activeIdx = 0;
  }
  editorConfig.activeAccount = activeIdx;
  
  try {
    editorConfig.accounts = accounts;
    await SaveConfig(editorConfig);
    
    renderEditorSelect();
    hideEditorForm();
  } catch (err) {
    console.error('Failed to delete account:', err);
    alert('Error deleting: ' + err);
  }
}

function closeAccountsEditor() {
  document.getElementById('bar-editor-contents').style.display = 'none';
  document.getElementById('bar-main-contents').style.display = 'flex';
  SetEditorOpen(false);
}

// Bind editor listeners once DOM is ready
document.getElementById('btn-acct-edit').addEventListener('click', () => showEditorForm('edit'));
document.getElementById('btn-acct-add').addEventListener('click', () => showEditorForm('add'));
document.getElementById('btn-acct-del').addEventListener('click', deleteAccount);
document.getElementById('btn-acct-save').addEventListener('click', saveAccount);
document.getElementById('btn-acct-cancel').addEventListener('click', hideEditorForm);
document.getElementById('btn-acct-close').addEventListener('click', closeAccountsEditor);

// ── Boot ─────────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);
