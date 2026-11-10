# v1.3.3 发版材料（已上线）

2026-11-08 准备，2026-11-10 商店审核通过并上线。扩展版本 `1.3.3`；云服务无代码变更，本次**不部署**，生产 `/health` 保持 `1.3.2`。

## 状态

| 材料 | 状态 | 位置 |
|---|---|---|
| 安装包 | ✅ | Release `v1.3.3` 的 `HNReader-extension-1.3.3.zip` / `HNReader-Setup-1.3.3.exe` |
| 版本号 | ✅ `1.3.3` | [extension/manifest.json](../../../extension/manifest.json) |
| 摘要 | ✅ 来自软件包（未改） | `store-copy.txt` 的 Summary |
| 商店截图 | ✅ 新增 1280×800 云模式截图 | [`screenshots/jt1.png`](screenshots/jt1.png) |
| 新版本说明 | ✅ | [`whats-new.md`](whats-new.md) |
| 纯文本上架稿 | ✅ | [`store-copy.txt`](store-copy.txt) |
| 中文条目详细描述 | ✅ 沿用商店现有内容 | 如无变化可不动 |
| 合规核对 | ✅ 隐私政策页已是云模式版本（2026-11-08） | https://ejacky.github.io/hn-reader/ ；数据披露以商店后台为准 |
| 商店上线 | ✅ 1.3.3 审核通过（2026-11-10） | 商店页面版本 `1.3.3` |
| 部署记录 | — | 云服务不变，无需部署；见下方发布记录 |

## 本次变化

- #23 popup header 增加「打开 Hacker News」入口。
- #22 语言设置默认折叠，折叠行显示当前界面/摘要语言，记住展开状态。
- #16 `zh-TW` / `zh-HK` / `zh-Hant` 使用中文语言包，不再掉到英文。
- #3 统计封存到低权重 Stats 按钮，点开才请求。

## 发布记录

- tag：`v1.3.3`（2026-11-08）
- Release：https://github.com/ejacky/hn-reader/releases/tag/v1.3.3
- release-artifacts：run `42429991554`，`package` 与 `mark-release` 均 success
- milestone：`v1.3.3`（#22/#16/#3/#23 归入）
- 资产校验：`SHA256SUMS` 两项均 OK；扩展包 `manifest.json` 为 `1.3.3`，host 权限仅 HN 与云 API
- 本地备份：`artifacts/v1.3.3/`（gitignored）
- 商店提交：2026-11-08 提交 1.3.3 审核；2026-11-10 审核通过，商店页面版本更新为 `1.3.3`

## 待办

无。审核已通过，商店版本与包内版本一致（`1.3.3`）。

## 备注

- 本次是扩展发布，不带云服务二进制；Release 里的 Windows 安装包由 tag 构建，内含本地服务端 1.3.3。
- 商店摘要由软件包 `extDescription` 自动读取，本次未改摘要，因此无需改 listing 摘要。
