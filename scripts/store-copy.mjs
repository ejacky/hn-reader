#!/usr/bin/env node
/**
 * 把 docs/store-listing.md 编译成可以直接粘贴到 Chrome Web Store 后台的纯文本。
 *
 * Chrome Web Store 的描述字段不接受 markdown 或 HTML：星号、井号、反引号都会原样
 * 显示。这个脚本只做两件事——按字段切段、去掉排版符号——产出物里没有解释文字，
 * 复制哪一段就贴哪一段。
 *
 *   node scripts/store-copy.mjs docs/releases/v1.3.1          # 写 <dir>/store-copy.txt
 *   node scripts/store-copy.mjs docs/releases/v1.3.1 --stdout # 打印
 *
 * 版本说明取自该目录的 whats-new.md；summary 超过 132 字符时以非零码退出。
 */
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';

const repo = resolve(import.meta.dirname, '..');
const args = process.argv.slice(2);
const stdout = args.includes('--stdout');
const dirArg = args.find(a => !a.startsWith('--'));

if (!dirArg) {
  console.error('usage: node scripts/store-copy.mjs <release-dir> [--stdout]');
  process.exit(2);
}

const releaseDir = resolve(process.cwd(), dirArg);
const whatsNewPath = join(releaseDir, 'whats-new.md');
if (!existsSync(whatsNewPath)) {
  console.error(`FAIL no whats-new.md in ${dirArg}`);
  process.exit(2);
}

const listing = readFileSync(resolve(repo, 'docs/store-listing.md'), 'utf8');
const whatsNew = readFileSync(whatsNewPath, 'utf8');

/** 按 `## ` / `### ` 切段 */
function sections(md) {
  const out = [];
  let h2 = '';
  let h3 = '';
  let body = [];
  const flush = () => {
    if (h2 && body.some(l => l.trim())) out.push({ h2, h3, body: body.join('\n').trim() });
    body = [];
  };
  for (const line of md.split(/\r?\n/)) {
    const m3 = /^###\s+(.*)$/.exec(line);
    const m2 = /^##\s+(.*)$/.exec(line);
    if (m2 || m3) {
      flush();
      if (m2) { h2 = m2[1].trim(); h3 = ''; } else { h3 = m3[1].trim(); }
      continue;
    }
    body.push(line);
  }
  flush();
  return out;
}

/** markdown → 纯文本：保留换行与列表，去掉排版符号 */
function plain(md) {
  return md
    .split(/\r?\n/)
    .map(line => {
      let l = line;
      if (/^\s*-{3,}\s*$/.test(l)) return '';                   // markdown 分割线
      if (/^\s*\|?\s*-{2,}.*\|/.test(l)) return '';            // 表格分隔行
      if (/^\s*\|/.test(l)) {                                   // 表格行 → "键: 值"
        const cells = l.split('|').map(c => c.trim()).filter(Boolean);
        if (cells.length >= 2) return `• ${cells[0]}: ${cells.slice(1).join(' — ')}`;
      }
      l = l.replace(/!\[[^\]]*\]\(([^)]+)\)/g, '$1');            // 图片 → url
      l = l.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '$1 ($2)');      // 链接 → 文字 (url)
      l = l.replace(/\*\*|__|`/g, '');                           // 粗体 / 行内代码
      l = l.replace(/^\s*[-*+]\s+/, '• ');                       // 列表符号
      l = l.replace(/^\s*>\s?/, '');                             // 引用
      return l.replace(/\s+$/, '');
    })
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

const secs = sections(listing);
const pick = (h2part, h3part = '') =>
  secs.find(s => s.h2.includes(h2part) && (h3part ? s.h3.includes(h3part) : !s.h3));

const summary = plain((pick('商店描述', 'Short description') || {}).body || '');
const detailed = plain((pick('商店描述', 'Detailed description') || {}).body || '');
const singlePurpose = plain((pick('单一用途说明') || {}).body || '');
const remoteCode = plain((pick('远程代码声明') || {}).body || '');
const perms = secs
  .filter(s => s.h2.includes('权限说明'))
  .map(s => ({ name: s.h3.replace(/`/g, ''), text: plain(s.body) }))
  .filter(p => p.name);

const fences = [...whatsNew.matchAll(/```[a-z]*\r?\n([\s\S]*?)```/g)].map(m => m[1].trim());
const [whatsNewEn, whatsNewZh] = [fences[0] || '', fences[1] || ''];

const bar = '='.repeat(70);
const block = (title, note, text) =>
  [bar, title, note ? `(${note})` : '', bar, text, ''].join('\n');

const out = [
  block('SHORT DESCRIPTION', `${[...summary].length} / 132 characters`, summary),
  block('DETAILED DESCRIPTION', 'plain text: markdown 在这里会原样显示', detailed),
  block('SINGLE PURPOSE', '', singlePurpose),
  ...perms.map(p => block(`PERMISSION JUSTIFICATION - ${p.name}`, '', p.text)),
  block('REMOTE CODE', '后台选 No；下面这段是给审核看的说明', remoteCode),
  block("WHAT'S NEW (English)", `${[...whatsNewEn].length} characters`, whatsNewEn),
  block("WHAT'S NEW (Chinese)", `${[...whatsNewZh].length} characters`, whatsNewZh),
].join('\n');

const outPath = join(releaseDir, 'store-copy.txt');
if (stdout) {
  process.stdout.write(out);
} else {
  mkdirSync(dirname(outPath), { recursive: true });
  writeFileSync(outPath, out);
  console.log(`written: ${outPath}`);
}

const len = [...summary].length;
if (len > 132) {
  console.error(`FAIL short description is ${len} chars, ${len - 132} over the 132 cap`);
  process.exit(1);
}
console.log(`summary: ${len}/132 characters`);
