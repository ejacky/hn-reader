# v1.3.1 发版材料（待提交）

这是要提交给 Chrome Web Store 的版本。相对 `v1.3.0` 只有一处功能变化：**从 manifest 里移除了仅供本机联调的 staging 域名权限**，所以商店的权限提示不会多出一条用户永远用不到的域名。

约定见 [../README.md](../README.md)。

## 状态

| 材料 | 状态 | 位置 |
|---|---|---|
| 安装包 | ✅ | Release `v1.3.1` 的 `HNReader-extension-1.3.1.zip` |
| 版本号 | ✅ `1.3.1` | `extension/manifest.json` |
| 权限 | ✅ 只剩 `news.ycombinator.com` + `hn-reader.whoareyou.ren` | `extension/manifest.json` |
| 商店截图 | ⚠️ **待重拍**（见下） | `screenshots/jt1.png`、`jt2.png`、`jt3.png` |
| 新版本说明 | ✅ | [`whats-new.md`](whats-new.md) |
| 商店文案 | ✅ | [`../../store-listing.md`](../../store-listing.md) |
| 服务端 | ✅ `1.3.1`（与 1.3.0 行为一致，只同步版本号） | `/health` |

## 截图

| 文件 | 尺寸 | 内容 |
|---|---|---|
| `jt1.png` | 640×400 | 弹窗（**本地模式**）：已连接、自动加载开、界面语言 Auto、摘要语言中文、`localhost:8080` |
| `jt2.png` | 1280×800 | HN 首页：每条 `[summary]` + 打开的一条英文摘要卡片 |
| `jt3.png` | 640×400 | 弹窗（**本地模式**）：界面语言 English、摘要语言 English、`localhost:8080` |

### 提交前建议重拍

1. **两张弹窗图是本地模式，露出了 `localhost:8080` 和 "Switch back to cloud"**。商店页主打"零配置云模式"，这组图会让人以为要自己搭服务器，审核也可能追问。应改用**云模式**截图：`Connected`、账户状态、额度条。
2. **尺寸混用**（640×400 ×2 + 1280×800 ×1），列表里排列不齐；建议整套统一 1280×800。
3. **缺一张展示核心价值的图**（额度 / 推荐 / 一眼看出列表被摘要过）。

目标套装：

| # | 尺寸 | 拍什么 |
|---|---|---|
| jt1 | 1280×800 | HN 首页，多条已出摘要 + 一条展开的摘要卡片（可复用 jt2 的角度） |
| jt2 | 1280×800 | 弹窗·云模式：Connected、额度条、界面语言 Auto、摘要语言 |
| jt3 | 1280×800 | 推荐 / Your Stats |

## 提交步骤

1. 开发者后台 → 选择扩展 → **上传新版本**，选 `extension/extension.zip`。
2. 填「新版本说明」，复制 [`whats-new.md`](whats-new.md)。
3. 替换截图（按上面建议重拍）。
4. 核对权限说明、隐私披露、单一用途说明仍与 `manifest.json` 一致——本次**权限减少了**，商店会显示"权限减少"，这是正常的。
5. 提交审核。
6. 通过后：把本文件顶部截图状态改成 ✅，并在 [`../../store-listing.md`](../../store-listing.md) 的「审核历史」补一条记录。

## 相对 v1.3.0 的变化

- `host_permissions` 移除 `https://test.hn-reader.whoareyou.ren/*`（本机联调用，用户用不到）。
- 版本号 `1.3.1`；服务端同步重发，`/health` 报 `1.3.1`，行为与 `1.3.0` 相同。
- 其余功能与 `v1.3.0` 完全一致：界面英文化、摘要语言跟随界面、预加载状态显式化、请求 20 秒超时。
