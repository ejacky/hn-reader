document.addEventListener('DOMContentLoaded', () => {
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
      openHnBtn.textContent = '去 Hacker News 试试';
      openHnBtn.disabled = false;
      retryBtn.style.display = 'none';
      statusEl.textContent = '已准备就绪';
      statusEl.className = 'success';
      return;
    }

    if (failed) {
      openHnBtn.textContent = '去 Hacker News 试试';
      openHnBtn.disabled = true;
      retryBtn.style.display = 'block';
      statusEl.textContent = '无法自动创建账户，请检查网络后重试';
      statusEl.className = 'error';
      return;
    }

    // 还在等待中
    openHnBtn.textContent = '正在准备中...';
    openHnBtn.disabled = true;
    retryBtn.style.display = 'none';
    statusEl.textContent = '正在为你准备云服务...';
    statusEl.className = '';

    // 稍后再检查一次
    setTimeout(checkStatus, 1500);
  }

  openHnBtn.addEventListener('click', () => {
    chrome.tabs.create({ url: 'https://news.ycombinator.com' });
    window.close();
  });

  retryBtn.addEventListener('click', async () => {
    statusEl.textContent = '正在重试...';
    statusEl.className = '';
    retryBtn.style.display = 'none';
    const ok = await ensureToken();
    if (ok) {
      await checkStatus();
    } else {
      statusEl.textContent = '重试失败，请检查网络';
      statusEl.className = 'error';
      retryBtn.style.display = 'block';
    }
  });

  checkStatus();
});
