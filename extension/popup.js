const DOWNLOAD_URL = 'https://github.com/ejacky/hn-reader/releases/latest';

document.addEventListener('DOMContentLoaded', async () => {
  const statusEl = document.getElementById('status');

  // Cloud elements
  const cloudSection = document.getElementById('cloudSection');
  const cloudLanguageSelect = document.getElementById('cloudLanguage');
  const saveCloudBtn = document.getElementById('saveCloudBtn');
  const switchToLocalBtn = document.getElementById('switchToLocalBtn');
  const quotaSection = document.getElementById('quotaSection');
  const quotaFill = document.getElementById('quotaFill');
  const quotaText = document.getElementById('quotaText');
  const quotaExhausted = document.getElementById('quotaExhausted');
  const exportDataBtn = document.getElementById('exportDataBtn');
  const deleteAccountBtn = document.getElementById('deleteAccountBtn');
  const cloudAccountStatus = document.getElementById('cloudAccountStatus');

  // Local elements
  const localSection = document.getElementById('localSection');
  const serverUrlInput = document.getElementById('serverUrl');
  const localLanguageSelect = document.getElementById('localLanguage');
  const saveLocalBtn = document.getElementById('saveLocalBtn');
  const switchToCloudBtn = document.getElementById('switchToCloudBtn');
  const localGuide = document.getElementById('localGuide');

  // Connection
  const connIndicator = document.getElementById('connIndicator');
  const connText = document.getElementById('connText');

  // Auto preload toggle
  const autoPreloadToggle = document.getElementById('autoPreloadToggle');
  const autoPreloadResult = await chrome.storage.local.get([CONFIG.KEYS.AUTO_PRELOAD]);
  autoPreloadToggle.checked = !!autoPreloadResult[CONFIG.KEYS.AUTO_PRELOAD];

  autoPreloadToggle.addEventListener('change', async () => {
    await chrome.storage.local.set({ [CONFIG.KEYS.AUTO_PRELOAD]: autoPreloadToggle.checked });
    statusEl.textContent = '已保存';
    statusEl.style.color = '#008000';
    setTimeout(() => statusEl.textContent = '', 2000);
  });

  // Load config
  let cfg = await getReaderConfig();

  function showSection(mode) {
    cloudSection.style.display = 'none';
    localSection.style.display = 'none';

    if (mode === 'cloud') {
      cloudSection.style.display = 'block';
      cloudLanguageSelect.value = cfg.language;
    } else if (mode === 'local') {
      localSection.style.display = 'block';
      serverUrlInput.value = cfg.serverUrl;
      localLanguageSelect.value = cfg.language;
    }
  }

  showSection(cfg.mode);

  switchToLocalBtn.addEventListener('click', async (e) => {
    e.preventDefault();
    await setReaderConfig({ mode: 'local' });
    cfg = await getReaderConfig();
    showSection('local');
    await refreshConnection();
  });

  switchToCloudBtn.addEventListener('click', async (e) => {
    e.preventDefault();
    await setReaderConfig({ mode: 'cloud' });
    cfg = await getReaderConfig();
    showSection('cloud');
    await refreshConnection();
  });

  saveCloudBtn.addEventListener('click', async () => {
    await setReaderConfig({
      language: cloudLanguageSelect.value
    });
    statusEl.textContent = '已保存';
    statusEl.style.color = '#008000';
    setTimeout(() => statusEl.textContent = '', 2000);
  });

  saveLocalBtn.addEventListener('click', async () => {
    const url = serverUrlInput.value.trim() || 'http://localhost:8080';
    if (!url.startsWith('http')) {
      statusEl.textContent = '地址格式错误';
      statusEl.style.color = '#e00';
      return;
    }
    await setReaderConfig({
      mode: 'local',
      serverUrl: url,
      language: localLanguageSelect.value
    });
    cfg = await getReaderConfig();
    statusEl.textContent = '保存中...';
    await refreshConnection();
  });

  exportDataBtn.addEventListener('click', async () => {
    const resp = await sendAction('apiRequest', { method: 'GET', path: '/api/account/export' });
    if (!resp.ok) {
      statusEl.textContent = '导出失败：' + (resp.error || '未知错误');
      statusEl.style.color = '#e00';
      return;
    }
    const blob = new Blob([JSON.stringify(resp.data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'hn-reader-export.json';
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    statusEl.textContent = '已导出';
    statusEl.style.color = '#008000';
  });

  deleteAccountBtn.addEventListener('click', async () => {
    if (!confirm('确定删除云端账户及所有阅读数据？此操作不可恢复。')) return;
    const resp = await sendAction('apiRequest', { method: 'DELETE', path: '/api/account' });
    if (!resp.ok) {
      statusEl.textContent = '删除失败：' + (resp.error || '未知错误');
      statusEl.style.color = '#e00';
      return;
    }
    await chrome.storage.local.remove([CONFIG.KEYS.CLOUD_TOKEN]);
    cloudAccountStatus.textContent = '云端数据已删除';
    quotaSection.style.display = 'none';
    quotaExhausted.style.display = 'none';
    statusEl.textContent = '已删除；下次使用会创建新的匿名账户';
    statusEl.style.color = '#008000';
  });

  async function sendAction(action, payload = {}) {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ action, payload }, (resp) => {
        resolve(resp || { ok: false, error: 'no response' });
      });
    });
  }

  async function refreshConnection() {
    cfg = await getReaderConfig();
    connText.textContent = '检测中...';
    connIndicator.style.color = '#999';
    statusEl.textContent = '';

    const probe = await sendAction('probeConnection');

    if (probe.ok && probe.data.connected) {
      connIndicator.style.color = '#008000';
      const info = probe.data.info || {};
      connText.textContent = cfg.mode === 'cloud' ? '云服务已连接' : '本地服务已连接';
      statusEl.textContent = '连接成功';
      statusEl.style.color = '#008000';
      localGuide.style.display = 'none';

      if (cfg.mode === 'cloud') {
        await loadCloudMeta();
      }
      await loadServerData();
    } else {
      connIndicator.style.color = '#e00';
      connText.textContent = cfg.mode === 'cloud' ? '云服务未连接' : '未检测到本地服务';
      statusEl.textContent = probe.ok ? '无法连接' : (probe.error || '无法连接');
      statusEl.style.color = '#e00';
      if (cfg.mode === 'local') {
        localGuide.style.display = 'block';
      }
      hideDataSections();
    }
  }

  async function loadCloudMeta() {
    const resp = await sendAction('apiRequest', { method: 'GET', path: '/auth/me' });
    if (!resp.ok || !resp.data) {
      cloudAccountStatus.textContent = '匿名账户';
      return;
    }

    const { plan, quota_used, quota_limit, quota_reset_at } = resp.data;
    cloudAccountStatus.textContent = plan === 'free' ? '匿名账户（免费版）' : (plan || '匿名账户');

    quotaSection.style.display = 'block';
    const used = quota_used || 0;
    const limit = quota_limit || 50;
    const pct = Math.min(100, Math.round((used / limit) * 100));
    quotaFill.style.width = `${pct}%`;
    quotaFill.style.background = pct >= 90 ? '#e00' : '#ff6600';

    const reset = quota_reset_at ? new Date(quota_reset_at) : null;
    const resetText = reset && !isNaN(reset.getTime()) ? ` · ${reset.toLocaleDateString()} 重置` : '';
    quotaText.textContent = `${used}/${limit} 次${resetText}`;

    quotaExhausted.style.display = used >= limit ? 'block' : 'none';
    if (used >= limit) {
      quotaFill.style.background = '#e00';
    }
  }

  async function loadServerData() {
    const statsSection = cfg.mode === 'cloud' ? document.getElementById('cloudStatsSection') : document.getElementById('localStatsSection');
    const statsContent = cfg.mode === 'cloud' ? document.getElementById('cloudStatsContent') : document.getElementById('localStatsContent');
    const recSection = cfg.mode === 'cloud' ? document.getElementById('cloudRecSection') : document.getElementById('localRecSection');
    const recContent = cfg.mode === 'cloud' ? document.getElementById('cloudRecContent') : document.getElementById('localRecContent');

    try {
      const statsResp = await sendAction('getStats');
      if (statsResp.ok) {
        const stats = statsResp.data;
        statsSection.style.display = 'block';
        let html = `<p>Total signals: <strong>${stats.total_signals || 0}</strong></p>`;
        if (stats.by_type) {
          html += '<ul>';
          for (const [type, count] of Object.entries(stats.by_type)) {
            html += `<li>${type}: ${count}</li>`;
          }
          html += '</ul>';
        }
        statsContent.innerHTML = html;
      }
    } catch (e) {
      console.error('stats fetch failed', e);
    }

    try {
      const recResp = await sendAction('getRecommendations', { limit: 5 });
      if (recResp.ok) {
        const data = recResp.data;
        if (data.recommendations && data.recommendations.length > 0) {
          recSection.style.display = 'block';
          let html = '<ul>';
          for (const rec of data.recommendations) {
            html += `<li><a href="https://news.ycombinator.com/item?id=${rec.hn_id}" target="_blank">${rec.title}</a><br><small>${rec.reason}</small></li>`;
          }
          html += '</ul>';
          recContent.innerHTML = html;
        }
      }
    } catch (e) {
      console.error('rec fetch failed', e);
    }
  }

  function hideDataSections() {
    ['cloudStatsSection', 'cloudRecSection', 'localStatsSection', 'localRecSection', 'quotaSection', 'quotaExhausted'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.style.display = 'none';
    });
  }

  await refreshConnection();
});
