/**
 * HN Reader i18n：界面语言解析、文案查找、页面替换。
 *
 * 界面语言有两种来源：
 *   - 设置 hn_reader_ui_language = 'auto'（默认）：跟随 Chrome 显示语言，
 *     直接使用 chrome.i18n；
 *   - 'en' / 'zh'：显式指定，运行时加载对应 _locales 语言包覆盖 chrome.i18n，
 *     这样用户不改浏览器语言也能切换界面。
 *
 * 该文件在 service worker（importScripts）、extension 页面（<script>）和内容
 * 脚本（manifest content_scripts）里都会先于其他扩展脚本加载。
 *
 * Markup（仅 extension 页面）：
 *   data-i18n="key"        -> textContent
 *   data-i18n-html="key"   -> innerHTML（只用于本地语言包里含 <strong>/<a> 的文案）
 *   data-i18n-title="key"  -> title 属性
 */
(function (global) {
  'use strict';

  const UI_LANGUAGE_KEY = 'hn_reader_ui_language';
  const SUPPORTED_SETTINGS = ['auto', 'en', 'zh'];

  const state = {
    setting: 'auto',
    lang: null,      // 'en' | 'zh'，实际使用的界面语言
    messages: null   // 显式指定语言时加载的语言包；auto 时为 null
  };

  function browserLocale() {
    try {
      if (typeof chrome !== 'undefined' && chrome.i18n && chrome.i18n.getUILanguage) {
        return String(chrome.i18n.getUILanguage() || '').toLowerCase();
      }
    } catch (e) {
      // 没有 chrome.i18n 时退回 navigator（例如 Node 里 require）。
    }
    try {
      return String(navigator.language || 'en').toLowerCase();
    } catch (e) {
      return 'en';
    }
  }

  /** Chrome 只有 en（默认）和 zh_CN 两套语言包，zh-TW/zh-HK 会回落到英文。 */
  function browserUiLang() {
    const locale = browserLocale();
    return locale === 'zh-cn' || locale === 'zh-sg' ? 'zh' : 'en';
  }

  function localeDir(lang) {
    return lang === 'zh' ? 'zh_CN' : 'en';
  }

  async function fetchMessages(lang) {
    const url = chrome.runtime.getURL(`_locales/${localeDir(lang)}/messages.json`);
    const resp = await fetch(url);
    if (!resp.ok) return null;
    return resp.json();
  }

  function requestMessages(lang) {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ action: 'getLocaleMessages', payload: { lang } }, (resp) => {
        if (chrome.runtime.lastError || !resp || !resp.ok) {
          resolve(null);
          return;
        }
        resolve(resp.data);
      });
    });
  }

  async function loadMessages(lang) {
    try {
      // extension 页面和 service worker 可以直接读自己的资源；
      // 内容脚本的 fetch 受页面来源限制，改由 background 代读。
      const isExtensionContext = typeof location !== 'undefined' && location.protocol === 'chrome-extension:';
      const messages = isExtensionContext ? await fetchMessages(lang) : await requestMessages(lang);
      return messages || null;
    } catch (e) {
      return null;
    }
  }

  async function initI18n() {
    let setting = 'auto';
    try {
      const result = await chrome.storage.local.get([UI_LANGUAGE_KEY]);
      setting = result[UI_LANGUAGE_KEY];
    } catch (e) {
      // storage 不可用时保持 auto。
    }
    if (!SUPPORTED_SETTINGS.includes(setting)) setting = 'auto';
    state.setting = setting;

    if (setting === 'auto') {
      state.messages = null;
      state.lang = browserUiLang();
      return state.lang;
    }

    const messages = await loadMessages(setting);
    if (messages) {
      state.messages = messages;
      state.lang = setting;
    } else {
      // 语言包读取失败时退回浏览器语言，避免界面空白。
      state.messages = null;
      state.lang = browserUiLang();
    }
    return state.lang;
  }

  let readyPromise = null;

  function ensureI18n() {
    if (!readyPromise) readyPromise = initI18n();
    return readyPromise;
  }

  function formatMessage(entry, substitutions) {
    const subs = Array.isArray(substitutions)
      ? substitutions
      : (substitutions == null ? [] : [substitutions]);

    let message = entry.message;
    for (const [name, spec] of Object.entries(entry.placeholders || {})) {
      const index = Number(String(spec.content || '').replace('$', '')) - 1;
      const value = subs[index] == null ? '' : String(subs[index]);
      message = message.split(`$${name}$`).join(value);
    }
    return message;
  }

  function translate(key, substitutions) {
    if (state.messages && state.messages[key]) {
      return formatMessage(state.messages[key], substitutions);
    }
    return chrome.i18n.getMessage(key, substitutions) || '';
  }

  function t(key, substitutions) {
    return translate(key, substitutions) || key;
  }

  function applyTranslations(root) {
    const scope = root || document;

    scope.querySelectorAll('[data-i18n]').forEach((el) => {
      const msg = translate(el.dataset.i18n);
      if (msg) el.textContent = msg;
    });

    scope.querySelectorAll('[data-i18n-html]').forEach((el) => {
      const msg = translate(el.dataset.i18nHtml);
      if (msg) el.innerHTML = msg;
    });

    scope.querySelectorAll('[data-i18n-title]').forEach((el) => {
      const msg = translate(el.dataset.i18nTitle);
      if (msg) el.title = msg;
    });

    if (typeof document !== 'undefined' && location.protocol === 'chrome-extension:') {
      document.documentElement.lang = resolvedHtmlLang();
    }
  }

  /** 摘要语言默认值跟随界面语言；用户显式设置过摘要语言时以存储值为准。 */
  function defaultSummaryLanguage() {
    const lang = state.lang || browserUiLang();
    return lang === 'zh' ? 'zh' : 'en';
  }

  /** 页面 <html lang> 应声明的语言：与实际渲染的语言包一致。 */
  function resolvedHtmlLang() {
    const lang = state.lang || browserUiLang();
    return lang === 'zh' ? 'zh-CN' : 'en';
  }

  global.UI_LANGUAGE_KEY = UI_LANGUAGE_KEY;
  global.initI18n = initI18n;
  global.ensureI18n = ensureI18n;
  global.loadLocaleMessages = loadMessages;
  global.t = t;
  global.applyTranslations = applyTranslations;
  global.defaultSummaryLanguage = defaultSummaryLanguage;
  global.resolvedHtmlLang = resolvedHtmlLang;
  global.getUiLanguageSetting = () => state.setting;

  if (typeof document !== 'undefined') {
    global.i18nReady = ensureI18n();

    const apply = () => {
      ensureI18n().then(() => applyTranslations(document));
    };
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', apply);
    } else {
      apply();
    }
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
      initI18n,
      ensureI18n,
      t,
      applyTranslations,
      defaultSummaryLanguage,
      resolvedHtmlLang
    };
  }
})(typeof globalThis !== 'undefined' ? globalThis : this);
