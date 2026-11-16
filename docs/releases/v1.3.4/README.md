# v1.3.4 发版材料（Phase 1：代码与材料就绪，待 Windows 实机验证后发版）

2026-11-16 准备。扩展版本 `1.3.4`；本地服务改为 per-user 安装，云服务无行为变更，本次**不部署**，生产 `/health` 保持 `1.3.2`。

## 状态

| 材料 | 状态 | 位置 |
|---|---|---|
| 安装包 | ⏳ Phase 2 由 `release-artifacts` 生成 | Release `v1.3.4` 的 `HNReader-Setup-1.3.4.exe` |
| 扩展包 | ⏳ Phase 2 由 `release-artifacts` 生成 | `HNReader-extension-1.3.4.zip` |
| 版本号 | ✅ `1.3.4` | [extension/manifest.json](../../../extension/manifest.json) |
| 摘要 | ✅ 来自软件包（未改） | `store-copy.txt` 的 Summary |
| 商店截图 | ✅ 云模式首屏未变，沿用 v1.3.3 的 1280×800 截图 | [`screenshots/jt1.png`](screenshots/jt1.png) |
| 新版本说明 | ✅ | [`whats-new.md`](whats-new.md) |
| 纯文本上架稿 | ✅ | [`store-copy.txt`](store-copy.txt) |
| 合规核对 | ✅ 权限与数据披露未变 | 无需改商店隐私字段 |
| 部署记录 | — | 云服务不变，无需部署 |

## 本次变化

- #35 卸载默认保留数据，升级前自动备份数据库与 `.env`。
- #36 数据迁到 `%LOCALAPPDATA%\HN Reader`，程序迁到 `%LOCALAPPDATA%\Programs\HN Reader`。
- #37 popup 本地模式显示服务端版本；低于扩展版本时给下载入口。
- #38 升级/卸载等待 `hn-reader.exe` 真正退出，超时中止。
- #39 启动日志带版本号、数据目录与数据库路径。
- #40 per-user 安装，免 UAC；旧服务版首次升级用一次 UAC 迁移。

## 验证记录

### Phase 1（Linux + GitHub `windows-latest`）

- [x] `go test ./...` / `go test -tags cloud ./...` / `go vet -tags cloud ./...`
- [x] `node scripts/check-i18n.mjs`
- [x] `node scripts/check-popup.mjs`
- [x] `scripts/build-windows.sh` 编译出安装包与 helper
- [x] `Installer smoke` 的 `fresh` / `legacy` 两个 job 绿
- [x] `release-artifacts` 未触发（不打 tag）

### Phase 2（Windows 实机，按 [`windows-test.md`](windows-test.md) 执行）

- [ ] 从当前 1.1.0/1.3.3 服务版升级：只弹一次 UAC
- [ ] 迁移后服务 `HNReader` 消失，数据库完整迁到 `%LOCALAPPDATA%\HN Reader`
- [ ] 再次运行安装包不再弹 UAC
- [ ] 注销/重新登录后 `hn-reader.exe` 自动启动，`/health` 为 1.3.4
- [ ] popup 本地模式显示 `本地服务 v1.3.4`
- [ ] 卸载默认保留数据；`/DELETE_DATA=1` 才删除

## 发布记录

- tag：待 Phase 2
- Release：待 Phase 2
- milestone：待 Phase 2 确认包含 #35–#40
- CWS：1.3.3 已于 2026-11-10 审核通过并上线；1.3.4 待 Windows 实机验证通过后再提交

## 备注

- 本次是扩展 + 本地服务发布，不包含云服务部署。
- 安装包不是代码签名产物，SmartScreen 可能提示未知发布者。
- 商店截图沿用 v1.3.3：云模式首屏没有视觉变化；版本行只在本地模式出现。
