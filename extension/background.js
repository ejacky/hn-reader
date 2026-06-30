importScripts('config.js');

const MIN_SERVER_VERSION = '1.0.0';

async function createAnonymousAccount() {
  try {
    const cfg = await getReaderConfig();
    const resp = await fetch(`${cfg.baseUrl}/auth/anonymous`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}`);
    }
    const data = await resp.json();
    if (!data.token) {
      throw new Error('no token returned');
    }
    await setReaderConfig({
      mode: 'cloud',
      cloudToken: data.token
    });
    await chrome.storage.local.remove(['hn_reader_cloud_auto_auth_failed']);
    console.log('[HN Reader] anonymous account created');
    return true;
  } catch (err) {
    console.error('[HN Reader] create anonymous account failed:', err);
    await chrome.storage.local.set({ hn_reader_cloud_auto_auth_failed: true });
    return false;
  }
}

async function ensureCloudToken() {
  const cfg = await getReaderConfig();
  if (cfg.mode !== 'cloud') return true;
  if (cfg.token) return true;

  // 已有失败标记，避免频繁重试；popup 打开时可手动触发重试
  const result = await chrome.storage.local.get(['hn_reader_cloud_auto_auth_failed']);
  if (result.hn_reader_cloud_auto_auth_failed) {
    return false;
  }

  return await createAnonymousAccount();
}

async function apiRequest(method, path, body = null) {
  const tokenReady = await ensureCloudToken();
  if (!tokenReady) {
    throw new Error('Cloud account not ready. Please check your network and try again.');
  }
  const cfg = await getReaderConfig();
  const url = `${cfg.baseUrl}${path}`;
  const options = {
    method,
    headers: buildAuthHeaders(cfg)
  };
  if (body) {
    options.body = JSON.stringify(body);
  }
  const resp = await fetch(url, options);
  if (!resp.ok) {
    const text = await resp.text();
    const err = new Error(`HTTP ${resp.status}: ${text}`);
    err.status = resp.status;
    throw err;
  }
  return resp.json();
}

async function probeConnection() {
  const cfg = await getReaderConfig();
  try {
    const headers = buildAuthHeaders(cfg);
    const resp = await fetch(`${cfg.baseUrl}/health`, {
      method: 'GET',
      headers,
      signal: AbortSignal.timeout(3000)
    });
    if (!resp.ok) return null;
    return resp.json();
  } catch {
    return null;
  }
}

function checkVersion(serverVersion) {
  const min = MIN_SERVER_VERSION.split('.').map(Number);
  const svr = (serverVersion || '0.0.0').split('.').map(Number);
  for (let i = 0; i < 3; i++) {
    if (svr[i] > min[i]) return true;
    if (svr[i] < min[i]) return false;
  }
  return true;
}

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  (async () => {
    try {
      switch (request.action) {
        case 'summarizeArticle':
          sendResponse({ ok: true, data: await apiRequest('POST', '/api/summarize/article', request.payload) });
          break;
        case 'summarizeComment':
          sendResponse({ ok: true, data: await apiRequest('POST', '/api/summarize/comment', request.payload) });
          break;
        case 'sendSignal':
          sendResponse({ ok: true, data: await apiRequest('POST', '/api/signal', request.payload) });
          break;
        case 'batchMatch':
          sendResponse({ ok: true, data: await apiRequest('POST', '/api/match/batch', request.payload) });
          break;
        case 'getStats':
          sendResponse({ ok: true, data: await apiRequest('GET', '/api/signal/stats') });
          break;
        case 'getRecommendations':
          sendResponse({ ok: true, data: await apiRequest('GET', `/api/recommendations?limit=${request.payload?.limit || 10}`) });
          break;
        case 'preloadArticles':
          sendResponse({ ok: true, data: await apiRequest('POST', '/api/summarize/article/preload', request.payload) });
          break;
        case 'preloadStatus':
          sendResponse({ ok: true, data: await apiRequest('GET', `/api/summarize/article/preload-status?ids=${request.payload.ids}&lang=${request.payload.lang}`) });
          break;
        case 'getConfig':
          sendResponse({ ok: true, data: await getReaderConfig() });
          break;
        case 'setConfig':
          await setReaderConfig(request.payload);
          sendResponse({ ok: true, data: await getReaderConfig() });
          break;
        case 'ensureCloudToken':
          const ok = await ensureCloudToken();
          sendResponse({ ok: true, data: { success: ok } });
          break;
        case 'apiRequest':
          sendResponse({ ok: true, data: await apiRequest(request.payload.method, request.payload.path, request.payload.body) });
          break;
        case 'probeConnection':
          const info = await probeConnection();
          sendResponse({ ok: true, data: { connected: !!info, info, versionOk: info ? checkVersion(info.version) : false } });
          break;
        default:
          sendResponse({ ok: false, error: 'Unknown action' });
      }
    } catch (err) {
      sendResponse({ ok: false, error: err.message, status: err.status || 0 });
    }
  })();
  return true;
});

chrome.runtime.onInstalled.addListener((details) => {
  console.log('[HN Reader] extension installed', details.reason);
  (async () => {
    const result = await chrome.storage.local.get([CONFIG.KEYS.MODE]);
    if (!result[CONFIG.KEYS.MODE]) {
      // 默认 cloud 模式，并尝试自动创建匿名账户
      await setReaderConfig({ mode: CONFIG.DEFAULTS.MODE });
      await createAnonymousAccount();
    }
    // 始终打开欢迎页做产品介绍
    chrome.tabs.create({ url: chrome.runtime.getURL('welcome.html') });
  })();
});
