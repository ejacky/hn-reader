const DOWNLOAD_URL = 'https://github.com/ejacky/hn-reader/releases/latest';
const UPGRADE_URL = 'https://hn-reader.app/upgrade'; // 占位，后续替换为真实支付页

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
  const upgradeCard = document.getElementById('upgradeCard');
  const upgradeBtn = document.getElementById('upgradeBtn');
  const quotaWarning = document.getElementById('quotaWarning');
  const upgradeWarningBtn = document.getElementById('upgradeWarningBtn');
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

  upgradeBtn.addEventListener('click', () => {
    chrome.tabs.create({ url: UPGRADE_URL });
  });

  upgradeWarningBtn.addEventListener('click', () => {
    chrome.tabs.create({ url: UPGRADE_URL });
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

    const { plan, quota_used, quota_limit } = resp.data;
    cloudAccountStatus.textContent = plan === 'free' ? '匿名账户（免费版）' : (plan || '匿名账户');

    quotaSection.style.display = 'block';
    const used = quota_used || 0;
    const limit = quota_limit || 50;
    const pct = Math.min(100, Math.round((used / limit) * 100));
    quotaFill.style.width = `${pct}%`;
    quotaFill.style.background = pct >= 90 ? '#e00' : '#ff6600';
    quotaText.textContent = `${used}/${limit} 次`;

    upgradeCard.style.display = 'none';
    quotaWarning.style.display = 'none';

    if (used >= limit) {
      quotaFill.style.background = '#e00';
      upgradeCard.style.display = 'block';
    } else if (pct >= 80) {
      quotaWarning.style.display = 'block';
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
    ['cloudStatsSection', 'cloudRecSection', 'localStatsSection', 'localRecSection', 'quotaSection'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.style.display = 'none';
    });
  }

  await refreshConnection();
});
