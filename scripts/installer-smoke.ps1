param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('fresh', 'legacy')]
    [string]$Mode
)

$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $PSScriptRoot
$Version = (Get-Content (Join-Path $Root 'extension\manifest.json') -Raw | ConvertFrom-Json).version
$Installer = Join-Path $Root "installer\HNReader-Setup-$Version.exe"
$DataDir = Join-Path $env:LOCALAPPDATA 'HN Reader'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\HN Reader'
$ServerExe = Join-Path $InstallDir 'hn-reader.exe'
$Uninstaller = Join-Path $InstallDir 'uninstall.exe'
$TempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { $env:TEMP }
$RunKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$UninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\HN Reader Server'
$LegacyUninstallKey = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\HN Reader Server'
$LegacyDirs = @(
    (Join-Path ([Environment]::GetFolderPath('ProgramFilesX86')) 'HN Reader'),
    (Join-Path ([Environment]::GetFolderPath('ProgramFiles')) 'HN Reader')
)

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Stop-HnReader {
    Get-Process -Name hn-reader -ErrorAction SilentlyContinue |
        Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500
}

function Wait-ProcessGone {
    param([int]$Seconds = 30)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        if (-not (Get-Process -Name hn-reader -ErrorAction SilentlyContinue)) { return }
        Start-Sleep -Milliseconds 500
    }
    throw 'hn-reader.exe did not stop'
}

function Wait-Health {
    param([string]$Expected, [int]$Seconds = 30)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $health = Invoke-RestMethod 'http://127.0.0.1:8080/health' -TimeoutSec 2
            if ($health.version -eq $Expected) { return $health }
        } catch {
            # The server may still be starting.
        }
        Start-Sleep -Milliseconds 500
    }
    throw "health did not report $Expected"
}

function Wait-PortClosed {
    param([int]$Seconds = 20)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $open = Test-NetConnection -ComputerName 127.0.0.1 -Port 8080 -InformationLevel Quiet -WarningAction SilentlyContinue
        if (-not $open) { return }
        Start-Sleep -Milliseconds 500
    }
    throw 'port 8080 is still open'
}

function Start-Local {
    Start-Process -FilePath $ServerExe -ArgumentList '-background' -WindowStyle Hidden | Out-Null
}

function Assert-Installed {
    Assert-True (Test-Path $ServerExe) "server exe missing at $ServerExe"
    Assert-True (Test-Path (Join-Path $DataDir 'hn-reader.db')) 'database missing'
    Assert-True (Test-Path (Join-Path $DataDir '.env')) '.env missing'
    Assert-True (Test-Path (Join-Path $DataDir '.hn-reader-data')) 'data marker missing'
    $run = (Get-ItemProperty $RunKey -Name HNReader -ErrorAction SilentlyContinue).HNReader
    Assert-True ($run -and $run.Contains('hn-reader.exe')) 'HKCU Run entry missing'
}

function Get-DbHash {
    $db = Join-Path $DataDir 'hn-reader.db'
    Assert-True (Test-Path $db) "database missing at $db"
    return (Get-FileHash -Algorithm SHA256 $db).Hash
}

function Install-New {
    param([string[]]$Extra = @())
    $arguments = @('/S', '/APIKEY=test') + $Extra
    Start-Process -FilePath $Installer -ArgumentList $arguments -Wait
}

function Remove-State {
    try {
        Stop-HnReader
        $service = Get-Service -Name HNReader -ErrorAction SilentlyContinue
        if ($service) {
            Stop-Service -Name HNReader -Force -ErrorAction SilentlyContinue
            Start-Sleep -Seconds 1
            sc.exe delete HNReader | Out-Null
        }
        for ($i = 0; $i -lt 3; $i++) {
            Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
            Remove-Item $DataDir -Recurse -Force -ErrorAction SilentlyContinue
            foreach ($dir in $LegacyDirs) { Remove-Item $dir -Recurse -Force -ErrorAction SilentlyContinue }
            if (-not (Test-Path $InstallDir) -and -not (Test-Path $DataDir)) { break }
            Start-Sleep -Seconds 1
        }
        Remove-ItemProperty $RunKey -Name HNReader -ErrorAction SilentlyContinue
        Remove-Item $UninstallKey -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item $LegacyUninstallKey -Recurse -Force -ErrorAction SilentlyContinue
    } catch {
        Write-Warning "cleanup: $($_.Exception.Message)"
    }
}

function Test-Fresh {
    Remove-State
    Install-New
    Wait-Health $Version | Out-Null
    Assert-Installed
    Assert-True (Test-Path (Join-Path $DataDir 'hn-reader.log')) 'log file missing'

    Stop-HnReader
    $hash1 = Get-DbHash

    # Upgrade without starting the new binary, so the database can be compared
    # before SQLite opens it.
    Install-New @('/NOSTART=1')
    $backups = @(Get-ChildItem (Join-Path $DataDir 'backups') -Directory -ErrorAction SilentlyContinue)
    Assert-True ($backups.Count -ge 1) 'upgrade did not create a backup'
    $hash2 = Get-DbHash
    Assert-True ($hash1 -eq $hash2) 'database changed during upgrade'

    Start-Local
    Wait-Health $Version | Out-Null
    Stop-HnReader

    Start-Process -FilePath $Uninstaller -ArgumentList '/S' -Wait
    Wait-PortClosed
    Start-Sleep -Seconds 1
    Assert-True (-not (Test-Path $InstallDir)) 'program directory survived uninstall'
    Assert-True (Test-Path (Join-Path $DataDir 'hn-reader.db')) 'uninstall removed data by default'

    Install-New
    Wait-Health $Version | Out-Null
    Start-Process -FilePath $Uninstaller -ArgumentList @('/S', '/DELETE_DATA=1') -Wait
    Wait-PortClosed
    Start-Sleep -Seconds 1
    Assert-True (-not (Test-Path $DataDir)) '/DELETE_DATA=1 did not remove data'
}

function Test-Legacy {
    Remove-State
    $legacyDir = Join-Path ([Environment]::GetFolderPath('ProgramFilesX86')) 'HN Reader'
    # Remove-State above already removed both possible legacy directories.
    $legacyInstaller = Join-Path $TempRoot 'HNReader-Setup-1.3.3.exe'
    New-Item -ItemType Directory -Force (Split-Path $legacyInstaller) | Out-Null

    gh release download v1.3.3 --pattern 'HNReader-Setup-1.3.3.exe' --dir (Split-Path $legacyInstaller) --clobber
    Start-Process -FilePath $legacyInstaller -ArgumentList '/S' -Wait
    Wait-Health '1.3.3' 60 | Out-Null

    Assert-True (Get-Service -Name HNReader -ErrorAction SilentlyContinue) 'legacy service was not installed'
    $legacyDb = Join-Path $legacyDir 'hn-reader.db'
    Assert-True (Test-Path $legacyDb) "legacy database missing at $legacyDb"

    sc.exe stop HNReader | Out-Null
    Wait-ProcessGone
    $legacyHash = (Get-FileHash -Algorithm SHA256 $legacyDb).Hash

    Install-New @('/NOSTART=1')
    Assert-True (-not (Get-Service -Name HNReader -ErrorAction SilentlyContinue)) 'legacy service still exists'
    Assert-True (-not (Test-Path (Join-Path $legacyDir 'hn-reader.exe'))) 'legacy exe still exists'
    Assert-Installed
    $newHash = Get-DbHash
    Assert-True ($legacyHash -eq $newHash) 'migration changed the database'

    Start-Local
    Wait-Health $Version | Out-Null
    Start-Process -FilePath $Uninstaller -ArgumentList '/S' -Wait
    Wait-PortClosed
    Assert-True (Test-Path (Join-Path $DataDir 'hn-reader.db')) 'uninstall removed migrated data by default'
}

try {
    Assert-True (Test-Path $Installer) "installer missing at $Installer"
    if ($Mode -eq 'fresh') {
        Test-Fresh
    } else {
        Test-Legacy
    }
    Write-Host "installer smoke ($Mode) passed"
} finally {
    Remove-State
}
