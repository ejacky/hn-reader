(function() {
  'use strict';

  console.log('[HN Reader] content-list.js loaded');

  async function sendToBg(action, payload) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage({ action, payload }, (response) => {
        if (chrome.runtime.lastError) {
          reject(new Error(chrome.runtime.lastError.message));
          return;
        }
        if (!response || !response.ok) {
          reject(new Error(response?.error || 'Unknown error'));
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

  async function getServerUrl() {
    const result = await chrome.storage.local.get(['hn_reader_server_url']);
    return result['hn_reader_server_url'] || 'http://localhost:8080';
  }

  async function fetchSummary(hnId, url, title) {
    const result = await chrome.storage.local.get(['hn_reader_language']);
    const lang = result['hn_reader_language'] || 'zh';
    return await sendToBg('summarizeArticle', { hn_id: hnId, url: url, title: title, lang: lang });
  }

  async function preloadArticles(articles) {
    if (articles.length === 0) return;
    const serverUrl = await getServerUrl();
    try {
      fetch(`${serverUrl}/api/summarize/article/preload`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ articles })
      }).catch(() => {});
    } catch (e) {
      console.error('[HN Reader] preload request failed:', e);
    }
  }

  async function pollPreloadStatus(articleIds, lang) {
    if (articleIds.length === 0) return;
    const serverUrl = await getServerUrl();
    try {
      const resp = await fetch(`${serverUrl}/api/summarize/article/preload-status?ids=${articleIds.join(',')}&lang=${lang}`);
      if (!resp.ok) return;
      const data = await resp.json();
      if (data.ready_ids) {
        data.ready_ids.forEach(id => {
          const btn = document.querySelector(`a[data-hn-reader-summary][data-hn-id="${id}"]`);
          if (btn) {
            btn.style.color = '#228b22';
            btn.title = '已预加载';
          }
        });
      }
    } catch (e) {
      console.error('[HN Reader] preload status poll failed:', e);
    }
  }

  function createSummaryCard(data, hnId) {
    const card = document.createElement('div');
    card.className = 'hn-reader-summary-card';
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
    closeBtn.onclick = () => card.remove();
    card.appendChild(closeBtn);

    return card;
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
          card.remove();
          card = null;
          return;
        }

        btn.textContent = ' [loading...]';
        try {
          const data = await fetchSummary(hnId, url, title);
          card = createSummaryCard(data, hnId);
          const rect = btn.getBoundingClientRect();
          card.style.top = (rect.bottom + 4) + 'px';
          card.style.left = rect.left + 'px';
          document.body.appendChild(card);
          await sendSignal('article', hnId, 'summary_viewed', 1, { title });
        } catch (err) {
          console.error('[HN Reader] summarize error:', err);
          alert('Failed to summarize: ' + err.message);
        } finally {
          btn.textContent = ' [summary]';
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

    // 只预加载前 15 篇
    const batch = articles.slice(0, 15);
    if (batch.length === 0) return;

    const result = await chrome.storage.local.get(['hn_reader_language']);
    const lang = result['hn_reader_language'] || 'zh';
    batch.forEach(a => a.lang = lang);

    // 提交预加载（不等待）
    preloadArticles(batch);

    // 开始轮询状态
    const ids = batch.map(a => a.hn_id);
    pollPreloadStatus(ids, lang);
    const intervalId = setInterval(() => {
      pollPreloadStatus(ids, lang);
    }, 3000);

    // 页面卸载时停止轮询
    window.addEventListener('beforeunload', () => {
      clearInterval(intervalId);
    });
  }

  function init() {
    console.log('[HN Reader] init() called, readyState:', document.readyState);
    injectSummaryButtons();
    applyInterestMatches();
    // 延迟 2 秒后触发预加载，等页面稳定
    setTimeout(startPreload, 2000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  const observerTarget = document.body || document.documentElement;
  if (observerTarget) {
    const observer = new MutationObserver(() => {
      injectSummaryButtons();
    });
    observer.observe(observerTarget, { childList: true, subtree: true });
  }
})();
