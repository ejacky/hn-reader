# HN Reader - AI 个性化阅读助手

通过浏览器扩展 + Go 后端服务，在 Hacker News 页面注入 AI 总结按钮，并基于用户的隐式行为建立兴趣模型，辅助你判断哪些内容值得阅读。

## 核心功能

- **文章总结**: 在 HN 列表页每篇文章旁点击 `[summary]`，获取 AI 生成的摘要和关键要点
- **评论总结**: 在评论页为每条评论生成核心观点摘要
- **行为追踪**: 自动收集阅读行为（点击文章、查看评论、收藏），形成兴趣画像
- **列表页匹配**: 根据你的阅读偏好，在列表页自动高亮可能感兴趣的文章
- **个性化推荐**: 在扩展弹窗中查看为你推荐的文章

## 项目结构

```
hn-reader/
├── extension/          # Chrome 扩展 (Manifest V3)
│   ├── manifest.json
│   ├── content-list.js     # 列表页注入
│   ├── content-item.js     # 评论页注入
│   ├── background.js
│   ├── popup.html
│   └── icons/
└── server/             # Go 后端服务
    ├── main.go
    └── internal/
        ├── api/        # HTTP API
        ├── fetcher/    # 网页内容抓取
        ├── summarizer/ # LLM 总结
        ├── storage/    # SQLite 数据层
        ├── model/      # 兴趣模型
        └── config/     # 配置管理
```

## 快速开始

### 1. 配置后端

复制配置文件模板并填写你的 LLM API Key：

```bash
cd server
copy .env.example .env
```

编辑 `.env` 文件：

```env
LLM_API_KEY=your-api-key-here
LLM_MODEL=gpt-4o-mini
```

后端支持从以下位置加载 `.env` 文件（按优先级）：
1. 当前工作目录的 `.env`
2. `server/.env`
3. `~/.hn-reader/.env` (用户主目录)

环境变量会覆盖 `.env` 文件中的值。

### 2. 启动后端服务

```bash
cd server
go build -o hn-reader.exe .
.\hn-reader.exe
```

后端默认运行在 `http://localhost:8080`。

### 2. 加载浏览器扩展

1. 打开 Chrome，进入 `chrome://extensions/`
2. 开启右上角「开发者模式」
3. 点击「加载已解压的扩展程序」，选择 `./extension` 文件夹
4. 点击扩展图标，配置 Server URL 和 LLM API Key

### 3. 使用

- 访问 [news.ycombinator.com](https://news.ycombinator.com)，每篇文章旁会出现 `[summary]` 按钮
- 点击按钮获取 AI 摘要
- 点击进入文章 → 自动记录为感兴趣
- 进入评论页 → 每条评论旁有 `[summarize]` 按钮
- 点击 favorite → 记录最强兴趣信号

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

## 技术栈

- **后端**: Go + Gin + SQLite (modernc.org/sqlite)
- **扩展**: Chrome Extension Manifest V3 + Vanilla JS
- **LLM**: OpenAI 兼容 API (GPT-4o-mini 等)

## 隐私说明

- 所有用户数据存储在本地 SQLite 数据库
- LLM 调用仅发送文章内容，不含用户标识
- API Key 由用户自行配置，不经过第三方服务
