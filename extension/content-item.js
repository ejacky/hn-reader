(function() {
  'use strict';

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
      console.error('HN Reader: send signal failed', e);
    }
  }

  async function fetchCommentSummary(hnId, articleId, author, text) {
    const result = await chrome.storage.local.get(['hn_reader_language']);
    const lang = result['hn_reader_language'] || 'zh';
    return await sendToBg('summarizeComment', { hn_id: hnId, article_id: articleId, author, text, lang: lang });
  }

  function getArticleIdFromUrl() {
    const params = new URLSearchParams(window.location.search);
    return parseInt(params.get('id'), 10);
  }

  function injectCommentSummaryButtons() {
    const comments = document.querySelectorAll('.comtr');
    const articleId = getArticleIdFromUrl();

    comments.forEach(commentRow => {
      if (commentRow.dataset.hnReaderInjected) return;
      commentRow.dataset.hnReaderInjected = 'true';

      const commentCell = commentRow.querySelector('.comment');
      const header = commentRow.querySelector('.comhead');
      if (!commentCell || !header) return;

      const hnId = parseInt(commentRow.id, 10);
      if (!hnId) return;

      const authorEl = header.querySelector('.hnuser');
      const author = authorEl ? authorEl.textContent : '';

      const textEl = commentCell.querySelector('.commtext');
      const text = textEl ? textEl.innerText : commentCell.innerText;

      const btn = document.createElement('a');
      btn.href = 'javascript:void(0)';
      btn.textContent = ' [summarize]';
      btn.style.color = '#ff6600';
      btn.style.fontSize = '10px';
      btn.style.textDecoration = 'none';
      btn.style.cursor = 'pointer';
      btn.style.marginLeft = '6px';

      let summaryDiv = null;
      btn.addEventListener('click', async (e) => {
        e.preventDefault();
        e.stopPropagation();

        if (summaryDiv) {
          summaryDiv.remove();
          summaryDiv = null;
          return;
        }

        btn.textContent = ' [loading...]';
        try {
          const data = await fetchCommentSummary(hnId, articleId, author, text);
          summaryDiv = document.createElement('div');
          summaryDiv.style.cssText = `
            margin-top: 6px;
            padding: 8px;
            background: #f6f6ef;
            border: 1px solid #ff6600;
            border-radius: 3px;
            font-size: 11px;
            color: #000;
            line-height: 1.4;
          `;
          summaryDiv.textContent = data.summary || 'No summary available.';

          const closeBtn = document.createElement('a');
          closeBtn.href = 'javascript:void(0)';
          closeBtn.textContent = ' [close]';
          closeBtn.style.color = '#666';
          closeBtn.style.fontSize = '10px';
          closeBtn.style.marginLeft = '8px';
          closeBtn.style.cursor = 'pointer';
          closeBtn.onclick = (ev) => { ev.preventDefault(); summaryDiv.remove(); summaryDiv = null; btn.textContent = ' [summarize]'; };
          closeBtn.addEventListener('mousedown', (e) => e.stopPropagation());
          summaryDiv.appendChild(closeBtn);

          makeDraggable(summaryDiv);
          commentCell.appendChild(summaryDiv);
          await sendSignal('comment', hnId, 'comment_summary_viewed', 2, { author });
        } catch (err) {
          console.error('HN Reader comment summary:', err);
        } finally {
          btn.textContent = ' [summarize]';
        }
      });

      const replyLink = header.querySelector('a[href^="reply"]');
      if (replyLink) {
        replyLink.parentElement.insertBefore(btn, replyLink.nextSibling);
      } else {
        header.appendChild(btn);
      }

      const links = commentCell.querySelectorAll('a');
      links.forEach(link => {
        if (link.href && link.href.includes('news.ycombinator.com/item')) {
          link.addEventListener('click', () => {
            sendSignal('comment', hnId, 'comment_clicked', 3, { author });
          });
        }
      });
    });
  }

  function monitorFavoriteButtons() {
    const favLinks = document.querySelectorAll('a[href^="fave"], a[href^="/fave"]');
    favLinks.forEach(link => {
      if (link.dataset.hnReaderFavMonitored) return;
      link.dataset.hnReaderFavMonitored = 'true';

      link.addEventListener('click', () => {
        const params = new URLSearchParams(window.location.search);
        const itemId = parseInt(params.get('id'), 10);
        if (itemId) {
          sendSignal('article', itemId, 'favorited', 5, {});
        }
      });
    });
  }

  function makeDraggable(el) {
    let isDragging = false;
    let startX, startY, currentX = 0, currentY = 0;

    el.addEventListener('mousedown', (e) => {
      isDragging = false;
      startX = e.clientX;
      startY = e.clientY;

      const onMove = (ev) => {
        const dx = ev.clientX - startX;
        const dy = ev.clientY - startY;
        if (!isDragging && (Math.abs(dx) > 3 || Math.abs(dy) > 3)) {
          isDragging = true;
          el.style.cursor = 'grabbing';
          el.style.userSelect = 'none';
        }
        if (isDragging) {
          el.style.transform = `translate(${currentX + dx}px, ${currentY + dy}px)`;
        }
      };

      const onUp = (ev) => {
        if (isDragging) {
          currentX += ev.clientX - startX;
          currentY += ev.clientY - startY;
        }
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        el.style.cursor = '';
        el.style.userSelect = '';
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    });
  }

  function init() {
    injectCommentSummaryButtons();
    monitorFavoriteButtons();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  const observer = new MutationObserver(() => {
    injectCommentSummaryButtons();
    monitorFavoriteButtons();
  });
  observer.observe(document.body, { childList: true, subtree: true });
})();
