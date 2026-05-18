const SERVER_URL_KEY = 'hn_reader_server_url';
const LANGUAGE_KEY = 'hn_reader_language';

document.addEventListener('DOMContentLoaded', async () => {
  const serverInput = document.getElementById('serverUrl');
  const languageSelect = document.getElementById('language');
  const saveBtn = document.getElementById('saveBtn');
  const status = document.getElementById('status');
  const statsSection = document.getElementById('statsSection');
  const statsContent = document.getElementById('statsContent');
  const recSection = document.getElementById('recSection');
  const recContent = document.getElementById('recContent');

  const result = await chrome.storage.local.get([SERVER_URL_KEY, LANGUAGE_KEY]);
  serverInput.value = result[SERVER_URL_KEY] || 'http://localhost:8080';
  languageSelect.value = result[LANGUAGE_KEY] || 'zh';

  saveBtn.addEventListener('click', async () => {
    await chrome.storage.local.set({
      [SERVER_URL_KEY]: serverInput.value,
      [LANGUAGE_KEY]: languageSelect.value
    });
    status.textContent = 'Saved!';
    setTimeout(() => status.textContent = '', 2000);
  });

  const serverUrl = serverInput.value;

  try {
    const statsResp = await fetch(`${serverUrl}/api/signal/stats`);
    if (statsResp.ok) {
      const stats = await statsResp.json();
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
    const recResp = await fetch(`${serverUrl}/api/recommendations?limit=5`);
    if (recResp.ok) {
      const data = await recResp.json();
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
});
