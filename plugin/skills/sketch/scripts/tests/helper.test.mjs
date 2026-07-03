// weft-owned test for the vendored visual-companion helper.js.
//
// Guards the indicator-bar XSS hardening (weft-9i3): a choice label whose text
// content contains markup (e.g. a literal "<img onerror=...>") must be rendered
// as inert text, never parsed into live DOM. The pre-hardening code assigned the
// label into `innerHTML` via string concatenation, which re-parsed such markup
// into real elements.
//
// Each case runs helper.js inside a fresh, disposable happy-dom Window (closed
// after use) so the Node test process exits cleanly.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import { Window } from 'happy-dom';

const HELPER_SRC = readFileSync(
  fileURLToPath(new URL('../visual-companion/helper.js', import.meta.url)),
  'utf8',
);

// Render the indicator by loading helper.js into a window, then clicking a
// selected choice whose <h3> label text is `labelText`. Returns a plain snapshot
// of the resulting #indicator-text node and closes the window.
async function renderIndicator(labelText, { selectedCount = 1 } = {}) {
  const window = new Window({ url: 'http://localhost/' });
  // helper.js opens a WebSocket on load; stub it so no real network occurs.
  window.WebSocket = class {
    constructor() { this.readyState = 0; }
    send() {}
    close() {}
  };
  window.WebSocket.OPEN = 1;
  const { document } = window;

  document.body.innerHTML = `
    <div id="indicator-text">initial</div>
    <div class="options">
      <div class="option selected" data-choice="a"><h3></h3></div>
      <div class="option" data-choice="b"><h3>other</h3></div>
    </div>`;

  // Execute the IIFE with the window as global scope: registers the document
  // click listener + window.toggleSelect.
  vm.createContext(window);
  vm.runInContext(HELPER_SRC, window);

  const options = document.querySelectorAll('.option');
  // Set via textContent so the markup is the element's *text* — exactly the path
  // helper.js reads (`h3.textContent.trim()`).
  options[0].querySelector('h3').textContent = labelText;
  if (selectedCount > 1) options[1].classList.add('selected');

  options[0].dispatchEvent(new window.Event('click', { bubbles: true }));
  await window.happyDOM.waitUntilComplete(); // flush the deferred (setTimeout 0) update

  const indicator = document.getElementById('indicator-text');
  const snapshot = {
    text: indicator.textContent,
    hasImg: indicator.querySelector('img') !== null,
    spanTag: indicator.querySelector('.selected-text')?.tagName,
  };
  await window.happyDOM.close();
  return snapshot;
}

test('malicious label text is not parsed into DOM (XSS hardened)', async () => {
  const payload = '<img src=x onerror="window.__xssFired = true">';
  const { text, hasImg } = await renderIndicator(payload);

  assert.equal(hasImg, false, 'label markup must not become a live <img> element');
  assert.ok(text.includes('<img'), 'label must survive as literal text');
  assert.ok(text.includes('selected'), 'indicator keeps its "… selected" copy');
});

test('benign single selection renders the label text', async () => {
  const { text, spanTag } = await renderIndicator('Bold & Minimal');
  assert.ok(text.includes('Bold & Minimal selected'));
  assert.equal(spanTag, 'SPAN', 'keeps the styled .selected-text span');
});

test('multi-selection renders the count', async () => {
  const { text, spanTag } = await renderIndicator('ignored', { selectedCount: 2 });
  assert.ok(text.includes('2 selected'));
  assert.equal(spanTag, 'SPAN');
});
