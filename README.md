# HN Reader

**AI summaries and personalized highlights for Hacker News.** A Chrome extension that puts a `[summary]` link next to every story and comment, plus a Go backend — run it hosted, or self-hosted.

[Install for Chrome](https://chromewebstore.google.com/detail/hn-reader-ai-reading-assi/bjjgpeekaedbhcpglinhfcdkglmfmbkm) · [Landing page](https://hn-reader.whoareyou.ren) · [Privacy](docs/PRIVACY.md) · [Release notes](docs/RELEASE_NOTES.md)

通过浏览器扩展 + 后端服务，在 Hacker News 页面注入 AI 总结按钮，并基于用户的隐式行为建立兴趣模型，辅助你判断哪些内容值得阅读。

## 核心功能

- **文章总结**: 在 HN 列表页每篇文章旁点击 `[summary]`，获取 AI 生成的摘要和关键要点
- **评论总结**: 在评论页为每条评论生成核心观点摘要
- **行为追踪**: 自动收集阅读行为（点击文章、查看评论、收藏），形成兴趣画像
- **列表页匹配**: 根据你的阅读偏好，在列表页自动高亮可能感兴趣的文章
- **个性化推荐**: 在扩展弹窗中查看为你推荐的文章
- **双模式支持**: 云服务即装即用 / 本地服务数据自主可控

## 项目结构

```
hn-reader/
├── extension/          # Chrome 扩展 (Manifest V3)
│   ├── manifest.json
│   ├── content-list.js     # 列表页注入
│   ├── content-item.js     # 评论页注入
│   ├── background.js       # 统一 API 代理（支持 cloud/local 双模式）
│   ├── popup.html / popup.js
│   ├── welcome.html / welcome.js
│   ├── config.js           # 全局配置模块
│   ├── i18n.js             # 界面语言判定 + 文案替换（data-i18n）
│   ├── _locales/           # 语言包：en（默认）/ zh_CN
│   └── icons/
├── server/             # Go 后端（local + cloud 共用）
│   ├── cmd/hn-reader/          # 本地服务入口
│   ├── cmd/hn-reader-cloud/    # 云服务入口（必须 -tags cloud）
│   └── internal/
│       ├── api/        # HTTP API + 云鉴权/配额
│       ├── fetcher/    # 网页内容抓取
│       ├── summarizer/ # LLM 总结
│       ├── storage/    # SQLite / PostgreSQL 数据层
│       ├── preloader/  # 预加载
│       ├── site/       # 落地页（go:embed，随云服务二进制发布）
│       └── config/     # 配置管理
└── deploy/h43/         # 生产部署（docker-compose / Caddy / 备份）
└── deploy/h43-test/    # staging 部署（test.hn-reader.whoareyou.ren，CI 自动部署）
```

## 快速开始

### 方式一：云服务（推荐）

1. 安装 Chrome 扩展（开发者模式加载 `./extension` 文件夹）
2. 扩展会自动为你创建匿名云账户，无需手动配置
3. 访问 [news.ycombinator.com](https://news.ycombinator.com)，点击文章旁的 `[summary]` 即可使用
4. 每月免费额度用完后，扩展会引导你升级到 Pro

> 当前云服务域名：`https://hn-reader.whoareyou.ren`（同域 `/` 是产品落地页，`/api/*` 是扩展调用的接口；
> staging 为 `https://test.hn-reader.whoareyou.ren`，联调时用 `hn_reader_cloud_url` 覆盖键切换，
> 见 [deploy/h43-test/README.md](deploy/h43-test/README.md)）

### 方式二：本地服务

适合注重隐私、希望数据完全本地化的用户。

1. 配置 `.env`：

```bash
cd server
copy .env.example .env
```

编辑 `.env`：

```env
LLM_API_KEY=your-api-key-here
LLM_MODEL=gpt-4o-mini
```

2. 启动本地服务：

```bash
cd server
go build -o hn-reader.exe .
.\hn-reader.exe
```

3. 扩展中选择「使用本地服务」，输入 `http://localhost:8080`

### 使用

- 访问 [news.ycombinator.com](https://news.ycombinator.com)，每篇文章旁会出现 `[summary]` 按钮
- 点击按钮获取 AI 摘要
- 点击进入文章 → 自动记录为感兴趣
- 进入评论页 → 每条评论旁有 `[summarize]` 按钮
- 点击 favorite → 记录最强兴趣信号
- 列表页中，橙色边框表示可能感兴趣的文章

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务端口 |
| `LLM_BASE_URL` | `https://api.openai.com/v1` | LLM API 地址 |
| `LLM_API_KEY` | - | LLM API Key |
| `LLM_MODEL` | `gpt-4o-mini` | 模型名称 |
| `DB_PATH` | `./hn-reader.db` | SQLite 数据库路径 |

云服务额外配置（限流、LLM 日预算/并发、预加载队列、可信代理等）见 [deploy/h43/.env.example](deploy/h43/.env.example)。

## 开发

```bash
# 编译后端
make build

# 启动后端开发模式
make dev

# 清理构建产物
make clean
```

## Cloud Server 本地开发

云服务代码在 `server/cmd/hn-reader-cloud`，**必须带 `-tags cloud` 构建**；不带 tag 会因构建约束直接失败，避免误部署出无鉴权服务。

```bash
cd server

# 1. 准备 PostgreSQL，并设置环境变量
export DATABASE_URL=postgres://hn_reader:hn_reader@localhost:5432/hn_reader?sslmode=disable
export LLM_API_KEY=your-api-key

# 2. 启动云服务（启动时自动建表）
go build -tags cloud -o cloud-server.exe ./cmd/hn-reader-cloud
./cloud-server.exe
```

测试扩展 cloud 模式前，修改 `extension/config.js` 中的 `CLOUD_BASE_URL`：

```js
CLOUD_BASE_URL: 'http://localhost:8080'
```

## Cloud Server 部署

当前生产部署：`https://hn-reader.whoareyou.ren`（h43，PostgreSQL + Go + Caddy）。

完整的容器、Caddy、网络、备份与回滚说明见 [deploy/h43/README.md](deploy/h43/README.md)。

staging：`https://test.hn-reader.whoareyou.ren`，与生产同机但完全隔离（独立库/容器/网络别名/IP 白名单），
推 `main`（合并即部署）或 `dev`/`fix/**`/`feat/**`（合并前预验）由 GitHub Actions 自动部署，见 [deploy/h43-test/README.md](deploy/h43-test/README.md)。

## 技术栈

- **本地后端**: Go + Gin + SQLite (modernc.org/sqlite)
- **云后端**: Go + Gin + PostgreSQL + Docker
- **扩展**: Chrome Extension Manifest V3 + Vanilla JS
- **LLM**: OpenAI 兼容 API (GPT-4o-mini 等)

## 隐私说明

### 本地模式
- 所有用户数据存储在本地 SQLite 数据库
- LLM 调用仅发送文章内容，不含用户标识
- API Key 由用户自行配置，不经过第三方服务

### 云服务模式
- 匿名 token 标识账户，不收集姓名/邮箱/密码
- 阅读信号、按用户隔离的摘要记录和配额存储在 PostgreSQL
- 原始网页全文不长期存储；生成的摘要在共享缓存中按 `(hn_id, lang)` 保存，不含用户标识
- 文章/评论文本仅发送给配置的 LLM 提供商用于生成摘要
- 数据不出售、不用于广告或训练我们自己的模型
- 弹窗内支持导出数据和删除云端账户；删除后用户数据级联清除，共享公开摘要缓存保留
- 详细说明见 [docs/PRIVACY.md](docs/PRIVACY.md)

## License

MIT，见 [LICENSE](LICENSE)。可自由使用、修改、分发（含商用），保留版权声明即可。
