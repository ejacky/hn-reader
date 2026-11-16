# Windows 安装器

v1.3.4 起，本地服务改为 **per-user 安装**，不再注册 Windows 服务：

- 程序：`%LOCALAPPDATA%\Programs\HN Reader`
- 数据：`%LOCALAPPDATA%\HN Reader`（`hn-reader.db`、`.env`、`hn-reader.log`、`backups\`）
- 启动：安装完成立即启动；`HKCU\Software\Microsoft\Windows\CurrentVersion\Run\HNReader` 负责登录后自动后台启动
- 权限：主安装器是 `RequestExecutionLevel user`，新安装和后续升级都不需要管理员权限；不要右键「以管理员身份运行」，否则会装到管理员账户下

只有从 `1.3.3` 及更早的「Windows 服务」版本升级时，才需要一次 UAC：主安装器检测到 HKLM 卸载键或 `HNReader` 服务后，启动 `legacy-cleanup.exe` 完成迁移。

## 文件

| 文件 | 作用 |
|---|---|
| `setup.nsi` | 主安装器/卸载器，user-level |
| `legacy-cleanup.nsi` | 旧服务迁移 helper，admin-level，嵌入主安装器 |
| `common.nsh` | 停止进程、等待退出等共享宏 |

`setup.nsi` 通过 `File "../server/hn-reader.exe"` 打包服务端二进制，**构建前必须先编译** `server/hn-reader.exe`。

## 构建

Windows（需要 NSIS 3.x 和 Go）：

```powershell
make release
```

或手工：

```powershell
cd server
go build -trimpath -ldflags "-s -w -H windowsgui -X main.Version=1.3.4" -o hn-reader.exe ./cmd/hn-reader

cd ..\installer
makensis -INPUTCHARSET UTF8 legacy-cleanup.nsi
makensis -INPUTCHARSET UTF8 /DPRODUCT_VERSION=1.3.4 setup.nsi
```

Linux（只做编译校验，不能运行安装器）：

```bash
./scripts/build-windows.sh
```

脚本使用 PATH 里的 `makensis`；也可以设置 `MAKENSIS`/`NSISDIR` 指向解包的 NSIS。

## 静默参数

```text
HNReader-Setup-<版本>.exe /S /APIKEY=sk-... /BASEURL=https://api.deepseek.com /MODEL=deepseek-v4-pro /PORT=8080
```

- 已有 `%LOCALAPPDATA%\HN Reader\.env` 时保留原配置，忽略 `/APIKEY` 等参数；
- 没有 `.env` 且没传 `/APIKEY` 时会写一个空 Key 的 `.env`，服务能启动但摘要会失败；
- `/NOSTART=1` 只用于测试：安装后不自动启动服务；
- 卸载：`uninstall.exe /S` 默认保留数据；`uninstall.exe /S /DELETE_DATA=1` 删除数据；交互式卸载会弹确认框，默认保留。

## 旧服务迁移流程

1. 主安装器读 `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\HN Reader Server` 的 `InstallLocation`；
2. 写 `$PLUGINSDIR\migrate.ini`（旧目录、目标数据目录、结果文件路径），用 `ExecShell "runas"` 启动 helper；
3. helper 停止 `HNReader` 服务、等待 `hn-reader.exe` 退出、把数据库（含 WAL/SHM）与 `.env` 复制到目标数据目录并校验；
4. 复制成功后才 `sc delete`、删除旧程序文件与 HKLM 卸载键；
5. 任一步失败会写 `error:...` 到结果文件，并尝试 `sc start` 恢复旧服务；主安装器看到非 `ok` 就中止，不覆盖安装。

结果文件由 helper 先写 `.tmp` 再 rename，主安装器最多重试读取 5 秒；迁移失败时错误会写到 `%TEMP%\hn-reader-migrate.log`，静默安装也能在事后查。

## 测试

Windows 上：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/installer-smoke.ps1 -Mode fresh
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/installer-smoke.ps1 -Mode legacy
```

- `fresh`：新装 → 启动 → 升级备份与数据库不变 → 卸载保留数据 → 重装 → `/DELETE_DATA=1` 删除；
- `legacy`：下载并安装 v1.3.3 服务版 → 停止服务记录数据库哈希 → v1.3.4 迁移 → 校验服务删除、数据完整、启动后 `/health` 为新版本。

CI 对应 `.github/workflows/installer-smoke.yml`；`release-artifacts.yml` 会在上传 Release 前再跑一次 `fresh`。

## 已知限制

- UAC 弹窗本身、登录后 HKCU Run 是否生效，只能在真实 Windows 用户会话里验证；CI runner 是管理员，测不到这两项。
- Linux 只能编译，不能运行安装器；Wine 对服务/UAC/注册表的模拟不可靠，不作为验收依据。
- 安装器未做代码签名，SmartScreen 仍可能提示未知发布者。
