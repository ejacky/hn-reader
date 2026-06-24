/**
 * HN Reader Mock Cloud Server
 * 用于开发阶段验证扩展 cloud 模式的基本连通性
 *
 * 启动: node server.js
 * 默认端口: 8081
 */

const http = require('http');
const url = require('url');

const PORT = process.env.PORT || 8081;
const VALID_TOKEN = 'dev-token';

const store = {
  articles: new Map(),
  comments: new Map(),
  signals: [],
  users: new Map(),
  summarizeCount: 0,
  quotaLimit: 3 // 开发测试用，很小的限额便于验证升级引导
};

function corsHeaders(res) {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
}

function sendJSON(res, status, data) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(data));
}

function getAuthUser(req) {
  const auth = req.headers['authorization'] || '';
  const token = auth.replace(/^Bearer\s+/i, '');
  if (token === VALID_TOKEN) {
    return {
      id: 'user-1',
      email: 'dev@example.com',
      plan: 'free',
      quota_used: store.summarizeCount,
      quota_limit: store.quotaLimit
    };
  }
  return null;
}

function mockSummary(title, lang) {
  const summaries = {
    zh: `这是关于 "${title}" 的 AI 摘要。文章讨论了相关技术细节、背景以及可能的影响。`,
    en: `This is an AI summary of "${title}". The article discusses relevant technical details, background, and potential implications.`
  };
  const keyPoints = lang === 'zh'
    ? ['核心观点一', '核心观点二', '核心观点三']
    : ['Key point one', 'Key point two', 'Key point three'];
  return { summary: summaries[lang] || summaries.en, key_points: keyPoints };
}

const server = http.createServer((req, res) => {
  corsHeaders(res);
  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }

  const parsed = url.parse(req.url, true);
  const pathname = parsed.pathname;

  if (pathname === '/health') {
    sendJSON(res, 200, { status: 'ok', version: '1.1.0' });
    return;
  }

  if (pathname === '/auth/anonymous') {
    const token = 'dev-token';
    store.users.set(token, { id: 'anon-' + Date.now(), plan: 'free', quota_used: 0, quota_limit: 50 });
    sendJSON(res, 200, { token });
    return;
  }

  if (pathname === '/auth/me') {
    const user = getAuthUser(req);
    if (!user) return sendJSON(res, 401, { error: 'Unauthorized' });
    sendJSON(res, 200, user);
    return;
  }

  const user = getAuthUser(req);
  if (!user) return sendJSON(res, 401, { error: 'Unauthorized' });

  let body = '';
  req.on('data', chunk => body += chunk);
  req.on('end', () => {
    let json = {};
    try { if (body) json = JSON.parse(body); } catch (e) {}

    if (pathname === '/api/summarize/article' && req.method === 'POST') {
      if (store.summarizeCount >= store.quotaLimit) {
        return sendJSON(res, 429, {
          error: 'quota exceeded',
          quota_used: store.summarizeCount,
          quota_limit: store.quotaLimit,
          upgrade_url: 'https://hn-reader.app/upgrade'
        });
      }
      const { hn_id, title, lang = 'zh' } = json;
      const key = `${user.id}:${hn_id}:${lang}`;
      if (store.articles.has(key)) {
        return sendJSON(res, 200, { ...store.articles.get(key), cached: true });
      }
      store.summarizeCount++;
      const result = { ...mockSummary(title, lang), cached: false };
      store.articles.set(key, result);
      return sendJSON(res, 200, result);
    }

    if (pathname === '/api/summarize/comment' && req.method === 'POST') {
      if (store.summarizeCount >= store.quotaLimit) {
        return sendJSON(res, 429, {
          error: 'quota exceeded',
          quota_used: store.summarizeCount,
          quota_limit: store.quotaLimit,
          upgrade_url: 'https://hn-reader.app/upgrade'
        });
      }
      const { hn_id, text, lang = 'zh' } = json;
      const key = `${user.id}:${hn_id}:${lang}`;
      if (store.comments.has(key)) {
        return sendJSON(res, 200, { summary: store.comments.get(key).summary, cached: true });
      }
      store.summarizeCount++;
      const summary = lang === 'zh'
        ? `评论摘要：用户表达了关于该话题的看法。`
        : `Comment summary: the user expressed an opinion on this topic.`;
      store.comments.set(key, { summary });
      return sendJSON(res, 200, { summary, cached: false });
    }

    if (pathname === '/api/summarize/article/preload' && req.method === 'POST') {
      const { articles = [] } = json;
      return sendJSON(res, 200, { queued: articles.length });
    }

    if (pathname === '/api/summarize/article/preload-status' && req.method === 'GET') {
      const ids = (parsed.query.ids || '').split(',').filter(Boolean).map(Number);
      return sendJSON(res, 200, { ready_ids: ids.slice(0, Math.floor(ids.length / 2)) });
    }

    if (pathname === '/api/signal' && req.method === 'POST') {
      store.signals.push({ user_id: user.id, ...json });
      return sendJSON(res, 200, { ok: true });
    }

    if (pathname === '/api/signal/stats' && req.method === 'GET') {
      const userSignals = store.signals.filter(s => s.user_id === user.id);
      const byType = {};
      userSignals.forEach(s => { byType[s.signal_type] = (byType[s.signal_type] || 0) + 1; });
      return sendJSON(res, 200, { total_signals: userSignals.length, by_type: byType });
    }

    if (pathname === '/api/preference/profile' && req.method === 'GET') {
      return sendJSON(res, 200, { topics: ['rust', 'llm', 'go'], tech_stacks: [], authors: [], updated_at: new Date().toISOString() });
    }

    if (pathname === '/api/match/batch' && req.method === 'POST') {
      const { articles = [] } = json;
      const matches = articles.map(a => ({
        hn_id: a.hn_id,
        match_type: 'none',
        match_tags: [],
        match_reason: ''
      }));
      return sendJSON(res, 200, { matches });
    }

    if (pathname === '/api/recommendations' && req.method === 'GET') {
      return sendJSON(res, 200, { recommendations: [] });
    }

    sendJSON(res, 404, { error: 'Not found' });
  });
});

server.listen(PORT, () => {
  console.log(`[HN Reader Mock Cloud] listening on http://localhost:${PORT}`);
  console.log(`Valid token: ${VALID_TOKEN}`);
});
