# Store Listing Materials

> 本文件存放 Chrome Web Store / Edge Add-ons 上架时需要填写的文字内容。  
> 截图素材见 `../assets/` 目录下的 `screen1.png`、`screen2.png`、`screen3.png`。  
> 隐私政策见 `PRIVACY.md`。

---

## 基本信息

| 字段 | 内容 |
|------|------|
| **名称** | HN Reader - AI Reading Assistant |
| **版本** | 1.0.1（与 `manifest.json` 保持一致） |
| **类别** | Productivity |
| **语言** | English（默认） |

---

## 商店描述（Description）

### 简短描述（Short Description）

Read Hacker News faster with AI summaries and personalized highlights. Your data stays local.

### 详细描述（Detailed Description）

Stop wasting time on Hacker News articles that aren't for you.

HN Reader is an AI-powered reading assistant that helps you read Hacker News faster and smarter — without ever leaving the page.

**What you get:**

⏱️ **Save Time**  
Get instant AI summaries for articles and comments with one click. Know if a post is worth your time before you open it.

🎯 **Discover What Matters**  
The extension learns from your reading habits and automatically highlights articles matched to your interests. No more scrolling through noise.

🔒 **Complete Privacy**  
Your data stays on your machine. The extension connects to your own self-hosted server using your own API key. Nothing is sent to us or any third party.

**How it works:**
1. Install the extension and download the free HN Reader server (runs locally on your computer).
2. Browse Hacker News normally — you'll see `[summary]` buttons next to posts and comments.
3. Click to get instant AI insights. Articles you'll love are highlighted automatically.

**Permissions:**
- **Storage** — remembers your server address and language preference.
- **news.ycombinator.com** — injects summary buttons on Hacker News only.

⚠️ **Requires a self-hosted local server.** Download it from the GitHub releases page.

---

## 单一用途说明（Single Purpose）

This extension has a single purpose: to enhance the Hacker News reading experience by providing AI-generated summaries and personalized content recommendations based on user reading behavior.

All functionality — article summaries, comment summaries, behavior tracking, and recommendations — serves this single purpose of improving the user's Hacker News browsing experience.

---

## 权限说明（Permissions Justification）

### `storage`

**Why needed:** The extension uses `chrome.storage.local` to persist two user-configured settings locally on the device:
1. The user's self-hosted HN Reader server URL
2. The preferred summary language (Chinese or English)

**Why required permission:** This data must be retained across browser sessions. It is stored locally via Chrome's `storage` API and never leaves the user's device.

### `host_permissions: https://news.ycombinator.com/*`

**Why needed:** This host permission is required for our content scripts (`content-list.js` and `content-item.js`) to inject AI-powered summary buttons and personalized article highlights directly into Hacker News pages.

**Why required permission:** The extension only runs on `news.ycombinator.com` and does not access any other websites.

---

## 远程代码声明（Remote Code）

This extension does **not** use any remote code. All JavaScript code is bundled within the extension package.

The extension communicates only with the user's self-hosted local server (configured via the popup UI, default `http://localhost:8080`). Communication is limited to data exchange via JSON API calls (`fetch`), not remote code execution. No external scripts are loaded or evaluated.

---

## 开发者信息

| 字段 | 内容 |
|------|------|
| **开发者/公司名** | （填写你的名称） |
| **网站** | https://github.com/ejacky/hn-reader |
| **隐私政策链接** | https://github.com/ejacky/hn-reader/blob/main/PRIVACY.md |
| **支持邮箱** | （可选） |

---

## 审核历史

### 2026-06-03 — 首次审核被拒绝（Purple Potassium）

**拒绝原因：** 请求但未使用 `activeTab` 权限。审核员要求全面检查权限，确保所有声明的权限都是当前功能所必需的。

**修改内容：**
- 从 `manifest.json` 的 `permissions` 中移除了未使用的 `activeTab`
- 从 `manifest.json` 的 `host_permissions` 中移除了 `http://localhost/*`
- 版本号从 `1.0.0` 提升至 `1.0.1`
- 同步更新了商店描述，强化价值主张和权限说明

**重新提交注意事项：**
- 权限理由需明确说明 `storage` 和 `news.ycombinator.com/*` 的用途
- 描述中需突出扩展需配合本地服务使用
- Remote Code 回答 `No`

---

## 上架检查清单

- [x] `manifest.json` 版本号已更新（`1.0.1`）
- [x] 未使用权限已移除（`activeTab`、`http://localhost/*`）
- [ ] 截图已上传（`../assets/screen1.png`、`screen2.png`、`screen3.png`）
- [ ] 商店图标已打包（`extension/icons/` 16×16, 48×48, 128×128）
- [ ] 隐私政策链接可用（GitHub 上的 `PRIVACY.md`）
- [ ] 单一用途说明已填写
- [ ] 权限理由已填写
- [ ] 远程代码声明已确认（`No`）
- [ ] 发布说明已更新（`RELEASE_NOTES.md`）
