#!/usr/bin/env node
/**
 * Headless popup smoke test for the local-server version row.
 *
 * The real popup talks to chrome.storage / chrome.runtime; this harness copies
 * the extension into a temp directory, injects a small mock before i18n.js and
 * loads the page in headless Chrome. It is intentionally dependency-free: the
 * only requirement is a `google-chrome` (or `chromium`) binary.
 */
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const extensionDir = join(root, 'extension');
const workRoot = join(root, '.gotmp', 'popup-test');
const messages = JSON.parse(readFileSync(join(extensionDir, '_locales', 'en', 'messages.json'), 'utf8'));
const manifest = JSON.parse(readFileSync(join(extensionDir, 'manifest.json'), 'utf8'));
const currentVersion = manifest.version;

function findChrome() {
  for (const name of [process.env.CHROME, 'google-chrome', 'chromium', 'chromium-browser']) {
    if (!name) continue;
    try {
      execFileSync(name, ['--version'], { stdio: 'ignore' });
      return name;
    } catch {
      // Try the next binary.
    }
  }
  throw new Error('Chrome/Chromium not found; set CHROME to the binary path');
}

const chrome = findChrome();
const cases = [
  { name: 'local-outdated', mode: 'local', serverVersion: '1.1.0', expectRow: true, expectLink: true },
  { name: 'local-current', mode: 'local', serverVersion: currentVersion, expectRow: true, expectLink: false },
  { name: 'cloud', mode: 'cloud', serverVersion: '1.3.2', expectRow: false, expectLink: false },
];

function mockScript(testCase) {
  return `
(function () {
  const messages = ${JSON.stringify(messages)};
  const state = {
    hn_reader_mode: ${JSON.stringify(testCase.mode)},
    hn_reader_server_url: 'http://localhost:8080'
  };
  const extDir = 'file://' + ${JSON.stringify(join(workRoot, testCase.name, 'extension'))};

  function getMessage(key, subs) {
    const entry = messages[key];
    if (!entry) return '';
    let msg = entry.message;
    for (const [name, spec] of Object.entries(entry.placeholders || {})) {
      const index = Number(String(spec.content).replace('$', '')) - 1;
      msg = msg.split('$' + name + '$').join(subs && subs[index] != null ? String(subs[index]) : '');
    }
    return msg;
  }

  function sendMessage(request, callback) {
    setTimeout(() => {
      let response = { ok: false, error: 'unexpected ' + request.action };
      switch (request.action) {
        case 'probeConnection':
          response = {
            ok: true,
            data: {
              connected: true,
              info: { status: 'ok', version: ${JSON.stringify(testCase.serverVersion)} },
              versionOk: true
            }
          };
          break;
        case 'getRecommendations':
          response = { ok: true, data: { recommendations: [] } };
          break;
        case 'apiRequest':
          response = { ok: true, data: { plan: 'free', quota_used: 0, quota_limit: 50, quota_reset_at: null } };
          break;
        case 'getStats':
          response = { ok: true, data: { total_signals: 0 } };
          break;
        case 'getLocaleMessages':
          response = { ok: true, data: messages };
          break;
      }
      callback(response);
    }, 0);
  }

  window.chrome = {
    i18n: {
      getUILanguage: () => 'en-US',
      getMessage: (key, subs) => getMessage(key, subs)
    },
    storage: {
      local: {
        get: (keys) => {
          const list = Array.isArray(keys) ? keys : [keys];
          const out = {};
          for (const key of list) {
            if (key in state) out[key] = state[key];
          }
          return Promise.resolve(out);
        },
        set: (values) => { Object.assign(state, values); return Promise.resolve(); },
        remove: (keys) => {
          for (const key of (Array.isArray(keys) ? keys : [keys])) delete state[key];
          return Promise.resolve();
        }
      }
    },
    runtime: {
      getManifest: () => ({ version: ${JSON.stringify(currentVersion)} }),
      getURL: (path) => extDir + '/' + path,
      sendMessage,
      lastError: undefined,
      onMessage: { addListener: () => {} },
      onInstalled: { addListener: () => {} }
    },
    tabs: { create: () => {} }
  };
})();
`;
}

function runCase(testCase) {
  const dir = join(workRoot, testCase.name);
  const ext = join(dir, 'extension');
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  cpSync(extensionDir, ext, { recursive: true });

  const mock = `
<script>window.__TEST_CASE__ = ${JSON.stringify(testCase.name)};</script>
<script src="mock-chrome.js"></script>
`;
  writeFileSync(join(ext, 'mock-chrome.js'), mockScript(testCase));

  let html = readFileSync(join(ext, 'popup.html'), 'utf8');
  html = html.replace('<script src="i18n.js"></script>', mock + '<script src="i18n.js"></script>');
  html = html.replace('</body>', `
<script>
setTimeout(() => {
  const row = document.getElementById('localVersionRow');
  const link = document.getElementById('localUpdateLink');
  const payload = {
    hidden: !row || row.hidden,
    text: document.getElementById('localVersionText')?.textContent || '',
    linkHidden: !link || link.hidden,
    linkText: link?.textContent || ''
  };
  document.body.setAttribute('data-test-result', JSON.stringify(payload));
}, 2000);
</script>
</body>`);
  writeFileSync(join(ext, 'popup.html'), html);

  const userData = join(dir, 'chrome-profile');
  const output = execFileSync(chrome, [
    '--headless=new',
    '--disable-gpu',
    '--no-sandbox',
    '--allow-file-access-from-files',
    '--user-data-dir=' + userData,
    '--virtual-time-budget=5000',
    '--dump-dom',
    'file://' + join(ext, 'popup.html'),
  ], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });

  const match = output.match(/data-test-result="([^"]*)"/);
  if (!match) {
    throw new Error(`${testCase.name}: test result attribute missing`);
  }
  const result = JSON.parse(match[1]
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>'));
  return result;
}

let failed = 0;
for (const testCase of cases) {
  const result = runCase(testCase);
  const rowOk = result.hidden === !testCase.expectRow;
  const textOk = testCase.expectRow ? result.text === `Local server v${testCase.serverVersion}` : true;
  const linkOk = result.linkHidden === !testCase.expectLink
    && (!testCase.expectLink || result.linkText === `v${currentVersion} available — download`);
  const ok = rowOk && textOk && linkOk;
  if (!ok) {
    failed++;
    console.error(`FAIL ${testCase.name}: ${JSON.stringify(result)}`);
  } else {
    console.log(`ok ${testCase.name}: ${JSON.stringify(result)}`);
  }
}

if (failed > 0) {
  process.exit(1);
}
console.log('popup version row checks passed');
