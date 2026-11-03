# v1.3.0 发版材料

提交到 Chrome Web Store 需要的东西，一份清单看到底。约定见 [../README.md](../README.md)。

## 状态

| 材料 | 状态 | 位置 |
|---|---|---|
| 安装包 | ✅ | `extension/extension.zip`（也是 Release `v1.3.0` 的附件） |
| 版本号 | ✅ `1.3.0` | `extension/manifest.json` |
| 商店截图 | ⚠️ 有，但**建议重拍**（见下） | `screenshots/jt1.png`、`jt2.png`、`jt3.png` |
| 新版本说明 | ✅ | [`whats-new.md`](whats-new.md) |
| 商店文案 | ✅ | [`../../store-listing.md`](../../store-listing.md) |
| 部署记录 | ✅ | Release `v1.3.0` 的 `### Deployed` 段 |
| 权限披露 | ⚠️ **本次 manifest 多了一条 staging 域名权限**，见下 | `manifest.json` 的 `host_permissions` |

## 截图

| 文件 | 尺寸 | 内容 |
|---|---|---|
| `jt1.png` | 640×400 | 弹窗：本地模式已连接、自动加载开、界面语言 Auto、摘要语言中文、Server URL `localhost:8080` |
| `jt2.png` | 1280×800 | HN 首页：每条的 `[summary]`、打开的一条英文摘要卡片 |
| `jt3.png` | 640×400 | 弹窗：本地模式已连接、自动加载关、界面语言 English、摘要语言 English、露出 Your Stats |

### 提交前建议重拍

1. **两张弹窗图是本地模式，且露出了 `localhost:8080` 和 "Switch back to cloud"。** 商店页主打的是"零配置云模式"，这组图会让用户以为要自己搭服务器，审核也可能追问"为什么需要本地服务"。建议以**云模式**截图：弹窗顶部显示 `Connected`、账户状态、额度条。
2. **尺寸混用**（640×400 ×2 + 1280×800 ×1）。CWS 允许混用，但列表里排列不齐；建议整套统一 1280×800。
3. **缺一张展示核心价值的图**：云模式的额度、推荐、或一看就懂的"列表全被摘要过"的对比。

目标套装（建议）：

| # | 尺寸 | 拍什么 |
|---|---|---|
| jt1 | 1280×800 | HN 首页，多条已出摘要 + 一条展开的摘要卡片（可复用 jt2 的角度） |
| jt2 | 1280×800 | 弹窗·云模式：Connected、额度条、界面语言 Auto、摘要语言 |
| jt3 | 1280×800 | 推荐 / Your Stats |

## 发版前必须先决定：staging 域名权限

`extension/manifest.json` 的 `host_permissions` 里有：

```json
"https://test.hn-reader.whoareyou.ren/*"
```

那是给本机联调 staging 用的。它会被 CWS 当作一条**新增的主机权限**展示给用户（"读取和更改你在 test.hn-reader.whoareyou.ren 上的数据"），而普通用户永远不会访问这个域名。

两个选项：

- **A（推荐）**：发版前从 `manifest.json` 移除这一条，另起 `v1.3.1` 提交。联调仍可用：staging 的 API 返回 `Access-Control-Allow-Origin: *`，service worker 不带该权限也能请求（走 CORS）。
- **B**：保留并接受这条权限，商店描述里不提。

> 注意：`v1.3.0` 已经打了 tag 并部署了服务端。改 manifest 需要新的 tag，不要移动 `v1.3.0`。

## 提交步骤

1. 开发者后台 → 选择扩展 → **上传新版本**，选 `extension/extension.zip`。
2. 填「新版本说明」，复制 [`whats-new.md`](whats-new.md)。
3. 替换截图（按上面的建议重拍后）。
4. 核对：隐私披露说法、权限说明、单一用途说明是否仍与 `manifest.json` 一致（本次没有新增数据类别）。
5. 提交审核。
6. 审核通过后：在本文件顶部把截图状态改成 ✅，并在 [`../../store-listing.md`](../../store-listing.md) 的「审核历史」补一条 2026-11-03 的记录。

## 本次相对 v1.2.0 的变化（写说明时的依据）

- 界面英文化：默认跟随浏览器语言（`_locales`，`en` 为默认目录，`zh_CN` 保留），设置里可显式选 Auto / English / 中文。
- 摘要语言默认跟随界面语言（用户显式选过则不变）。
- 预加载状态显式化：每条显示"已预加载"或"失败+原因"，不再无提示地一直转圈；超过 5 分钟的条目自动标失败。
- 摘要/预加载请求 20 秒超时，不再挂在死连接上。
- 服务端：推理模型空摘要修复、预加载接口返回逐条状态、`/health` 带构建后缀。
