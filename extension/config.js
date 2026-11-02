/**
 * HN Reader 全局配置模块
 * 支持 cloud / local 双模式
 */

/**
 * 摘要语言的默认值由 i18n.js 的 defaultSummaryLanguage() 提供（与界面语言一致）。
 * 用户显式选择过语言时以存储值为准（见 getReaderConfig）。
 */
const CONFIG = {
  // 云服务默认地址（生产）。联调 staging 时不要改这里，用下面的 CLOUD_URL 覆盖键。
  CLOUD_BASE_URL: 'https://hn-reader.whoareyou.ren',

  // local storage keys
  KEYS: {
    MODE: 'hn_reader_mode',
    CLOUD_TOKEN: 'hn_reader_cloud_token',
    CLOUD_EMAIL: 'hn_reader_cloud_email',
    // 可选的云端地址覆盖（staging 联调开关，非发行面功能）：
    //   await chrome.storage.local.set({ hn_reader_cloud_url: 'https://test.hn-reader.whoareyou.ren' })
    //   await chrome.storage.local.remove(['hn_reader_cloud_url', 'hn_reader_cloud_token'])
    CLOUD_URL: 'hn_reader_cloud_url',
    SERVER_URL: 'hn_reader_server_url',
    LANGUAGE: 'hn_reader_language',
    UI_LANGUAGE: 'hn_reader_ui_language',
    AUTO_PRELOAD: 'hn_reader_auto_preload'
  },

  // 默认值
  DEFAULTS: {
    MODE: 'cloud',
    SERVER_URL: 'http://localhost:8080',
    AUTO_PRELOAD: false
  }
};

/**
 * 读取当前生效配置
 * @returns {Promise<{mode: string, baseUrl: string, token: string|null, email: string|null, serverUrl: string, language: string}>}
 */
async function getReaderConfig() {
  // 摘要语言的默认值依赖界面语言（可能来自语言包设置），先确保 i18n 就绪。
  await ensureI18n();

  const keys = Object.values(CONFIG.KEYS);
  const result = await chrome.storage.local.get(keys);

  const mode = result[CONFIG.KEYS.MODE] || CONFIG.DEFAULTS.MODE;
  const token = result[CONFIG.KEYS.CLOUD_TOKEN] || null;
  const email = result[CONFIG.KEYS.CLOUD_EMAIL] || null;
  const serverUrl = result[CONFIG.KEYS.SERVER_URL] || CONFIG.DEFAULTS.SERVER_URL;
  const language = result[CONFIG.KEYS.LANGUAGE] || defaultSummaryLanguage();
  // 覆盖键只用于 staging 联调；没有设置时行为与之前完全一致。
  const cloudUrl = result[CONFIG.KEYS.CLOUD_URL] || CONFIG.CLOUD_BASE_URL;

  const baseUrl = mode === 'cloud'
    ? cloudUrl
    : (serverUrl || CONFIG.DEFAULTS.SERVER_URL);

  return { mode, baseUrl, token, email, serverUrl, language };
}

/**
 * 保存配置
 * @param {Object} cfg
 * @param {string} [cfg.mode]
 * @param {string} [cfg.cloudToken]
 * @param {string} [cfg.cloudEmail]
 * @param {string} [cfg.serverUrl]
 * @param {string} [cfg.language]
 */
async function setReaderConfig(cfg) {
  const data = {};
  if (cfg.mode !== undefined) data[CONFIG.KEYS.MODE] = cfg.mode;
  if (cfg.cloudToken !== undefined) data[CONFIG.KEYS.CLOUD_TOKEN] = cfg.cloudToken;
  if (cfg.cloudEmail !== undefined) data[CONFIG.KEYS.CLOUD_EMAIL] = cfg.cloudEmail;
  if (cfg.serverUrl !== undefined) data[CONFIG.KEYS.SERVER_URL] = cfg.serverUrl;
  if (cfg.language !== undefined) data[CONFIG.KEYS.LANGUAGE] = cfg.language;
  await chrome.storage.local.set(data);
}

/**
 * 构造请求头
 */
function buildAuthHeaders(cfg) {
  const headers = {
    'Content-Type': 'application/json'
  };
  if (cfg.mode === 'cloud' && cfg.token) {
    headers['Authorization'] = `Bearer ${cfg.token}`;
  }
  return headers;
}

// 导出给 content scripts 和 popup 使用
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { CONFIG, getReaderConfig, setReaderConfig, buildAuthHeaders };
}
