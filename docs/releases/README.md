# 发版材料

一次发版一个目录：`docs/releases/v<版本>/`。目的很简单——打开这个目录，就知道这次要在 Chrome Web Store 交哪些东西、各自在哪、还缺什么。

```
docs/releases/
  README.md                ← 本文件：约定
  v1.2.0/                  ← 最后一次已上架的材料（截图归档）
  v1.3.0/                  ← 服务端上过生产，商店包未提交，被 1.3.1 取代
  v1.3.1/                  ← 已提交但被挂起，被 1.3.2 取代；截图留在它的 screenshots/
  v1.3.2/                  ← 已提交审核（默认英文；中文条目正文待过审后补）
    README.md              ← 材料清单（含状态、待办）
    whats-new.md           ← 商店「新版本说明」，可直接粘贴
    store-copy.txt         ← 可直接粘贴的纯文本，由 scripts/store-copy.mjs 生成
```

截图跟着产生它的那一版走：下一版重拍后放进新目录，旧目录的图不搬。

已上架的版本不要删，旧的截图留在它自己的目录里；新版本目录里只放本次要交的材料。

发布之间的改动先记在 `docs/RELEASE_NOTES.md` 顶部的 `Unreleased` 段；发版时把它改名成正式版本段（补 `## Release Title`），再建本目录。

## 每次发版要准备的东西

| 材料 | 位置 | 说明 |
|---|---|---|
| 安装包 | GitHub Release 附件 `HNReader-extension-<版本>.zip` | 由 `release-artifacts` 工作流按 tag 从 `extension/` 打包；**不进仓库**，**不带顶层目录**，`manifest.json` 在根 |
| 商店截图 | `docs/releases/v<ver>/screenshots/` | 命名 `jt1/jt2/jt3`；尺寸见下表 |
| 新版本说明 | `docs/releases/v<ver>/whats-new.md` | 商店「What's new in this version」 |
| 纯文本上架稿 | `docs/releases/v<ver>/store-copy.txt` | `node scripts/store-copy.mjs docs/releases/v<ver>` 从 `store-listing.md` + `whats-new.md` 生成，直接粘贴 |
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

安装包不进仓库，也不由人手工打包：推 `v*` tag 后，`.github/workflows/release-artifacts.yml` 会用该 tag 的源码
构建 `HNReader-Setup-<版本>.exe`（Windows 本地服务）、`HNReader-extension-<版本>.zip`（商店包）和 `SHA256SUMS`，
上传到同名 GitHub Release。

需要本地验证时手工打一次即可，但**不要把产物提交进仓库**（`extension/extension.zip` 已在 `.gitignore` 里）：

```powershell
Compress-Archive -Path extension\* -DestinationPath $env:TEMP\extension.zip -Force
```

手工包自检：`manifest.json` 在 zip 根目录、`_locales/` 与 `i18n.js` 都在、条目里没有 `extension.zip`。

## 生成可粘贴的纯文本

商店的描述字段不接受 markdown（星号、井号、反引号都会原样显示），所以每次发版生成一份纯文本：

```powershell
node scripts/store-copy.mjs docs/releases/v1.3.1      # 写 store-copy.txt，并检查 summary ≤ 132 字符
node scripts/store-copy.mjs docs/releases/v1.3.1 --stdout   # 只打印
```

`store-copy.txt` 按字段分段（Summary / Detailed description / Single purpose / 权限说明 / Remote code / What's new），复制哪段贴哪段。

## 本地备份

要用的时候从 Release 下，放在 `artifacts/v<版本>/`（**已 gitignore**，进不了仓库）：

```powershell
New-Item -ItemType Directory -Force artifacts\v1.3.1 | Out-Null
gh release download v1.3.1 --pattern "HNReader-extension-1.3.1.zip" --dir artifacts\v1.3.1
gh release download v1.3.1 --pattern "SHA256SUMS" --dir artifacts\v1.3.1
(Get-FileHash -Algorithm SHA256 artifacts\v1.3.1\HNReader-extension-1.3.1.zip).Hash.ToLower()  # 与 SHA256SUMS 对照
```

`artifacts/` 只是本机缓存：删了随时能从 Release 再下，所以不用备份它本身，也不要提交。

## 新版本目录怎么建

1. `New-Item -ItemType Directory docs\releases\v<ver>\screenshots -Force`
2. 复制上一版目录的 `README.md`，按本版改：材料状态、截图说明、本次变更点。
3. 放截图、写 `whats-new.md`。
4. 更新 `docs/store-listing.md`（版本号、文案、审核历史里加一条）。
5. 发版后在 GitHub Release 的 body 里补 `### Deployed` 段（环境、版本、时间）。

## 检查清单（每个版本都要过）

- [ ] `manifest.json` 的 `version` 与 tag 一致
- [ ] Release 附件里的 `HNReader-extension-<版本>.zip` 已生成，`SHA256SUMS` 对得上
- [ ] 截图是新 UI（截图会在拦截位显示旧版界面）
- [ ] `whats-new.md` 已写，且没有承诺未实现的功能
- [ ] `docs/store-listing.md` 的权限/隐私披露与 `manifest.json` 一致
- [ ] GitHub Release 已建，附件含 zip
- [ ] `### Deployed` 记录了两个环境的版本与时间
