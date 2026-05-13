const DEFAULT_SERVER = 'http://localhost:8080';

async function getServerUrl() {
  return new Promise((resolve) => {
    chrome.storage.local.get(['hn_reader_server_url'], (result) => {
      resolve(result['hn_reader_server_url'] || DEFAULT_SERVER);
    });
  });
}

async function apiRequest(method, path, body = null) {
  const serverUrl = await getServerUrl();
  const url = `${serverUrl}${path}`;
  const options = {
    method,
    headers: { 'Content-Type': 'application/json' }
  };
  if (body) {
    options.body = JSON.stringify(body);
  }
  const resp = await fetch(url, options);
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`HTTP ${resp.status}: ${text}`);
  }
  return resp.json();
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
          sendResponse({ ok: true, data: await apiRequest('GET', `/api/recommendations?limit=${request.limit || 10}`) });
          break;
        default:
          sendResponse({ ok: false, error: 'Unknown action' });
      }
    } catch (err) {
      sendResponse({ ok: false, error: err.message });
    }
  })();
  return true; // keep channel open for async
});

chrome.runtime.onInstalled.addListener(() => {
  console.log('HN Reader extension installed');
});
