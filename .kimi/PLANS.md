# HN Reader - 项目计划

> 本项目通过浏览器扩展 + Go 后端，在 Hacker News 页面注入 AI 总结按钮，并基于用户隐式行为建立兴趣模型。
> 原始 Kimi 计划文件见 `~/.kimi/plans/orion-mister-terrific-metamorpho.md`

## 架构

```
浏览器扩展 (MV3)  ←→  Go 后端 (:8080)  ←→  LLM API
     │                      │
     └─ 行为信号 ────────────┘
            ↓
        SQLite (本地)
```

## 数据模型

- `articles` - 文章元数据、抓取内容、LLM 总结
- `comments` - 评论元数据、总结
- `user_signals` - 用户行为信号（5 级强度）

## 实现阶段

### ✅ Phase 1: 列表页文章总结
- [x] 扩展框架（Manifest V3）
- [x] Go 后端服务（Gin + SQLite）
- [x] 内容抓取（goquery）
- [x] LLM 总结（OpenAI 兼容 API）
- [x] `[summary]` 按钮 + 浮动卡片（fixed 定位）
- [x] 行为信号收集（点击文章 → `article_clicked`）

### 🔄 Phase 2: 评论页总结 + 完整行为追踪
- [ ] 评论页 `item?id=` 注入 `[summarize]` 按钮
- [ ] 评论内容获取（HN Firebase API）
- [ ] 评论 LLM 总结
- [ ] 评论行为追踪（查看评论、点击 favorite）

### 📋 Phase 3: 列表页实时兴趣匹配 + 推荐
- [ ] 用户偏好提取（关键词 TF-IDF）
- [ ] 列表页批量匹配（标题 vs 用户画像）
- [ ] 视觉提示（左边框高亮 + 主题标签）
- [ ] 个性化总结 Prompt（注入用户画像）
- [ ] Popup 推荐标签页

### 📋 Phase 4: 部署优化
- [ ] Makefile `make dev`
- [ ] Docker Compose
- [ ] 性能优化（缓存、预加载）

## 环境配置

后端通过 `server/.env` 文件配置：

```env
LLM_API_KEY=sk-...
LLM_MODEL=gpt-4o-mini
PORT=8080
```

## 目录结构

```
hn-reader/
├── extension/          # Chrome 扩展
│   ├── manifest.json
│   ├── content-list.js     # 列表页
│   ├── content-item.js     # 评论页
│   ├── background.js       # Service Worker 代理
│   ├── popup.html/js/css
│   └── icons/
├── server/             # Go 后端
│   ├── main.go
│   ├── go.mod
│   ├── .env
│   └── internal/
│       ├── api/
│       ├── fetcher/
│       ├── summarizer/
│       ├── storage/
│       └── config/
├── Makefile
├── README.md
└── .kimi/PLANS.md      # 本文件
```

## 当前状态

文章总结功能已完成，卡片使用 `position: fixed` 显示在按钮下方，避免被 HN 原有 `overflow: hidden` 截断。
