# 发版材料

一次发版一个目录：`docs/releases/v<版本>/`。目的很简单——打开这个目录，就知道这次要在 Chrome Web Store 交哪些东西、各自在哪、还缺什么。

```
docs/releases/
  README.md                ← 本文件：约定
  v1.3.0/
    README.md              ← 本次要交的材料清单（含状态）
    screenshots/           ← 商店截图：jt1.png / jt2.png / jt3.png
    whats-new.md           ← 商店「新版本说明」，可直接粘贴
```

## 每次发版要准备的东西

| 材料 | 位置 | 说明 |
|---|---|---|
| 安装包 | `extension/extension.zip`（同时挂到 GitHub Release） | 由 `extension/` 打包，**不要带顶层目录**，`manifest.json` 在根 |
| 商店截图 | `docs/releases/v<ver>/screenshots/` | 命名 `jt1/jt2/jt3`；尺寸见下表 |
| 新版本说明 | `docs/releases/v<ver>/whats-new.md` | 商店「What's new in this version」 |
| 商店文案 | `docs/store-listing.md` | 名称/简介/详细描述/权限说明/隐私披露，长期维护，就地更新 |
| 版本号 | `extension/manifest.json` 的 `version` | 必须与 tag 一致，否则商店拒收 |
| 发布记录 | GitHub Release 的 body | 见 `docs/RELEASE_NOTES.md` 的对应段落 |

## 截图规格（Chrome Web Store）

| 用途 | 尺寸 | 数量 | 备注 |
|---|---|---|---|
| 截图 | 1280×800 或 640×400 | 1–5 张 | 两种尺寸可以混用，但同一套里混用看起来不整齐 |
| 小型宣传图（可选） | 440×280 | 0–1 | 列表页右上角 |
| 大型宣传图（可选） | 1400×560 | 0–1 | 首页横幅，需要单独设计 |
| 商店图标 | 128×128 | 1 | 已有 `extension/icons/icon128.png` |

## 打包安装包

```powershell
# 与仓库里的做法一致：内容铺平，不带 extension/ 前缀，排除 zip 自身
$stage = '.gotmp\ziproot'
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item extension $stage -Recurse
Remove-Item "$stage\extension.zip" -ErrorAction SilentlyContinue
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::CreateFromDirectory(
  (Resolve-Path $stage).Path, (Join-Path $PWD 'extension\extension.zip'),
  [System.IO.Compression.CompressionLevel]::Optimal, $false)
```

打包后自检：`manifest.json` 在 zip 根目录、`_locales/` 与 `i18n.js` 都在、条目里**没有** `extension.zip`。

## 新版本目录怎么建

1. `New-Item -ItemType Directory docs\releases\v<ver>\screenshots -Force`
2. 复制上一版目录的 `README.md`，按本版改：材料状态、截图说明、本次变更点。
3. 放截图、写 `whats-new.md`。
4. 更新 `docs/store-listing.md`（版本号、文案、审核历史里加一条）。
5. 发版后在 GitHub Release 的 body 里补 `### Deployed` 段（环境、版本、时间）。

## 检查清单（每个版本都要过）

- [ ] `manifest.json` 的 `version` 与 tag 一致
- [ ] `extension/extension.zip` 已重打，且是当前代码
- [ ] 截图是新 UI（截图会在拦截位显示旧版界面）
- [ ] `whats-new.md` 已写，且没有承诺未实现的功能
- [ ] `docs/store-listing.md` 的权限/隐私披露与 `manifest.json` 一致
- [ ] GitHub Release 已建，附件含 zip
- [ ] `### Deployed` 记录了两个环境的版本与时间
