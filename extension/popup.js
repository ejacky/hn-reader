const SERVER_URL_KEY = 'hn_reader_server_url';
const LANGUAGE_KEY = 'hn_reader_language';
const MIN_SERVER_VERSION = '1.0.0';
const DOWNLOAD_URL = 'https://github.com/xxx/hn-reader/releases/latest';

document.addEventListener('DOMContentLoaded', async () => {
  const serverInput = document.getElementById('serverUrl');
  const languageSelect = document.getElementById('language');
  const saveBtn = document.getElementById('saveBtn');
  const statusEl = document.getElementById('status');
  const statsSection = document.getElementById('statsSection');
  const statsContent = document.getElementById('statsContent');
  const recSection = document.getElementById('recSection');
  const recContent = document.getElementById('recContent');
  const connIndicator = document.getElementById('connIndicator');
  const connText = document.getElementById('connText');
  const normalSection = document.getElementById('normalSection');
  const guideSection = document.getElementById('guideSection');
  const downloadLink = document.getElementById('downloadLink');
  const manualServerInput = document.getElementById('manualServerUrl');
  const reconnectBtn = document.getElementById('reconnectBtn');

  const result = await chrome.storage.local.get([SERVER_URL_KEY, LANGUAGE_KEY]);
  const serverUrl = result[SERVER_URL_KEY] || 'http://localhost:8080';
  serverInput.value = serverUrl;
  languageSelect.value = result[LANGUAGE_KEY] || 'zh';
  downloadLink.href = DOWNLOAD_URL;

  saveBtn.addEventListener('click', async () => {
    await chrome.storage.local.set({
      [SERVER_URL_KEY]: serverInput.value,
      [LANGUAGE_KEY]: languageSelect.value
    });
    statusEl.textContent = 'Saved!';
    setTimeout(() => statusEl.textContent = '', 2000);
  });

  // Check connection
  await checkAndDisplay(serverUrl);

  reconnectBtn.addEventListener('click', async () => {
    const newUrl = manualServerInput.value || serverUrl;
    if (!newUrl.startsWith('http')) {
      connText.textContent = '地址格式错误，请以 http:// 或 https:// 开头';
      connIndicator.style.color = '#e00';
      return;
    }
    connText.textContent = '检测中...';
    connIndicator.style.color = '#999';
    const ok = await checkAndDisplay(newUrl);
    if (ok) {
      await chrome.storage.local.set({ [SERVER_URL_KEY]: newUrl });
      serverInput.value = newUrl;
    }
  });
});

async function checkAndDisplay(serverUrl) {
  const info = await probeConnection(serverUrl);

  if (info && info.status === 'ok') {
    showConnectedState(info);
    await loadServerData(serverUrl);
    return true;
  } else {
    showDisconnectedState();
    return false;
  }
}

async function probeConnection(url) {
  try {
    const resp = await fetch(`${url}/health`, { signal: AbortSignal.timeout(3000) });
    if (!resp.ok) return null;
    return resp.json();
  } catch {
    return null;
  }
}

function showConnectedState(info) {
  document.getElementById('normalSection').style.display = 'block';
  document.getElementById('guideSection').style.display = 'none';

  const indicator = document.getElementById('connIndicator');
  const text = document.getElementById('connText');
  indicator.style.color = '#008000';
  text.textContent = `服务已连接 v${info.version}`;

  if (!checkVersion(info.version)) {
    text.innerHTML = `⚠ 服务端版本过旧 (v${info.version})，扩展需要 >= v${MIN_SERVER_VERSION} <a href="${DOWNLOAD_URL}" target="_blank">下载最新版本</a>`;
    indicator.style.color = '#e67e00';
  }
}

function showDisconnectedState() {
  document.getElementById('normalSection').style.display = 'none';
  document.getElementById('guideSection').style.display = 'block';

  const indicator = document.getElementById('connIndicator');
  const text = document.getElementById('connText');
  indicator.style.color = '#e00';
  text.textContent = '未检测到本地服务';
}

function checkVersion(serverVersion) {
  const min = MIN_SERVER_VERSION.split('.').map(Number);
  const svr = serverVersion.split('.').map(Number);
  for (let i = 0; i < 3; i++) {
    if (svr[i] > min[i]) return true;
    if (svr[i] < min[i]) return false;
  }
  return true;
}

async function loadServerData(serverUrl) {
  try {
    const statsResp = await fetch(`${serverUrl}/api/signal/stats`);
    if (statsResp.ok) {
      const stats = await statsResp.json();
      document.getElementById('statsSection').style.display = 'block';
      let html = `<p>Total signals: <strong>${stats.total_signals || 0}</strong></p>`;
      if (stats.by_type) {
        html += '<ul>';
        for (const [type, count] of Object.entries(stats.by_type)) {
          html += `<li>${type}: ${count}</li>`;
        }
        html += '</ul>';
      }
      document.getElementById('statsContent').innerHTML = html;
    }
  } catch (e) {
    console.error('stats fetch failed', e);
  }

  try {
    const recResp = await fetch(`${serverUrl}/api/recommendations?limit=5`);
    if (recResp.ok) {
      const data = await recResp.json();
      if (data.recommendations && data.recommendations.length > 0) {
        document.getElementById('recSection').style.display = 'block';
        let html = '<ul>';
        for (const rec of data.recommendations) {
          html += `<li><a href="https://news.ycombinator.com/item?id=${rec.hn_id}" target="_blank">${rec.title}</a><br><small>${rec.reason}</small></li>`;
        }
        html += '</ul>';
        document.getElementById('recContent').innerHTML = html;
      }
    }
  } catch (e) {
    console.error('rec fetch failed', e);
  }
}
