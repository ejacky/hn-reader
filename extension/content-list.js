(function() {
  'use strict';

  console.log('[HN Reader] content-list.js loaded');

  async function sendToBg(action, payload) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage({ action, payload }, (response) => {
        if (chrome.runtime.lastError) {
          const err = new Error(chrome.runtime.lastError.message);
          err.status = 0;
          reject(err);
          return;
        }
        if (!response || !response.ok) {
          const err = new Error(response?.error || 'Unknown error');
          err.status = response?.status || 0;
          err.code = response?.code || '';
          reject(err);
          return;
        }
        resolve(response.data);
      });
    });
  }

  async function sendSignal(targetType, targetId, signalType, strength, context) {
    try {
      await sendToBg('sendSignal', {
        target_type: targetType,
        target_id: targetId,
        signal_type: signalType,
        strength: strength,
        context: context || {}
      });
    } catch (e) {
      console.error('[HN Reader] send signal failed:', e);
    }
  }

  async function fetchSummary(hnId, url, title) {
    const result = await chrome.storage.local.get(['hn_reader_language']);
    const lang = result['hn_reader_language'] || 'zh';
    return await sendToBg('summarizeArticle', { hn_id: hnId, url: url, title: title, lang: lang });
  }

  async function preloadArticles(articles) {
    if (articles.length === 0) return;
    try {
      await sendToBg('preloadArticles', { articles });
    } catch (e) {
      console.error('[HN Reader] preload request failed:', e);
    }
  }

  async function pollPreloadStatus(articleIds, lang) {
    if (articleIds.length === 0) return;
    try {
      const data = await sendToBg('preloadStatus', { ids: articleIds.join(','), lang });
      if (data.ready_ids) {
        data.ready_ids.forEach(id => {
          const btn = document.querySelector(`a[data-hn-reader-summary][data-hn-id="${id}"]`);
          if (btn) {
            btn.dataset.hnReaderPreloaded = 'true';
            // 如果当前摘要弹窗正打开，不要覆盖蓝色激活状态
            const popupOpen = document.querySelector(`.hn-reader-popup[data-hn-reader-hn-id="${id}"]`);
            if (!popupOpen) {
              btn.style.color = '#228b22';
              btn.title = '已预加载';
            }
          }
        });
      }
    } catch (e) {
      console.error('[HN Reader] preload status poll failed:', e);
    }
  }

  function showNoticeTip(targetBtn, title, message) {
    // 移除已存在的提示
    const existing = document.getElementById('hn-reader-quota-tip');
    if (existing) existing.remove();

    const tip = document.createElement('div');
    tip.id = 'hn-reader-quota-tip';
    tip.className = 'hn-reader-popup';
    tip.style.cssText = `
      position: fixed;
      z-index: 99999;
      background: #fff;
      border: 1px solid #ff6600;
      border-radius: 6px;
      padding: 12px;
      width: 260px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      font-family: Verdana, Geneva, sans-serif;
      font-size: 12px;
      line-height: 1.5;
      color: #000;
    `;
    tip.innerHTML = `
      <div style="font-weight:bold;color:#ff6600;margin-bottom:6px;">${title}</div>
      <div style="margin-bottom:10px;color:#666;">${message}</div>
      <button id="hn-reader-quota-close" style="background:#ff6600;color:#fff;border:none;padding:5px 12px;border-radius:3px;cursor:pointer;font-size:11px;">知道了</button>
    `;

    const rect = targetBtn.getBoundingClientRect();
    tip.style.top = (rect.bottom + 6) + 'px';
    tip.style.left = rect.left + 'px';
    document.body.appendChild(tip);

    document.getElementById('hn-reader-quota-close').addEventListener('click', () => tip.remove());

    setTimeout(() => {
      if (tip.parentNode) tip.remove();
    }, 8000);
  }

  function showQuotaExceededTip(targetBtn) {
    showNoticeTip(
      targetBtn,
      '本月免费额度已用完',
      '下月 1 日自动重置；已缓存的摘要仍可查看。'
    );
  }

  function setSummaryBtnState(hnId, active) {
    const btn = document.querySelector(`a[data-hn-reader-summary][data-hn-id="${hnId}"]`);
    if (!btn) return;
    if (active) {
      btn.style.color = '#0066cc';
      btn.title = '摘要已打开';
    } else if (btn.dataset.hnReaderPreloaded === 'true') {
      btn.style.color = '#228b22';
      btn.title = '已预加载';
    } else {
      btn.style.color = '#ff6600';
      btn.title = '';
    }
  }

  function createSummaryCard(data, hnId, title) {
    const card = document.createElement('div');
    card.className = 'hn-reader-summary-card hn-reader-popup';
    if (!window.__hnReaderPopupCounter) window.__hnReaderPopupCounter = 0;
    card.dataset.hnReaderPopupId = ++window.__hnReaderPopupCounter;
    card.dataset.hnReaderHnId = hnId;
    card.style.cssText = `
      position: fixed;
      z-index: 99999;
      background: #f6f6ef;
      border: 1px solid #ff6600;
      border-radius: 4px;
      padding: 12px;
      width: 420px;
      max-width: 90vw;
      max-height: 80vh;
      overflow-y: auto;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      font-family: Verdana, Geneva, sans-serif;
      font-size: 12px;
      line-height: 1.5;
      color: #000;
    `;

    const header = document.createElement('div');
    header.style.cssText = `
      display: none;
      margin: -6px 0 4px 0;
      padding-top: 2px;
      font-size: 11px;
      color: #888;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    `;
    header.textContent = title || 'Untitled';
    card.appendChild(header);

    const summaryP = document.createElement('p');
    summaryP.style.margin = '0 0 8px 0';
    summaryP.style.fontWeight = 'bold';
    summaryP.textContent = data.summary || 'No summary available.';
    card.appendChild(summaryP);

    if (data.key_points && data.key_points.length > 0) {
      const ul = document.createElement('ul');
      ul.style.margin = '0';
      ul.style.paddingLeft = '16px';
      data.key_points.forEach(kp => {
        const li = document.createElement('li');
        li.style.marginBottom = '2px';
        li.textContent = kp;
        ul.appendChild(li);
      });
      card.appendChild(ul);
    }

    const closeBtn = document.createElement('button');
    closeBtn.textContent = 'Close';
    closeBtn.style.cssText = `
      margin-top: 8px;
      background: #ff6600;
      color: #fff;
      border: none;
      padding: 4px 10px;
      cursor: pointer;
      font-size: 11px;
      border-radius: 2px;
    `;
    closeBtn.onclick = () => {
      setSummaryBtnState(hnId, false);
      card.remove();
    };
    closeBtn.addEventListener('mousedown', (e) => e.stopPropagation());
    card.appendChild(closeBtn);

    makeDraggable(card, () => {
      header.style.display = 'block';
    });
    return card;
  }

  function makeDraggable(el, onDragStart) {
    let isDragging = false;
    let startX, startY, initLeft, initTop;

    el.addEventListener('mousedown', (e) => {
      isDragging = false;
      startX = e.clientX;
      startY = e.clientY;
      initLeft = parseFloat(el.style.left) || 0;
      initTop = parseFloat(el.style.top) || 0;

      const onMove = (ev) => {
        const dx = ev.clientX - startX;
        const dy = ev.clientY - startY;
        if (!isDragging && (Math.abs(dx) > 3 || Math.abs(dy) > 3)) {
          isDragging = true;
          if (onDragStart) onDragStart();
          el.style.cursor = 'grabbing';
          el.style.userSelect = 'none';
        }
        if (isDragging) {
          el.style.left = (initLeft + dx) + 'px';
          el.style.top = (initTop + dy) + 'px';
        }
      };

      const onUp = () => {
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        el.style.cursor = '';
        el.style.userSelect = '';
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    });
  }

  function injectSummaryButtons() {
    const rows = document.querySelectorAll('.athing');
    console.log('[HN Reader] found', rows.length, '.athing rows');

    rows.forEach(row => {
      if (row.dataset.hnReaderInjected) return;
      row.dataset.hnReaderInjected = 'true';

      const titleline = row.querySelector('span.titleline');
      if (!titleline) {
        console.log('[HN Reader] no titleline in row', row.id);
        return;
      }

      const titleLink = titleline.querySelector('a');
      if (!titleLink) {
        console.log('[HN Reader] no link in titleline');
        return;
      }

      const hnId = parseInt(row.id, 10);
      if (!hnId) {
        console.log('[HN Reader] no hnId');
        return;
      }

      const url = titleLink.href;
      const title = titleLink.textContent;

      const btn = document.createElement('a');
      btn.href = 'javascript:void(0)';
      btn.textContent = ' [summary]';
      btn.style.color = '#ff6600';
      btn.style.fontSize = '10px';
      btn.style.textDecoration = 'none';
      btn.style.cursor = 'pointer';
      btn.style.marginLeft = '4px';
      btn.setAttribute('data-hn-reader-summary', 'true');
      btn.setAttribute('data-hn-id', hnId);

      const titleCell = titleline.closest('td.title');

      let card = null;
      btn.addEventListener('click', async (e) => {
        e.preventDefault();
        e.stopPropagation();

        if (card) {
          setSummaryBtnState(hnId, false);
          card.remove();
          card = null;
          return;
        }

        if (btn.dataset.hnReaderQuotaExceeded === 'true') {
          showQuotaExceededTip(btn);
          return;
        }

        btn.textContent = ' [loading...]';
        try {
          const data = await fetchSummary(hnId, url, title);
          card = createSummaryCard(data, hnId, title);
          const rect = btn.getBoundingClientRect();
          card.style.top = (rect.bottom + 4) + 'px';
          card.style.left = rect.left + 'px';
          document.body.appendChild(card);
          btn.dataset.hnReaderPreloaded = 'true';
          setSummaryBtnState(hnId, true);
          await sendSignal('article', hnId, 'summary_viewed', 1, { title });
        } catch (err) {
          console.error('[HN Reader] summarize error:', err);
          if (err.code === 'quota_exceeded' || (err.status === 429 && !err.code)) {
            btn.textContent = ' [额度已用完]';
            btn.style.color = '#e00';
            btn.title = '点击查看额度说明';
            btn.dataset.hnReaderQuotaExceeded = 'true';
          } else if (err.code === 'rate_limited') {
            btn.textContent = ' [请稍后]';
            btn.style.color = '#e00';
            btn.title = '请求过于频繁，请稍后重试';
            showNoticeTip(btn, '请求过于频繁', '请等待几秒后再试。');
            setTimeout(() => {
              if (btn.textContent === ' [请稍后]') {
                btn.textContent = ' [summary]';
                btn.style.color = '#ff6600';
                btn.title = '';
              }
            }, 5000);
          } else if (err.code === 'service_busy' || err.code === 'llm_unavailable') {
            btn.textContent = ' [服务繁忙]';
            btn.style.color = '#e00';
            btn.title = '服务繁忙，请稍后重试';
            setTimeout(() => {
              if (btn.textContent === ' [服务繁忙]') {
                btn.textContent = ' [summary]';
                btn.style.color = '#ff6600';
                btn.title = '';
              }
            }, 5000);
          } else {
            btn.textContent = ' [failed]';
            btn.style.color = '#e00';
            btn.title = '摘要失败：' + err.message;
            setTimeout(() => {
              btn.textContent = ' [summary]';
              btn.style.color = '#ff6600';
              btn.title = '';
            }, 3000);
          }
        } finally {
          if (btn.textContent === ' [loading...]') {
            btn.textContent = ' [summary]';
          }
        }
      });

      titleline.appendChild(btn);

      titleLink.addEventListener('click', () => {
        sendSignal('article', hnId, 'article_clicked', 3, { title, url });
      });
    });
  }

  async function applyInterestMatches() {
    const rows = document.querySelectorAll('.athing');
    const articles = [];
    const rowMap = new Map();

    rows.forEach(row => {
      const titleline = row.querySelector('span.titleline');
      if (!titleline) return;
      const titleLink = titleline.querySelector('a');
      if (!titleLink) return;
      const hnId = parseInt(row.id, 10);
      if (!hnId) return;
      const title = titleLink.textContent;
      let domain = '';
      const siteBit = titleline.querySelector('span.sitestr');
      if (siteBit) {
        domain = siteBit.textContent.trim();
      }
      articles.push({ hn_id: hnId, title, domain });
      rowMap.set(hnId, row);
    });

    if (articles.length === 0) return;

    try {
      const data = await sendToBg('batchMatch', { articles });
      if (!data.matches) return;

      data.matches.forEach(match => {
        const row = rowMap.get(match.hn_id);
        if (!row) return;

        const titleline = row.querySelector('span.titleline');
        if (!titleline) return;

        if (match.match_type === 'high') {
          row.style.borderLeft = '3px solid #ff6600';
          row.style.paddingLeft = '4px';
        } else if (match.match_type === 'medium') {
          row.style.borderLeft = '2px solid #ffb380';
          row.style.paddingLeft = '4px';
        }

        if (match.match_tags && match.match_tags.length > 0) {
          const tagSpan = document.createElement('span');
          tagSpan.style.cssText = 'font-size: 10px; color: #666; margin-left: 6px;';
          tagSpan.textContent = '🔖 ' + match.match_tags.slice(0, 3).join(', ');
          titleline.appendChild(tagSpan);
        }
      });
    } catch (e) {
      console.error('[HN Reader] batch match failed:', e);
    }
  }

  async function startPreload() {
    const rows = document.querySelectorAll('.athing');
    const articles = [];
    rows.forEach(row => {
      const titleline = row.querySelector('span.titleline');
      if (!titleline) return;
      const titleLink = titleline.querySelector('a');
      if (!titleLink) return;
      const hnId = parseInt(row.id, 10);
      if (!hnId) return;
      articles.push({
        hn_id: hnId,
        url: titleLink.href,
        title: titleLink.textContent,
        lang: 'zh'
      });
    });

    if (articles.length === 0) return;

    const result = await chrome.storage.local.get(['hn_reader_language', 'hn_reader_auto_preload']);
    const lang = result['hn_reader_language'] || 'zh';
    const autoPreload = !!result['hn_reader_auto_preload'];

    const allIds = articles.map(a => a.hn_id);

    // 只在开启自动预加载且非收藏页面时提交新的预加载请求；收藏页文章可能已总结过
    if (autoPreload && shouldPreloadNewArticles()) {
      const batch = articles.slice(0, 15);
      batch.forEach(a => a.lang = lang);
      preloadArticles(batch);
    }

    // 总是查询当前页文章的已总结状态，刷新后恢复绿色标记
    // （手动加载的文章也会存入服务端缓存，可据此还原状态）
    pollPreloadStatus(allIds, lang);

    // 仅开启自动预加载时才持续轮询，等待后台预加载完成
    if (autoPreload) {
      const intervalId = setInterval(() => {
        pollPreloadStatus(allIds, lang);
      }, 3000);

      // 页面卸载时停止轮询
      window.addEventListener('beforeunload', () => {
        clearInterval(intervalId);
      });
    }
  }

  function shouldPreloadNewArticles() {
    // 收藏页面是以前看过的文章，不要自动请求新的总结
    const path = window.location.pathname;
    return path !== '/fave' && path !== '/favorites';
  }

  async function shouldShowOnboarding() {
    try {
      const result = await chrome.storage.local.get(['hn_reader_onboarding_shown']);
      return !result['hn_reader_onboarding_shown'];
    } catch (e) {
      return false;
    }
  }

  async function markOnboardingShown() {
    try {
      await chrome.storage.local.set({ 'hn_reader_onboarding_shown': true });
    } catch (e) {
      console.error('[HN Reader] mark onboarding failed', e);
    }
  }

  function showOnboardingTooltip() {
    if (document.getElementById('hn-reader-onboarding')) return;

    const card = document.createElement('div');
    card.id = 'hn-reader-onboarding';
    card.className = 'hn-reader-popup';
    card.style.cssText = `
      position: fixed;
      z-index: 99999;
      bottom: 24px;
      right: 24px;
      width: 320px;
      background: #fff;
      border: 1px solid #ff6600;
      border-radius: 6px;
      padding: 16px;
      box-shadow: 0 4px 16px rgba(0,0,0,0.15);
      font-family: Verdana, Geneva, sans-serif;
      font-size: 12px;
      line-height: 1.6;
      color: #000;
    `;

    card.innerHTML = `
      <div style="font-weight:bold;font-size:13px;margin-bottom:8px;color:#ff6600;">👋 HN Reader 已就绪</div>
      <ul style="margin:0 0 12px 16px;padding:0;">
        <li>点击 <strong>[summary]</strong> 查看 AI 摘要</li>
        <li>橙色边框 = 可能感兴趣的文章</li>
        <li>点击扩展图标查看推荐和数据</li>
      </ul>
      <button id="hn-reader-onboarding-close" style="background:#ff6600;color:#fff;border:none;padding:6px 14px;border-radius:3px;cursor:pointer;font-size:11px;">知道了，不再显示</button>
    `;

    document.body.appendChild(card);

    document.getElementById('hn-reader-onboarding-close').addEventListener('click', () => {
      markOnboardingShown();
      card.remove();
    });

    // 10 秒后自动消失
    setTimeout(() => {
      if (card.parentNode) {
        card.remove();
      }
    }, 10000);
  }

  async function init() {
    console.log('[HN Reader] init() called, readyState:', document.readyState);
    injectSummaryButtons();
    applyInterestMatches();
    // 延迟 2 秒后触发预加载/状态查询，等页面稳定
    // startPreload 内部会自己决定是否发送新的预加载请求；收藏页只查已有状态
    setTimeout(startPreload, 2000);

    if (await shouldShowOnboarding()) {
      showOnboardingTooltip();
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  if (!window.__hnReaderEscListenerAdded) {
    window.__hnReaderEscListenerAdded = true;
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        const popups = document.querySelectorAll('.hn-reader-popup');
        if (popups.length === 0) return;
        let topMost = null;
        let maxId = -1;
        popups.forEach(p => {
          const id = parseInt(p.dataset.hnReaderPopupId, 10);
          if (id > maxId) {
            maxId = id;
            topMost = p;
          }
        });
        if (topMost) {
          const popupHnId = parseInt(topMost.dataset.hnReaderHnId, 10);
          if (popupHnId) setSummaryBtnState(popupHnId, false);
          topMost.remove();
          e.stopPropagation();
        }
      }
    });
  }

  const observerTarget = document.body || document.documentElement;
  if (observerTarget) {
    const observer = new MutationObserver(() => {
      injectSummaryButtons();
    });
    observer.observe(observerTarget, { childList: true, subtree: true });
  }
})();
