#!/usr/bin/env node
/**
 * 校验扩展的语言包（extension/_locales）：
 *   1. 各语言包的 key 完全一致、无重复 key；
 *   2. manifest.default_locale 有对应的语言目录；
 *   3. message 非空，占位符声明与 message 里的 $name$ 一一对应，content 是 $1..$9；
 *   4. HTML 的 data-i18n*（单/双引号）、JS 里的 t()/getMessage() 字面量、
 *      manifest 的 __MSG_x__ 引用的 key 都存在。
 *
 * 用法：node scripts/check-i18n.mjs
 */
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const extensionDir = join(root, 'extension');
const localesDir = join(extensionDir, '_locales');

const problems = [];

function readJson(file) {
  try {
    return JSON.parse(readFileSync(file, 'utf8'));
  } catch (err) {
    problems.push(`${file}: JSON 解析失败 - ${err.message}`);
    return null;
  }
}

function loadCatalogs() {
  const names = readdirSync(localesDir)
    .filter((name) => statSync(join(localesDir, name)).isDirectory())
    .sort();
  if (names.length === 0) {
    problems.push('_locales 下没有任何语言目录');
  }

  const catalogs = {};
  for (const name of names) {
    const file = join(localesDir, name, 'messages.json');
    const raw = readFileSync(file, 'utf8');
    const catalog = readJson(file);
    if (!catalog) continue;
    catalogs[name] = catalog;

    // JSON.parse 会静默保留最后一个重复 key，这里按本仓库的排版（顶格两空格）扫一遍。
    const seen = new Set();
    for (const match of raw.matchAll(/^ {2}"([A-Za-z0-9_]+)"\s*:\s*\{/gm)) {
      if (seen.has(match[1])) problems.push(`${name}: 重复 key ${match[1]}`);
      seen.add(match[1]);
    }

    for (const [key, entry] of Object.entries(catalog)) {
      if (!entry || typeof entry.message !== 'string' || entry.message.trim() === '') {
        problems.push(`${name}/${key}: message 为空`);
        continue;
      }

      const declared = Object.keys(entry.placeholders || {});
      for (const placeholder of declared) {
        if (!entry.message.includes(`$${placeholder}$`)) {
          problems.push(`${name}/${key}: 声明了占位符 ${placeholder}，但 message 里没有 $${placeholder}$`);
        }
      }
      for (const match of entry.message.matchAll(/\$([A-Za-z0-9_]+)\$/g)) {
        if (!declared.includes(match[1])) {
          problems.push(`${name}/${key}: message 里的 $${match[1]}$ 没有在 placeholders 里声明`);
        }
      }
      for (const [placeholder, spec] of Object.entries(entry.placeholders || {})) {
        if (!/^\$[1-9]$/.test(String(spec?.content || ''))) {
          problems.push(`${name}/${key}: 占位符 ${placeholder} 的 content 必须是 $1..$9，实际是 ${spec?.content}`);
        }
      }
    }
  }
  return { names, catalogs };
}

function checkKeyParity(names, catalogs) {
  const [reference, ...rest] = names;
  const referenceKeys = new Set(Object.keys(catalogs[reference] || {}));
  for (const name of rest) {
    const keys = new Set(Object.keys(catalogs[name] || {}));
    for (const key of referenceKeys) {
      if (!keys.has(key)) problems.push(`${name}: 缺少 key ${key}（${reference} 里有）`);
    }
    for (const key of keys) {
      if (!referenceKeys.has(key)) problems.push(`${name}: 多出 key ${key}（${reference} 里没有）`);
    }
  }
}

function checkDefaultLocale(manifest, names) {
  const defaultLocale = manifest?.default_locale;
  if (!defaultLocale) {
    problems.push('manifest.json: 有 _locales 目录时必须声明 default_locale');
  } else if (!names.includes(defaultLocale)) {
    problems.push(`manifest.json: default_locale=${defaultLocale}，但没有 _locales/${defaultLocale}/messages.json`);
  }
}

function collectSourceFiles(dir) {
  const files = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === '_locales') continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectSourceFiles(full));
    } else if (/\.(html|js)$/.test(entry.name)) {
      files.push(full);
    }
  }
  return files;
}

function collectUsedKeys() {
  const used = new Map();
  const add = (key, file) => {
    if (!used.has(key)) used.set(key, file);
  };

  for (const file of collectSourceFiles(extensionDir)) {
    const rel = relative(extensionDir, file);
    const text = readFileSync(file, 'utf8');
    if (file.endsWith('.html')) {
      for (const match of text.matchAll(/data-i18n(?:-html|-title)?\s*=\s*(?:"([^"]+)"|'([^']+)')/g)) {
        add(match[1] || match[2], rel);
      }
    } else {
      for (const match of text.matchAll(/\bt\(\s*(?:"([^"]+)"|'([^']+)')/g)) {
        add(match[1] || match[2], rel);
      }
      for (const match of text.matchAll(/\.getMessage\(\s*(?:"([^"]+)"|'([^']+)')/g)) {
        add(match[1] || match[2], rel);
      }
    }
  }

  const manifest = readJson(join(extensionDir, 'manifest.json'));
  for (const match of JSON.stringify(manifest || {}).matchAll(/__MSG_([A-Za-z0-9_]+)__/g)) {
    add(match[1], 'manifest.json');
  }
  return used;
}

function checkReferences(names, catalogs, used) {
  for (const [key, file] of used) {
    for (const name of names) {
      if (!catalogs[name] || !catalogs[name][key]) {
        problems.push(`${file}: 引用了 ${name} 里不存在的 key ${key}`);
      }
    }
  }
}

const manifest = readJson(join(extensionDir, 'manifest.json'));
const { names, catalogs } = loadCatalogs();
if (names.length > 0) {
  checkDefaultLocale(manifest, names);
  checkKeyParity(names, catalogs);
  const used = collectUsedKeys();
  checkReferences(names, catalogs, used);
  console.log(`语言包：${names.join(', ')}；引用 key：${used.size} 个`);
}

if (problems.length > 0) {
  console.error(`\n发现 ${problems.length} 个问题：`);
  for (const problem of problems) console.error(`  - ${problem}`);
  process.exit(1);
}

console.log('i18n 校验通过');
