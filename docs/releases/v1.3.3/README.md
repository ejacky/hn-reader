# v1.3.3 发版材料（准备提交）

2026-11-08 准备。扩展版本 `1.3.3`；云服务无代码变更，本次**不部署**，生产 `/health` 保持 `1.3.2`。

## 状态

| 材料 | 状态 | 位置 |
|---|---|---|
| 安装包 | ⏳ 由 `release-artifacts` 在 tag 后产出 | Release `v1.3.3` 的 `HNReader-extension-1.3.3.zip` |
| 版本号 | ✅ `1.3.3` | [extension/manifest.json](../../../extension/manifest.json) |
| 摘要 | ✅ 来自软件包（未改） | `store-copy.txt` 的 Summary |
| 商店截图 | ⚠️ 沿用 v1.3.1，**未重拍** | [`../v1.3.1/screenshots/`](../v1.3.1/screenshots/) |
| 新版本说明 | ✅ | [`whats-new.md`](whats-new.md) |
| 纯文本上架稿 | ✅ | [`store-copy.txt`](store-copy.txt) |
| 中文条目详细描述 | ✅ 沿用商店现有内容 | 如无变化可不动 |
| 合规核对 | ⏳ 见 #24 | 商店数据披露/隐私政策链接需先对齐 |
| 部署记录 | — | 云服务不变，无需部署 |

## 本次变化

- #23 popup header 增加「打开 Hacker News」入口。
- #22 语言设置默认折叠，折叠行显示当前界面/摘要语言，记住展开状态。
- #16 `zh-TW` / `zh-HK` / `zh-Hant` 使用中文语言包，不再掉到英文。
- #3 统计封存到低权重 Stats 按钮，点开才请求。

## 待办

1. **商店提交前**完成 #24 的合规核对（数据披露 + 隐私政策链接）。
2. **截图重拍**：popup 首屏已变化，现有截图仍是 v1.3.1 的；建议三张统一 1280×800、云模式。
3. tag 后核对 Release 资产、`v1.3.3` milestone 与 #10 的 mark-release 记录。
4. 商店审核通过后，更新本目录状态与商店“上次更新日期”。

## 备注

- 本次是扩展发布，不带云服务二进制；Release 里的 Windows 安装包由 tag 构建，内含本地服务端 1.3.3。
- 商店摘要由软件包 `extDescription` 自动读取，本次未改摘要，因此无需改 listing 摘要。
