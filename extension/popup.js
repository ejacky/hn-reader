const DOWNLOAD_URL = 'https://github.com/ejacky/hn-reader/releases/latest';
const HN_URL = 'https://news.ycombinator.com/';

document.addEventListener('DOMContentLoaded', async () => {
  await ensureI18n();

  const statusEl = document.getElementById('status');

  const openHnLink = document.getElementById('openHnLink');
  openHnLink.addEventListener('click', (e) => {
    // 保留 Ctrl/Cmd/Shift/中键的原生新标签行为
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    chrome.tabs.create({ url: HN_URL }, () => {
      // 出错时保留 popup，让用户可以再点一次
      if (!chrome.runtime.lastError) window.close();
    });
  });

  // Cloud elements
  const cloudSection = document.getElementById('cloudSection');
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
    updateSummaryWarning();
    statusEl.textContent = t('statusSaved');
    statusEl.style.color = '#008000';
    setTimeout(() => statusEl.textContent = '', 2000);
  });

  // 语言设置：界面语言立即生效；摘要语言默认跟随界面，也可单独指定。
  const uiLanguageSelect = document.getElementById('uiLanguage');
  const summaryLanguageSelect = document.getElementById('summaryLanguage');
  const summaryLanguageWarning = document.getElementById('summaryLanguageWarning');
  const languageToggle = document.getElementById('languageToggle');
  const languageBody = document.getElementById('languageBody');
  const languageSummary = document.getElementById('languageSummary');
  const languageWarningBadge = document.getElementById('languageWarningBadge');

  function uiLanguageName() {
    return resolvedHtmlLang() === 'zh-CN' ? '中文' : 'English';
  }

  function updateSummaryFollowLabel() {
    summaryLanguageSelect.options[0].textContent = t('summaryLanguageFollow', [uiLanguageName()]);
  }

  function updateSummaryWarning() {
    const warning = t('summaryPreloadWarning');
    summaryLanguageWarning.hidden = !autoPreloadToggle.checked;
    languageWarningBadge.hidden = !autoPreloadToggle.checked;
    if (autoPreloadToggle.checked) {
      languageWarningBadge.title = warning;
    } else {
      languageWarningBadge.removeAttribute('title');
    }
  }

  // 折叠行只显示“当前是什么”，不重复展开区里的解释。
  function languageSummaryText() {
    const uiSetting = uiLanguageSelect.value;
    const uiText = uiSetting === 'auto'
      ? t('languageSummaryUiAuto', [uiLanguageName()])
      : uiLanguageName();
    const summarySetting = summaryLanguageSelect.value;
    const summaryText = summarySetting === 'auto'
      ? t('summaryLanguageFollowShort')
      : summaryLanguageSelect.options[summaryLanguageSelect.selectedIndex].textContent;
    return t('languageSummary', [uiText, summaryText]);
  }

  function updateLanguageSummary() {
    languageSummary.textContent = languageSummaryText();
  }

  const uiLanguageResult = await chrome.storage.local.get([CONFIG.KEYS.UI_LANGUAGE]);
  uiLanguageSelect.value = uiLanguageResult[CONFIG.KEYS.UI_LANGUAGE] || 'auto';
  uiLanguageSelect.addEventListener('change', async () => {
    await chrome.storage.local.set({ [CONFIG.KEYS.UI_LANGUAGE]: uiLanguageSelect.value });
    sessionStorage.setItem('hn_reader_ui_language_changed', '1');
    location.reload();
  });

  // 'auto'（跟随界面）不写存储；显式选择才写 hn_reader_language，选回跟随就删掉。
  const summaryResult = await chrome.storage.local.get([CONFIG.KEYS.LANGUAGE]);
  const storedSummaryLanguage = summaryResult[CONFIG.KEYS.LANGUAGE];
  summaryLanguageSelect.value = storedSummaryLanguage === 'zh' || storedSummaryLanguage === 'en'
    ? storedSummaryLanguage
    : 'auto';
  updateSummaryFollowLabel();
  updateSummaryWarning();
  updateLanguageSummary();

  // 折叠状态：默认收起；记住用户上次的选择。
  let languageOpen = false;
  try {
    const languageOpenResult = await chrome.storage.local.get([CONFIG.KEYS.LANGUAGE_OPEN]);
    languageOpen = languageOpenResult[CONFIG.KEYS.LANGUAGE_OPEN] === true;
  } catch (e) {
    // storage 不可用时保持收起。
  }

  function setLanguageOpen(open, persist) {
    languageOpen = open;
    languageBody.hidden = !open;
    languageToggle.setAttribute('aria-expanded', String(open));
    if (persist) {
      chrome.storage.local.set({ [CONFIG.KEYS.LANGUAGE_OPEN]: open }).catch(() => {});
    }
  }

  setLanguageOpen(languageOpen, false);
  languageToggle.addEventListener('click', () => setLanguageOpen(!languageOpen, true));

  summaryLanguageSelect.addEventListener('change', async () => {
    if (summaryLanguageSelect.value === 'auto') {
      await chrome.storage.local.remove([CONFIG.KEYS.LANGUAGE]);
    } else {
      await chrome.storage.local.set({ [CONFIG.KEYS.LANGUAGE]: summaryLanguageSelect.value });
    }
    updateLanguageSummary();
    statusEl.textContent = t('statusSaved');
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
    } else if (mode === 'local') {
      localSection.style.display = 'block';
      serverUrlInput.value = cfg.serverUrl;
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

  saveLocalBtn.addEventListener('click', async () => {
    const url = serverUrlInput.value.trim() || 'http://localhost:8080';
    if (!url.startsWith('http')) {
      statusEl.textContent = t('errInvalidUrl');
      statusEl.style.color = '#e00';
      return;
    }
    await setReaderConfig({
      mode: 'local',
      serverUrl: url
    });
    cfg = await getReaderConfig();
    statusEl.textContent = t('statusSaving');
    await refreshConnection();
  });

  exportDataBtn.addEventListener('click', async () => {
    const resp = await sendAction('apiRequest', { method: 'GET', path: '/api/account/export' });
    if (!resp.ok) {
      statusEl.textContent = t('errExportFailed', [resp.error || t('unknownError')]);
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
    statusEl.textContent = t('statusExported');
    statusEl.style.color = '#008000';
  });

  deleteAccountBtn.addEventListener('click', async () => {
    if (!confirm(t('confirmDeleteAccount'))) return;
    const resp = await sendAction('apiRequest', { method: 'DELETE', path: '/api/account' });
    if (!resp.ok) {
      statusEl.textContent = t('errDeleteFailed', [resp.error || t('unknownError')]);
      statusEl.style.color = '#e00';
      return;
    }
    await chrome.storage.local.remove([CONFIG.KEYS.CLOUD_TOKEN]);
    cloudAccountStatus.textContent = t('cloudDataDeleted');
    quotaSection.style.display = 'none';
    quotaExhausted.style.display = 'none';
    statusEl.textContent = t('accountDeletedHint');
    statusEl.style.color = '#008000';
  });

  async function sendAction(action, payload = {}) {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ action, payload }, (resp) => {
        resolve(resp || { ok: false, error: t('errNoResponse') });
      });
    });
  }

  async function refreshConnection() {
    cfg = await getReaderConfig();
    connText.textContent = t('connChecking');
    connIndicator.style.color = '#999';
    statusEl.textContent = '';

    const probe = await sendAction('probeConnection');

    if (probe.ok && probe.data.connected) {
      connIndicator.style.color = '#008000';
      const info = probe.data.info || {};
      connText.textContent = cfg.mode === 'cloud' ? t('cloudConnected') : t('localConnected');
      statusEl.textContent = t('connectSuccess');
      statusEl.style.color = '#008000';
      localGuide.style.display = 'none';

      if (cfg.mode === 'cloud') {
        await loadCloudMeta();
      }
      await loadServerData();
    } else {
      connIndicator.style.color = '#e00';
      connText.textContent = cfg.mode === 'cloud' ? t('cloudDisconnected') : t('localDisconnected');
      statusEl.textContent = probe.ok ? t('errCannotConnect') : (probe.error || t('errCannotConnect'));
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
      cloudAccountStatus.textContent = t('accountAnonymous');
      return;
    }

    const { plan, quota_used, quota_limit, quota_reset_at } = resp.data;
    cloudAccountStatus.textContent = plan === 'free' ? t('accountAnonymousFree') : (plan || t('accountAnonymous'));

    quotaSection.style.display = 'block';
    const used = quota_used || 0;
    const limit = quota_limit || 50;
    const pct = Math.min(100, Math.round((used / limit) * 100));
    quotaFill.style.width = `${pct}%`;
    quotaFill.style.background = pct >= 90 ? '#e00' : '#ff6600';

    const reset = quota_reset_at ? new Date(quota_reset_at) : null;
    const resetText = reset && !isNaN(reset.getTime()) ? t('quotaReset', [reset.toLocaleDateString()]) : '';
    quotaText.textContent = t('quotaUsage', [String(used), String(limit)]) + resetText;

    quotaExhausted.style.display = used >= limit ? 'block' : 'none';
    if (used >= limit) {
      quotaFill.style.background = '#e00';
    }
  }

  async function loadStats(mode) {
    const content = document.getElementById(mode === 'cloud' ? 'cloudStatsContent' : 'localStatsContent');
    try {
      const resp = await sendAction('getStats');
      if (!resp.ok) throw new Error(resp.error || 'stats request failed');
      const stats = resp.data;
      let html = `<p>${t('statsTotalSignals')} <strong>${stats.total_signals || 0}</strong></p>`;
      if (stats.by_type) {
        html += '<ul>';
        for (const [type, count] of Object.entries(stats.by_type)) {
          html += `<li>${type}: ${count}</li>`;
        }
        html += '</ul>';
      }
      content.innerHTML = html;
      return true;
    } catch (e) {
      console.error('stats fetch failed', e);
      content.textContent = t('statsLoadFailed');
      return false;
    }
  }

  // 统计默认封存：只有点开低权重按钮才请求，且同一 popup 会话内只请求一次。
  function setupStatsToggle(mode) {
    const toggle = document.getElementById(mode === 'cloud' ? 'cloudStatsToggle' : 'localStatsToggle');
    const section = document.getElementById(mode === 'cloud' ? 'cloudStatsSection' : 'localStatsSection');
    let loaded = false;
    toggle.addEventListener('click', async () => {
      const open = toggle.getAttribute('aria-expanded') !== 'true';
      if (open && !loaded) {
        await loadStats(mode);
        // 失败也算尝试过：展开区显示错误，不因反复点开而重复请求。
        loaded = true;
      }
      toggle.setAttribute('aria-expanded', String(open));
      section.hidden = !open;
    });
  }

  async function loadServerData() {
    const recSection = cfg.mode === 'cloud' ? document.getElementById('cloudRecSection') : document.getElementById('localRecSection');
    const recContent = cfg.mode === 'cloud' ? document.getElementById('cloudRecContent') : document.getElementById('localRecContent');

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
    ['cloudRecSection', 'localRecSection', 'quotaSection', 'quotaExhausted'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.style.display = 'none';
    });
    [['cloudStatsSection', 'cloudStatsToggle'], ['localStatsSection', 'localStatsToggle']].forEach(([sectionId, toggleId]) => {
      const section = document.getElementById(sectionId);
      if (section) section.hidden = true;
      const toggle = document.getElementById(toggleId);
      if (toggle) toggle.setAttribute('aria-expanded', 'false');
    });
  }

  setupStatsToggle('cloud');
  setupStatsToggle('local');

  await refreshConnection();

  // 刚切换过界面语言：连接状态之后提示，避免被 refreshConnection 覆盖。
  if (sessionStorage.getItem('hn_reader_ui_language_changed')) {
    sessionStorage.removeItem('hn_reader_ui_language_changed');
    const hint = t('uiLanguageChangedHint');
    statusEl.textContent = hint;
    statusEl.style.color = '#008000';
    setTimeout(() => {
      if (statusEl.textContent === hint) statusEl.textContent = '';
    }, 6000);
  }
});
