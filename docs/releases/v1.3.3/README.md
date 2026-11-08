# v1.3.3 发版材料（制品已发布，已提交商店审核）

2026-11-08 准备。扩展版本 `1.3.3`；云服务无代码变更，本次**不部署**，生产 `/health` 保持 `1.3.2`。

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
- 商店提交：2026-11-08 提交 1.3.3 审核，等待结果（审核通过前商店页仍显示 1.3.2）

## 待办

1. 等待商店审核结果（2026-11-08 提交）。通过后更新本目录状态、商店“上次更新日期”和 `store-listing.md` 的「审核历史」，再关闭 #25。
2. #24 的数据用途披露以商店后台为准；审核若提出合规问题，按后台提示改。
3. 审核通过后把 #24 / #25 置 Done。

## 备注

- 本次是扩展发布，不带云服务二进制；Release 里的 Windows 安装包由 tag 构建，内含本地服务端 1.3.3。
- 商店摘要由软件包 `extDescription` 自动读取，本次未改摘要，因此无需改 listing 摘要。
