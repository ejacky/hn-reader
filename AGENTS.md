# Agents

- 本地代理 127.0.0.1:7897 如果需要访问被屏蔽的网站，需要配置代理

## Issue、提交与关闭

- 需求或缺陷先开 issue，放进 Project（`ejacky` 的个人面板 #2「hn-reader」）；Auto-add 没收录就手动 `item-add`。
- 状态跟着工作走：开工置 `In progress`，等验收置 `In review`。关闭后由面板自带的 Item closed 置 `Done`，不要手改。
- **关闭只认提交信息里的 `Done #N`**（一个提交可以写多个）。`.github/workflows/close-issues.yml` 是可复用 workflow，由 `cloud-server.yml`（server / deploy / docs 路径）和 `extension-i18n.yml`（extension 路径）在各自的检查绿之后调用——关闭发生在检查之后，而不是任意一次推送。
- **不要写 `Closes` / `Fixes` / `Resolves`**：GitHub 在提交到达默认分支的瞬间就按这些词关 issue，抢在测试前面，红了也照关。
- `v*` tag 打包成功后，`release-artifacts.yml` 的 `mark-release` 把区间内 `Done #N` 提到的 issue 归进同名 milestone；只补没有 milestone 的，不覆盖手工规划的。

## 转 public 之前

转 public 会把全部提交历史一起公开，包括早就删掉、只躺在历史里的文件。检查清单、扫描命令和 2026-11-09 的结论见 [`docs/publishing.md`](docs/publishing.md)。
