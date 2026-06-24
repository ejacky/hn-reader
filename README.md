# HN Reader - AI 个性化阅读助手

通过浏览器扩展 + 后端服务，在 Hacker News 页面注入 AI 总结按钮，并基于用户的隐式行为建立兴趣模型，辅助你判断哪些内容值得阅读。

**v1.1 更新**：扩展现已支持「云服务」模式，安装后即可使用，无需下载本地服务端。注重隐私的用户仍可切换为「本地服务」模式。

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
│   └── icons/
├── server/             # Go 本地后端服务
│   ├── main.go
│   └── internal/
│       ├── api/        # HTTP API
│       ├── fetcher/    # 网页内容抓取
│       ├── summarizer/ # LLM 总结
│       ├── storage/    # SQLite 数据层
│       ├── preloader/  # 预加载
│       └── config/     # 配置管理
└── server/cloud/       # Go 云后端服务（开发中）
    ├── cmd/server/
    └── internal/
        ├── api/        # HTTP API + 认证/配额
        ├── storage/    # PostgreSQL 多租户数据层
        └── config/     # 云原生配置
```

## 快速开始

### 方式一：云服务（推荐）

1. 安装 Chrome 扩展（开发者模式加载 `./extension` 文件夹）
2. 扩展会自动为你创建匿名云账户，无需手动配置
3. 访问 [news.ycombinator.com](https://news.ycombinator.com)，点击文章旁的 `[summary]` 即可使用
4. 每月免费额度用完后，扩展会引导你升级到 Pro

> 当前云服务域名：`https://api.hn-reader.app`（部署前可修改 `extension/config.js` 中的 `CLOUD_BASE_URL`）

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

```bash
cd server/cloud

# 1. 启动 PostgreSQL（需要 Docker）
cp .env.example .env
# 编辑 .env 填入 LLM_API_KEY 和 DATABASE_URL
docker-compose up -d db

# 2. 运行 migrations（首次）并启动服务
export DATABASE_URL=postgres://hn_reader:hn_reader@localhost:5432/hn_reader?sslmode=disable
export LLM_API_KEY=your-api-key
./go.exe build -o cloud-server.exe ./cmd/server
./cloud-server.exe
```

测试扩展 cloud 模式前，修改 `extension/config.js` 中的 `CLOUD_BASE_URL`：

```js
CLOUD_BASE_URL: 'http://localhost:8080'
```

## Cloud Server 部署

推荐使用 Fly.io / Railway / Render：

1. 设置环境变量：`DATABASE_URL`, `LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL`, `PORT`
2. 部署 Docker 镜像
3. 将 `extension/config.js` 中的 `CLOUD_BASE_URL` 改为生产域名
4. 发布扩展到 Chrome Web Store

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
- 阅读信号、摘要记录按用户 ID 隔离存储在 PostgreSQL
- 原始网页全文不长期存储
- 数据不出售、不用于模型训练
- 支持导出数据和删除账户
