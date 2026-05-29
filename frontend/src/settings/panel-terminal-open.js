// "Open terminal with sublabel" panel for the settings popup. Shown via the
// bar's Shift-click on a terminal entry (Go's OpenTerminalPrompt). The user
// types an optional per-launch sublabel; OPEN (or Enter) launches the terminal
// with it appended to the tab title, then closes the popup. The sublabel is
// per-launch only — nothing is written to config.
import { OpenTerminal } from '../../bindings/claudepanel/app.js';
import { Window } from '@wailsio/runtime';

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}

export default {
  id: 'terminal-open',
  title: 'OPEN TERMINAL',
  async mount(body, data) {
    const index = data && Number.isInteger(data.index) ? data.index : 0;
    const name = (data && data.name) || '';

    body.innerHTML = `
      <div class="panel">
        <div class="panel-hint">Opening <b>${escapeHtml(name)}</b> — add an optional sublabel for this tab.</div>
        <label class="panel-row">
          <span class="panel-label">SUBLABEL</span>
          <input type="text" id="to-sublabel" class="editor-input panel-grow" placeholder="backend" autocomplete="off" spellcheck="false">
        </label>
        <div class="panel-actions">
          <button id="to-open" class="editor-btn editor-btn--accent">OPEN</button>
          <button id="to-cancel" class="editor-btn">CANCEL</button>
        </div>
      </div>`;

    const input = body.querySelector('#to-sublabel');

    const open = async () => {
      try {
        await OpenTerminal(index, input.value.trim());
      } catch (err) {
        console.error('Failed to open terminal:', err);
      }
      Window.Hide();
    };

    body.querySelector('#to-open').addEventListener('click', open);
    body.querySelector('#to-cancel').addEventListener('click', () => Window.Hide());
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); open(); }
    });
    input.focus();
  },
};
