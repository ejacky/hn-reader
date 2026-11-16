# v1.3.4 Windows 实机验证清单

Phase 2 用。目标不是重复 CI 的 `/S` 路径，而是验证 CI 测不到的两件事：**真实 UAC** 和 **登录后自启**。

## 前置

- 一台仍装着旧版 Windows 服务的机器（当前 1.1.0 / 1.3.3），普通用户会话，**不要**右键“以管理员身份运行”安装包。
- 先备份：

```powershell
$data = Join-Path $env:LOCALAPPDATA 'HN Reader'
$legacy = Join-Path ${env:ProgramFiles(x86)} 'HN Reader'
Copy-Item $legacy "$env:USERPROFILE\hn-reader-legacy-backup" -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item $data "$env:USERPROFILE\hn-reader-data-backup" -Recurse -Force -ErrorAction SilentlyContinue
sc.exe query HNReader
```

- 记录迁移前：

```powershell
Get-Service HNReader | Format-List Name,Status,StartType
(Get-FileHash (Join-Path $legacy 'hn-reader.db') -Algorithm SHA256).Hash
Invoke-RestMethod http://127.0.0.1:8080/health
```

## 测试 1：取消 UAC 不破坏旧版

1. 双击 `HNReader-Setup-1.3.4.exe`；
2. 出现“需要管理员授权迁移旧服务”的提示后，在 UAC 对话框点“否”；
3. 安装器应退出，不修改旧版。

验证：

```powershell
Get-Service HNReader          # 仍是 Running
Invoke-RestMethod http://127.0.0.1:8080/health   # 仍是旧版本
Test-Path (Join-Path ${env:ProgramFiles(x86)} 'HN Reader\hn-reader.exe')  # True
```

## 测试 2：首次迁移只弹一次 UAC

1. 再次双击安装包，UAC 点“是”；
2. 等待安装完成，确认只出现这一次 UAC。

验证：

```powershell
Get-Service HNReader -ErrorAction SilentlyContinue   # 应为空
Test-Path (Join-Path ${env:ProgramFiles(x86)} 'HN Reader\hn-reader.exe')  # False
$data = Join-Path $env:LOCALAPPDATA 'HN Reader'
Get-ChildItem $data                                  # hn-reader.db / .env / hn-reader.log / backups
(Get-FileHash (Join-Path $data 'hn-reader.db') -Algorithm SHA256).Hash  # 与迁移前一致
(Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name HNReader).HNReader
Invoke-RestMethod http://127.0.0.1:8080/health       # version = 1.3.4
```

## 测试 3：二次升级不再 UAC

在**普通** PowerShell（不是管理员）里：

```powershell
& "$env:USERPROFILE\Downloads\HNReader-Setup-1.3.4.exe" /S
```

验证：

- 没有 UAC 弹窗；
- `Get-ChildItem "$env:LOCALAPPDATA\HN Reader\backups"` 多一份带时间戳的备份；
- `/health` 仍为 1.3.4；
- 数据库仍在原路径。

## 测试 4：注销/重新登录后自启

1. 注销当前 Windows 用户并重新登录（或重启）；
2. 不手动启动任何程序，等待 10 秒；
3. 验证：

```powershell
Get-Process hn-reader
Invoke-RestMethod http://127.0.0.1:8080/health
Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name HNReader
```

4. 打开扩展 popup，本地模式应显示“本地服务 v1.3.4”。

## 测试 5：卸载默认保留数据

```powershell
$uninstaller = Join-Path $env:LOCALAPPDATA 'Programs\HN Reader\uninstall.exe'
& $uninstaller /S
```

验证：

```powershell
Test-Path (Join-Path $env:LOCALAPPDATA 'Programs\HN Reader')  # False
Test-Path (Join-Path $env:LOCALAPPDATA 'HN Reader\hn-reader.db')  # True
```

再重装一次，然后显式删除：

```powershell
& "$env:USERPROFILE\Downloads\HNReader-Setup-1.3.4.exe" /S
& $uninstaller /S /DELETE_DATA=1
Test-Path (Join-Path $env:LOCALAPPDATA 'HN Reader')  # False
```

## 失败时收集

```powershell
Get-Content "$env:TEMP\hn-reader-migrate.log" -ErrorAction SilentlyContinue
Get-Content "$env:LOCALAPPDATA\HN Reader\hn-reader.log" -Tail 100 -ErrorAction SilentlyContinue
sc.exe query HNReader
Get-Process hn-reader -ErrorAction SilentlyContinue
```

## 通过标准

- [ ] 取消 UAC 后旧服务仍可用、数据未变
- [ ] 首次迁移只弹一次 UAC，服务被移除，DB 哈希不变
- [ ] 二次 `/S` 升级无 UAC，且有升级前备份
- [ ] 注销/重新登录后自动启动，`/health` 为 1.3.4
- [ ] popup 显示“本地服务 v1.3.4”
- [ ] 静默卸载默认保留数据，`/DELETE_DATA=1` 才删除
