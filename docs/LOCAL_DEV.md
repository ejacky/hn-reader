# HN Reader 本地开发调试指南

## 概述

扩展默认连接生产云服务 `https://api.hn-reader.app`。本地调试时需要把扩展指向本地后端。

所有 API 请求都经过 `background.js` service worker 代理，因此只要 `manifest.json` 声明了本地域名权限，就能正常通信。

---

## 方式一：Mock Cloud Server（推荐，无需数据库和 LLM Key）

`server/mock-cloud/` 是一个 Node.js 模拟服务，用于快速验证扩展 cloud 模式。

### 1. 启动 mock server

```bash
cd server/mock-cloud
node server.js
```

默认监听 `http://localhost:8081`。

### 2. 修改扩展指向本地 mock

**`extension/config.js`**

```js
CLOUD_BASE_URL: 'http://localhost:8081'
```

**`extension/manifest.json`**

在 `host_permissions` 中增加本地域名：

```json
"host_permissions": [
  "https://news.ycombinator.com/*",
  "http://localhost:8081/*"
]
```

### 3. 加载扩展并测试

1. 打开 Chrome，`chrome://extensions/`
2. 开启「开发者模式」
3. 点击「加载已解压的扩展程序」，选择 `./extension`
4. 安装后会自动打开 `welcome.html`
5. 点击「去 Hacker News 试试」
6. 在 HN 页面点击任意文章旁的 `[summary]`

### 4. 验证额度用尽流程

mock server 的免费额度设置为 **3 次**。点击 3 次 `[summary]` 后，第 4 次会返回 `429 quota exceeded`，此时：

- 按钮变成 `[额度已用完]`
- 点击按钮显示升级引导浮层
- popup 中显示升级 Pro 卡片

---

## 方式二：真实 Cloud Server（完整调试，需要 PostgreSQL + LLM Key）

### 1. 启动 PostgreSQL

如果你已有 PostgreSQL，创建数据库即可：

```sql
CREATE DATABASE hn_reader;
CREATE USER hn_reader WITH PASSWORD 'hn_reader';
GRANT ALL PRIVILEGES ON DATABASE hn_reader TO hn_reader;
```

或者用 Docker：

```bash
cd server/cloud
docker-compose up -d db
```

### 2. 配置环境变量

```bash
cd server/cloud
cp .env.example .env
```

编辑 `.env`：

```env
PORT=8080
DATABASE_URL=postgres://hn_reader:hn_reader@localhost:5432/hn_reader?sslmode=disable
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=sk-your-key
LLM_MODEL=gpt-4o-mini
```

### 3. 启动 cloud-server

```bash
# Windows
cd server/cloud
/d/go-dev/bin/go.exe build -o cloud-server.exe ./cmd/server
./cloud-server.exe

# Linux / macOS
go build -o cloud-server ./cmd/server
./cloud-server
```

### 4. 修改扩展指向本地 cloud-server

**`extension/config.js`**

```js
CLOUD_BASE_URL: 'http://localhost:8080'
```

**`extension/manifest.json`**

```json
"host_permissions": [
  "https://news.ycombinator.com/*",
  "http://localhost:8080/*"
]
```

### 5. 加载扩展测试

同方式一。真实 cloud-server 会实际调用 LLM，因此摘要结果是真实的。

---

## 调试技巧

### 查看 background service worker 日志

1. 打开 `chrome://extensions/`
2. 找到 HN Reader，点击「Service Worker」链接
3. 在 DevTools Console 中查看 `background.js` 日志
4. 在 Network 面板中查看实际发出的请求

### 查看 popup 日志

1. 右键点击扩展图标
2. 选择「检查弹出内容」
3. 在 DevTools 中查看 `popup.js` 日志

### 查看 content script 日志

1. 打开 HN 页面
2. 按 F12 打开 DevTools
3. 切换到 Console
4. 筛选 `HN Reader` 相关日志

### 强制刷新扩展

修改扩展代码后，需要点击 `chrome://extensions/` 中的刷新按钮（🔄），或重新加载扩展。

---

## 常见问题

### 1. CORS 错误

如果 content script 或 popup 报 CORS 错误，检查：

- `manifest.json` 的 `host_permissions` 是否包含本地后端地址
- 本地后端是否正确返回 `Access-Control-Allow-Origin: *`

### 2. 扩展提示「Cloud account not ready」

- 检查本地后端是否已启动
- 检查 `extension/config.js` 的 `CLOUD_BASE_URL` 是否正确
- 查看 background service worker console 的详细错误

### 3. 修改代码后不生效

- 扩展代码修改后必须在 `chrome://extensions/` 中点击刷新
- 如果修改了 `manifest.json`，建议移除扩展后重新加载

### 4. 从本地切换到生产环境

发布前记得改回：

```js
// extension/config.js
CLOUD_BASE_URL: 'https://api.hn-reader.app'
```

```json
// extension/manifest.json
"host_permissions": [
  "https://news.ycombinator.com/*",
  "https://api.hn-reader.app/*"
]
```

---

## 推荐开发流程

1. 日常 UI/交互调试：使用 `server/mock-cloud`（启动快、不消耗 LLM 额度）
2. 后端逻辑/API 调试：使用 `server/cloud` + 本地 PostgreSQL
3. 集成测试：先用 mock 验证流程，再用真实 cloud-server 验证 LLM 摘要效果
