# HN Reader - 个性化阅读助手

## 项目概述

通过浏览器扩展 + Go 后端服务，在 Hacker News 页面注入 AI 总结按钮，辅助用户判断内容价值。同时通过追踪用户的隐式行为（点击文章、查看评论、收藏等）建立兴趣模型，最终让总结内容本身反映个性化偏好，而非生硬的打分或标签。

## 核心交互流程

### 列表页（news.ycombinator.com 首页/分类页）
1. 每篇文章标题旁注入「总结」按钮
2. 点击后浮动展示文章摘要 + 关键要点
3. 用户若随后点击进入原文 → 后端记录 `article_clicked` 信号

### 评论页（item?id=xxx）
1. 每条评论旁注入「总结」按钮
2. 点击后展开评论核心观点摘要
3. 用户若点击评论链接/展开原始评论 → 记录 `comment_clicked` 信号
4. 用户若点击 favorite 按钮 → 记录 `favorited` 最强信号

### 信号强度层级
| 信号 | 强度 | 触发条件 |
|------|------|----------|
| summary_viewed | 1 | 点击文章/评论的总结按钮 |
| article_clicked | 3 | 从列表页点击进入原文 |
| comment_summary_viewed | 2 | 点击评论的总结按钮 |
| comment_clicked | 3 | 在评论页点击查看原始评论详情 |
| favorited | 5 | 点击 favorite 按钮 |

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                    浏览器扩展 (Manifest V3)                    │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐  │
│  │ content_script  │  │ content_script  │  │ background  │  │
│  │ _list.js        │  │ _item.js        │  │ .js         │  │
│  │ (列表页注入)      │  │ (评论页注入)      │  │ (后台通信)   │  │
│  └────────┬────────┘  └────────┬────────┘  └──────┬──────┘  │
│           │                    │                   │         │
│           ▼                    ▼                   ▼         │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              chrome.storage.local (本地缓存)               ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
                              │ HTTP API
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        Go 后端服务                             │
│  ┌──────────┐  ┌────────────┐  ┌───────────┐  ┌──────────┐  │
│  │ API Server│  │  Content   │  │  LLM      │  │ Interest │  │
│  │ (gin/echo)│  │  Fetcher   │  │  Summarizer│  │ Model    │  │
│  └────┬─────┘  └────────────┘  └───────────┘  └────┬─────┘  │
│       │                                             │         │
│       └─────────────────────────────────────────────┘         │
│                         │                                     │
│                         ▼                                     │
│              ┌─────────────────────┐                         │
│              │   SQLite 数据库      │                         │
│              │  (articles, signals) │                         │
│              └─────────────────────┘                         │
└─────────────────────────────────────────────────────────────┘
```

## 项目目录结构

```
hn-reader/
├── extension/                 # 浏览器扩展
│   ├── manifest.json
│   ├── background.js          # Service worker
│   ├── content-list.js        # 列表页 content script
│   ├── content-item.js        # 评论页 content script
│   ├── popup.html             # 扩展弹窗
│   ├── popup.js
│   ├── popup.css
│   └── icons/                 # 扩展图标
├── server/                    # Go 后端
│   ├── main.go
│   ├── go.mod
│   ├── internal/
│   │   ├── api/               # HTTP handlers
│   │   ├── fetcher/           # 网页内容抓取
│   │   ├── summarizer/        # LLM 总结
│   │   ├── storage/           # SQLite 数据层
│   │   ├── model/             # 兴趣模型
│   │   └── config/            # 配置管理
│   └── migrations/            # 数据库迁移
├── Makefile
└── README.md
```

## 数据模型

### Article 表
```sql
CREATE TABLE articles (
    id INTEGER PRIMARY KEY,
    hn_id INTEGER UNIQUE NOT NULL,
    url TEXT,
    title TEXT,
    fetched_content TEXT,       -- 抓取的正文内容
    summary TEXT,               -- LLM 生成的总结
    key_points TEXT,            -- JSON 数组，关键要点
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Comment 表
```sql
CREATE TABLE comments (
    id INTEGER PRIMARY KEY,
    hn_id INTEGER NOT NULL,     -- HN 评论 ID
    article_id INTEGER,         -- 关联文章
    author TEXT,
    raw_text TEXT,
    summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### UserSignal 表
```sql
CREATE TABLE user_signals (
    id INTEGER PRIMARY KEY,
    target_type TEXT NOT NULL,  -- 'article' | 'comment'
    target_id INTEGER NOT NULL,
    signal_type TEXT NOT NULL,  -- summary_viewed | article_clicked | comment_summary_viewed | comment_clicked | favorited
    strength INTEGER NOT NULL,
    context TEXT,               -- JSON，额外上下文（如列表页位置、时间等）
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/summarize/article` | 获取文章总结 |
| POST | `/api/summarize/comment` | 获取评论总结 |
| POST | `/api/signal` | 上报用户行为信号 |
| GET | `/api/signal/stats` | 获取用户信号统计 |
| GET | `/api/preference/profile` | 获取用户兴趣画像（用于个性化） |

## 实现阶段

### Phase 1: 基础骨架（扩展框架 + Go 服务 + 文章总结）

**目标**: 在列表页看到「总结」按钮，点击后展示文章摘要。

1. **扩展框架搭建**
   - manifest.json (Manifest V3)
   - content-list.js: 识别 HN 列表页 DOM 结构，在每篇文章行插入「总结」按钮
   - background.js: 处理跨域请求，与 Go 后端通信
   - popup: 后端地址配置、API Key 设置

2. **Go 服务骨架**
   - 初始化 go module，选用 `gin` 或标准库 `net/http`
   - 配置管理：LLM API 的 base_url、api_key、model（支持 OpenAI 兼容格式）
   - SQLite 初始化与迁移

3. **内容抓取**
   - 接收 URL，使用 `http.Client` 获取页面
   - 使用 `go-readability` 或正则+选择器提取正文
   - 限制内容长度（如前 8000 字符），避免 LLM token 超限

4. **LLM 总结**
   - Prompt 设计：提取文章核心观点、技术栈/领域、与读者的相关性维度
   - 调用 OpenAI 兼容 API
   - 结果缓存到 SQLite，避免重复调用

5. **列表页交互闭环**
   - 点击「总结」→ 请求后端 → 弹出摘要卡片（绝对定位浮层）
   - 监听文章标题的点击事件 → 发送 `article_clicked` 信号

### Phase 2: 评论页总结 + 完整行为追踪

**目标**: 在评论页为每条评论生成总结，并收集全量行为信号。

6. **评论页内容注入**
   - content-item.js: 识别 `item?id=` 页面
   - 通过 HN API (`https://hacker-news.firebaseio.com/v0/item/{id}.json`) 获取完整评论树
   - 为每条评论 DOM 旁插入「总结」按钮

7. **评论总结 API**
   - `/api/summarize/comment`: 接收评论文本，返回简短摘要（核心观点/反驳/补充信息）
   - 评论通常较短，prompt 需与文章总结区分

8. **评论行为追踪**
   - 监听评论「总结」按钮点击 → `comment_summary_viewed`
   - 监听评论「reply」或折叠/展开交互 → `comment_clicked`
   - 监听 favorite 按钮 → `favorited`
   - 注意：HN 的 favorite 按钮点击后会跳转或发送请求，需通过 MutationObserver 或拦截 fetch 捕获

### Phase 3: 列表页实时兴趣匹配 + 个性化推荐（正向循环）

**目标**: 用户无需逐一点击「总结」，在列表页就能直观看到哪些文章可能感兴趣；同时系统主动推荐相关内容，形成「阅读 → 学习偏好 → 辅助决策 → 更多阅读」的正向循环。

**正向循环数据流**:
```
用户行为(点击/收藏) → 后端更新兴趣画像 → 列表页预匹配 → 用户更快找到感兴趣内容 → 更多正向行为
```

9. **偏好提取**
   - 从用户信号关联的文章/评论中提取高频关键词、技术栈、作者
   - 简单实现：TF-IDF 或词频统计用户高互动内容的主题
   - 存储为用户画像 JSON，包含：关注主题、偏好技术栈、常读作者、内容类型偏好（教程/讨论/展示）

10. **列表页实时兴趣匹配（无需点击总结）**
    - 列表页加载时，扩展提取每篇文章的标题 + 域名，批量发送到后端
    - 后端基于用户画像计算匹配度，返回每篇文章的「匹配类型」（如：主题匹配 / 技术栈匹配 / 作者匹配 / 无匹配）
    - 扩展在文章行旁添加软性视觉提示：
      - 高度匹配：淡色左边框 + 主题标签（如 `rust` `ai`）
      - 中度匹配：仅主题标签
      - 无匹配：保持原样
    - **完全不显示分数或星级**，用「🔖 你关注的话题」等文案替代

11. **个性化总结 Prompt**
    - 在调用 LLM 时注入用户画像（如：该用户关注 AI、Rust、分布式系统）
    - Prompt 增加要求：「请从以下角度分析该文章对读者的价值...」
    - 在摘要卡片顶部增加一行：「💡 你可能关注：xxx」或类似软性提示

12. **个性化推荐引擎**
    - **站内推荐**：从 HN 其他分类（如 Ask HN / Show HN / New / Best）获取内容，筛选出与用户画像高匹配但用户未读过的文章
    - **推荐展示方式**：扩展 popup 新增「推荐」标签页，展示 5-10 条「你可能错过」的文章
    - **推荐理由**：每条推荐附带一句话说明（如「与你收藏的 Rust 文章主题相似」）
    - **时效性**：推荐内容控制在最近 7 天内，可配置

13. **正向循环强化**
    - 用户对推荐内容的互动（点击/收藏）权重加倍，快速校准模型
    - 兴趣画像自动衰减：久未互动的主题权重缓慢下降，避免画像固化
    - 提供「不感兴趣」反馈按钮（点击后降低该类推荐频率）

### Phase 4: 部署与配置

14. **本地运行体验**
    - `make dev` 同时启动 Go server 和扩展开发模式
    - 提供 `docker-compose.yml` 可选方案

15. **扩展配置**
    - popup 中配置：后端地址、LLM API Key、模型选择
    - 支持导入/导出用户信号数据

16. **性能优化**
    - 内容抓取增加超时和重试
    - 总结结果本地缓存（扩展 storage + 后端 SQLite）
    - 批量预加载：列表页可见文章提前后台获取总结

## 关键设计决策

1. **Go 后端必要性**: 浏览器扩展无法直接跨域抓取任意文章 URL，必须通过后端代理。同时 LLM API key 不应存放在扩展中。

2. **为什么不纯前端做总结**: CORS 限制 + API Key 安全 + 需要服务端存储行为数据训练模型。

3. **信号收集方式**: 使用事件监听 + `navigator.sendBeacon` 或 fetch，确保页面跳转前信号已发送。

4. **HN API vs 页面抓取**: HN 官方 Firebase API 只提供元数据，文章正文必须从原始 URL 抓取。评论内容可通过 API 获取。

5. **隐私设计**: 所有数据存储在本地 SQLite，不上传云端。LLM 调用仅发送文章内容，不含用户标识。
