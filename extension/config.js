/**
 * HN Reader 全局配置模块
 * 支持 cloud / local 双模式
 */

const CONFIG = {
  // 云服务默认地址，部署后可修改
  CLOUD_BASE_URL: 'https://hn-reader.whoareyou.ren',

  // local storage keys
  KEYS: {
    MODE: 'hn_reader_mode',
    CLOUD_TOKEN: 'hn_reader_cloud_token',
    CLOUD_EMAIL: 'hn_reader_cloud_email',
    SERVER_URL: 'hn_reader_server_url',
    LANGUAGE: 'hn_reader_language',
    AUTO_PRELOAD: 'hn_reader_auto_preload'
  },

  // 默认值
  DEFAULTS: {
    MODE: 'cloud',
    SERVER_URL: 'http://localhost:8080',
    LANGUAGE: 'zh',
    AUTO_PRELOAD: false
  }
};

/**
 * 读取当前生效配置
 * @returns {Promise<{mode: string, baseUrl: string, token: string|null, email: string|null, serverUrl: string, language: string}>}
 */
async function getReaderConfig() {
  const keys = Object.values(CONFIG.KEYS);
  const result = await chrome.storage.local.get(keys);

  const mode = result[CONFIG.KEYS.MODE] || CONFIG.DEFAULTS.MODE;
  const token = result[CONFIG.KEYS.CLOUD_TOKEN] || null;
  const email = result[CONFIG.KEYS.CLOUD_EMAIL] || null;
  const serverUrl = result[CONFIG.KEYS.SERVER_URL] || CONFIG.DEFAULTS.SERVER_URL;
  const language = result[CONFIG.KEYS.LANGUAGE] || CONFIG.DEFAULTS.LANGUAGE;

  const baseUrl = mode === 'cloud'
    ? CONFIG.CLOUD_BASE_URL
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
