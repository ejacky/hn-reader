document.addEventListener('DOMContentLoaded', async () => {
  await ensureI18n();

  const openHnBtn = document.getElementById('openHnBtn');
  const retryBtn = document.getElementById('retryBtn');
  const statusEl = document.getElementById('status');

  async function ensureToken() {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ action: 'ensureCloudToken', payload: { force: true } }, (resp) => {
        resolve(resp?.ok && resp.data?.success);
      });
    });
  }

  async function checkStatus() {
    const cfg = await getReaderConfig();
    const failedResult = await chrome.storage.local.get(['hn_reader_cloud_auto_auth_failed']);
    const hasToken = !!cfg.token;
    const failed = !!failedResult.hn_reader_cloud_auto_auth_failed;

    if (hasToken) {
      openHnBtn.textContent = t('goToHackerNews');
      openHnBtn.disabled = false;
      retryBtn.style.display = 'none';
      statusEl.textContent = t('ready');
      statusEl.className = 'success';
      return;
    }

    if (failed) {
      openHnBtn.textContent = t('goToHackerNews');
      openHnBtn.disabled = true;
      retryBtn.style.display = 'block';
      statusEl.textContent = t('errAutoAccount');
      statusEl.className = 'error';
      return;
    }

    // 还在等待中
    openHnBtn.textContent = t('preparing');
    openHnBtn.disabled = true;
    retryBtn.style.display = 'none';
    statusEl.textContent = t('preparingCloud');
    statusEl.className = '';

    // 稍后再检查一次
    setTimeout(checkStatus, 1500);
  }

  openHnBtn.addEventListener('click', () => {
    chrome.tabs.create({ url: 'https://news.ycombinator.com' });
    window.close();
  });

  retryBtn.addEventListener('click', async () => {
    statusEl.textContent = t('retrying');
    statusEl.className = '';
    retryBtn.style.display = 'none';
    const ok = await ensureToken();
    if (ok) {
      await checkStatus();
    } else {
      statusEl.textContent = t('errRetry');
      statusEl.className = 'error';
      retryBtn.style.display = 'block';
    }
  });

  checkStatus();
});
