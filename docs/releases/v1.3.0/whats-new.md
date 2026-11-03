# v1.3.0 — 新版本说明

Chrome Web Store 的「What's new in this version」有字数限制，下面两版都可直接粘贴。

## English

```
HN Reader 1.3.0

• English interface, with 中文 kept for existing users — the UI now follows
  your browser language, and you can override it in Settings.
• Summary language follows the interface unless you have chosen one.
• Preload status is now visible per story: preloaded, or failed with a reason,
  instead of silently waiting.
• Summaries and preloads time out after 20 seconds instead of hanging on a
  dead connection.
• No new permissions, no change to what data is collected.
```

## 中文

```
HN Reader 1.3.0

• 界面支持英文，中文完整保留：默认跟随浏览器语言，也可以在设置里手动切换。
• 摘要语言默认跟随界面语言，你手动选过的语言保持不变。
• 每条文章的预加载状态可见：已预加载，或失败原因，不再无提示地一直转圈。
• 摘要与预加载请求 20 秒超时，不再挂在失去响应的连接上。
• 没有新增权限，收集的数据范围没有变化。
```

> 最后一行「没有新增权限」在第 1.3.0 的 manifest 里**还不成立**——`host_permissions` 多了 staging 域名，见 [README](README.md#发版前必须先决定staging-域名权限)。按选项 A 处理后这句话才准确。
