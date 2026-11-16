; HN Reader Local Server - per-user Windows installer
;
; Design:
;   - installed under %LOCALAPPDATA%\Programs\HN Reader, data under
;     %LOCALAPPDATA%\HN Reader, no Windows service;
;   - the installer itself is user-level, so upgrades never need UAC;
;   - a machine-wide install from an older version is migrated once by
;     legacy-cleanup.exe, which is the only elevated part.

Unicode true

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"
!include "common.nsh"

!ifndef PRODUCT_VERSION
  !define PRODUCT_VERSION "1.3.4"
!endif
!ifndef PRODUCT_NAME
  !define PRODUCT_NAME "HN Reader Server"
!endif
!ifndef PRODUCT_PUBLISHER
  !define PRODUCT_PUBLISHER "HN Reader"
!endif
!ifndef SERVICE_NAME
  !define SERVICE_NAME "HNReader"
!endif

Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "HNReader-Setup-${PRODUCT_VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\HN Reader"
RequestExecutionLevel user
ShowInstDetails show
BrandingText "HN Reader"

VIProductVersion "${PRODUCT_VERSION}.0"
VIAddVersionKey /LANG=1033 "ProductName" "${PRODUCT_NAME}"
VIAddVersionKey /LANG=1033 "ProductVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=1033 "CompanyName" "${PRODUCT_PUBLISHER}"
VIAddVersionKey /LANG=1033 "FileDescription" "HN Reader local server installer"
VIAddVersionKey /LANG=1033 "FileVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=1033 "LegalCopyright" "HN Reader"

; --- MUI2 pages ---
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
Page custom ConfigPage ConfigPageLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

; --- Variables ---
Var DataDir
Var LegacyDir
Var HasLegacy
Var HasExistingConfig
Var DeleteData
Var NoStart

Var ApiKey
Var BaseUrl
Var Model
Var Port
Var ApiKeyCtrl
Var BaseUrlCtrl
Var ModelCtrl
Var PortCtrl

; ============================================
; .onInit: parse options, detect old install
; ============================================
Function un.StopServerProcess
  nsExec::ExecToLog 'taskkill /IM hn-reader.exe /F'
  Pop $R0
  !insertmacro WaitProcessBody
FunctionEnd

Function .onInit
  StrCpy $DataDir "$LOCALAPPDATA\HN Reader"
  ${If} $LOCALAPPDATA == ""
    StrCpy $DataDir "$PROFILE\AppData\Local\HN Reader"
  ${EndIf}
  StrCpy $LegacyDir ""
  StrCpy $HasLegacy 0
  StrCpy $HasExistingConfig 0
  StrCpy $DeleteData "0"
  StrCpy $NoStart "0"
  StrCpy $ApiKey ""
  StrCpy $BaseUrl ""
  StrCpy $Model ""
  StrCpy $Port ""

  ${GetParameters} $R0
  ${GetOptions} $R0 "/APIKEY=" $ApiKey
  !insertmacro Unquote $ApiKey
  ${GetOptions} $R0 "/BASEURL=" $BaseUrl
  !insertmacro Unquote $BaseUrl
  ${GetOptions} $R0 "/MODEL=" $Model
  !insertmacro Unquote $Model
  ${GetOptions} $R0 "/PORT=" $Port
  !insertmacro Unquote $Port
  ${GetOptions} $R0 "/DELETE_DATA=" $DeleteData
  !insertmacro Unquote $DeleteData
  ${GetOptions} $R0 "/NOSTART=" $NoStart
  !insertmacro Unquote $NoStart

  ; An existing per-user install keeps its chosen directories.
  ReadRegStr $R0 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "InstallLocation"
  ${If} $R0 != ""
    StrCpy $INSTDIR $R0
  ${EndIf}
  ReadRegStr $R0 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DataDir"
  ${If} $R0 != ""
    StrCpy $DataDir $R0
  ${EndIf}

  ; A machine-wide install from <= 1.3.3 has an HKLM uninstall key and a
  ; Windows service. Migrate it once before the user-level install continues.
  ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "InstallLocation"
  ${If} $R0 != ""
    StrCpy $LegacyDir $R0
    StrCpy $HasLegacy 1
  ${Else}
    nsExec::ExecToStack 'sc query ${SERVICE_NAME}'
    Pop $R1
    Pop $R2
    ${If} $R1 == 0
      StrCpy $LegacyDir "$PROGRAMFILES\HN Reader"
      StrCpy $HasLegacy 1
    ${EndIf}
  ${EndIf}

  ${If} $HasLegacy == 1
    Call MigrateLegacy
  ${EndIf}

  ${If} ${FileExists} "$DataDir\.env"
    StrCpy $HasExistingConfig 1
  ${EndIf}
FunctionEnd

; ============================================
; Run the elevated migration helper and wait
; ============================================
Function MigrateLegacy
  ${IfNot} ${Silent}
    MessageBox MB_OKCANCEL|MB_ICONINFORMATION "检测到旧版 Windows 服务安装。安装程序需要一次管理员授权来停止并移除旧服务，并把数据库和配置迁移到当前用户目录。" IDOK migrate_continue
    Abort
    migrate_continue:
  ${EndIf}

  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File "legacy-cleanup.exe"

  WriteINIStr "$PLUGINSDIR\migrate.ini" "migrate" "LegacyDir" "$LegacyDir"
  WriteINIStr "$PLUGINSDIR\migrate.ini" "migrate" "TargetData" "$DataDir"
  WriteINIStr "$PLUGINSDIR\migrate.ini" "migrate" "Result" "$PLUGINSDIR\migrate-result.txt"
  Delete "$PLUGINSDIR\migrate-result.txt"

  ; If this process is already elevated (CI, administrator shell), run the
  ; helper directly. Otherwise ShellExecute "runas" is what actually prompts UAC.
  nsExec::ExecToStack 'net session'
  Pop $R0
  Pop $R1
  ${If} $R0 == 0
    ExecWait '"$PLUGINSDIR\legacy-cleanup.exe" /INI="$PLUGINSDIR\migrate.ini"'
    Pop $R0
  ${Else}
    ExecShell "runas" '"$PLUGINSDIR\legacy-cleanup.exe"' '/INI="$PLUGINSDIR\migrate.ini"'
  ${EndIf}

  StrCpy $R2 0
  ${Do}
    Sleep 500
    ${If} ${FileExists} "$PLUGINSDIR\migrate-result.txt"
      ${ExitDo}
    ${EndIf}
    IntOp $R2 $R2 + 1
    ${If} $R2 >= 240
      MessageBox MB_ICONSTOP|MB_OK "迁移旧版服务失败或未获得管理员授权。安装已中止，旧版本未被修改。"
      Abort
    ${EndIf}
  ${Loop}

  FileOpen $R3 "$PLUGINSDIR\migrate-result.txt" r
  ${If} ${Errors}
    MessageBox MB_ICONSTOP|MB_OK "无法读取旧版迁移结果。安装已中止。"
    Abort
  ${EndIf}
  FileRead $R3 $R4
  FileClose $R3
  StrCpy $R5 $R4 2
  ${If} $R5 != "ok"
    MessageBox MB_ICONSTOP|MB_OK "迁移旧版服务失败：$R4"
    Abort
  ${EndIf}

  StrCpy $HasLegacy 0
FunctionEnd

; ============================================
; Config page
; ============================================
Function ConfigPage
  ${If} $HasExistingConfig == 1
    Abort
  ${EndIf}

  !insertmacro MUI_HEADER_TEXT "LLM 配置" "配置 AI 摘要服务所需的 API 信息"

  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}

  ${If} $BaseUrl == ""
    StrCpy $BaseUrl "https://api.deepseek.com"
  ${EndIf}
  ${If} $Port == ""
    StrCpy $Port "8080"
  ${EndIf}

  ${NSD_CreateLabel} 0 0 100% 12u "API Key (必填):"
  Pop $0
  ${NSD_CreateText} 0 14u 100% 13u "$ApiKey"
  Pop $ApiKeyCtrl

  ${NSD_CreateLabel} 0 34u 100% 12u "Base URL:"
  Pop $0
  ${NSD_CreateText} 0 48u 100% 13u "$BaseUrl"
  Pop $BaseUrlCtrl

  ${NSD_CreateLabel} 0 68u 100% 12u "Model:"
  Pop $0
  ${NSD_CreateDropList} 0 82u 100% 24u ""
  Pop $ModelCtrl
  ${NSD_CB_AddString} $ModelCtrl "deepseek-v4-pro"
  ${NSD_CB_AddString} $ModelCtrl "deepseek-v4-flash"
  ${If} $Model != ""
    ${NSD_CB_SelectString} $ModelCtrl "$Model"
  ${Else}
    ${NSD_CB_SelectString} $ModelCtrl "deepseek-v4-pro"
  ${EndIf}

  ${NSD_CreateLabel} 0 114u 100% 12u "Port:"
  Pop $0
  ${NSD_CreateText} 0 128u 100% 13u "$Port"
  Pop $PortCtrl

  nsDialogs::Show
FunctionEnd

Function ConfigPageLeave
  ${NSD_GetText} $ApiKeyCtrl $ApiKey
  ${NSD_GetText} $BaseUrlCtrl $BaseUrl
  ${NSD_GetText} $ModelCtrl $Model
  ${NSD_GetText} $PortCtrl $Port

  ${If} $ApiKey == ""
    MessageBox MB_OK|MB_ICONEXCLAMATION "请输入 API Key。"
    Abort
  ${EndIf}
FunctionEnd

; ============================================
; Backup database, WAL/SHM and .env
; ============================================
Function BackupLocalData
  ${IfNot} ${FileExists} "$DataDir\hn-reader.db"
    Return
  ${EndIf}

  CreateDirectory "$DataDir\backups"
  ${GetTime} "" "L" $0 $1 $2 $3 $4 $5 $6
  StrCpy $R7 "$DataDir\backups\$0$1$2-$3$4$5"
  StrCpy $R1 "$R7"
  StrCpy $R2 0
  ${Do}
    ${IfNot} ${FileExists} "$R1\*.*"
      ${ExitDo}
    ${EndIf}
    IntOp $R2 $R2 + 1
    StrCpy $R1 "$R7-$R2"
  ${LoopUntil} $R2 >= 100
  CreateDirectory "$R1"

  CopyFiles /SILENT "$DataDir\hn-reader.db" "$R1\hn-reader.db"
  ${IfNot} ${FileExists} "$R1\hn-reader.db"
    MessageBox MB_ICONSTOP|MB_OK "备份数据库失败，安装已中止。请检查磁盘空间或文件权限后重试。"
    Abort
  ${EndIf}
  nsExec::ExecToLog 'fc /b "$DataDir\hn-reader.db" "$R1\hn-reader.db"'
  Pop $R2
  ${If} $R2 != 0
    MessageBox MB_ICONSTOP|MB_OK "数据库备份校验失败，安装已中止。现有数据未被修改。"
    Abort
  ${EndIf}

  ${If} ${FileExists} "$DataDir\hn-reader.db-wal"
    CopyFiles /SILENT "$DataDir\hn-reader.db-wal" "$R1\hn-reader.db-wal"
  ${EndIf}
  ${If} ${FileExists} "$DataDir\hn-reader.db-shm"
    CopyFiles /SILENT "$DataDir\hn-reader.db-shm" "$R1\hn-reader.db-shm"
  ${EndIf}
  ${If} ${FileExists} "$DataDir\.env"
    CopyFiles /SILENT "$DataDir\.env" "$R1\.env"
  ${EndIf}

  DetailPrint "Backed up local data to $R1"

  ; Keep the five newest backups. Failure here is not fatal.
  nsExec::ExecToLog "powershell -NoProfile -Command $\"Get-ChildItem -LiteralPath '$DataDir\backups' -Directory | Sort-Object Name | Select-Object -SkipLast 5 | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue$\""
FunctionEnd

; ============================================
; Install
; ============================================
Section "Install"
  CreateDirectory "$INSTDIR"
  CreateDirectory "$DataDir"
  CreateDirectory "$DataDir\backups"

  ${If} ${FileExists} "$INSTDIR\hn-reader.exe"
    DetailPrint "Stopping the running local server..."
    Call StopServerProcess
    Pop $R0
    ${If} $R0 != 0
      MessageBox MB_ICONSTOP|MB_OK "无法停止正在运行的本地服务。请手动结束 hn-reader.exe 后重试。"
      Abort
    ${EndIf}
  ${EndIf}

  Call BackupLocalData

  SetOutPath "$INSTDIR"
  File /oname=hn-reader.exe "../server/hn-reader.exe"

  ; .env lives in the data directory from this version on.
  ${IfNot} ${FileExists} "$DataDir\.env"
    ClearErrors
    FileOpen $0 "$DataDir\.env" w
    ${If} ${Errors}
      MessageBox MB_ICONSTOP|MB_OK "无法写入 $DataDir\.env。安装已中止，请检查目录权限。"
      Abort
    ${EndIf}
    FileWrite $0 "LLM_API_KEY=$ApiKey$\r$\n"
    FileWrite $0 "LLM_BASE_URL=$BaseUrl$\r$\n"
    FileWrite $0 "LLM_MODEL=$Model$\r$\n"
    FileWrite $0 "PORT=$Port$\r$\n"
    FileClose $0
    ${IfNot} ${FileExists} "$DataDir\.env"
      MessageBox MB_ICONSTOP|MB_OK "写入 $DataDir\.env 失败。安装已中止。"
      Abort
    ${EndIf}
  ${EndIf}

  ; Marker used by the uninstaller; it only deletes a directory it recognises.
  ClearErrors
  FileOpen $0 "$DataDir\.hn-reader-data" w
  ${If} ${Errors}
    MessageBox MB_ICONSTOP|MB_OK "无法写入数据目录标记。安装已中止。"
    Abort
  ${EndIf}
  FileWrite $0 "HN Reader local data"
  FileClose $0
  ${IfNot} ${FileExists} "$DataDir\.hn-reader-data"
    MessageBox MB_ICONSTOP|MB_OK "写入数据目录标记失败。安装已中止。"
    Abort
  ${EndIf}

  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; HKCU: no administrator rights needed.
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DataDir" "$DataDir"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "EstimatedSize" "$0"

  ; Start at every user logon; no manual click and no service.
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "HNReader" '"$INSTDIR\hn-reader.exe" -background'

  ${If} $NoStart != "1"
    DetailPrint "Starting the local server..."
    Exec '"$INSTDIR\hn-reader.exe" -background'
  ${EndIf}
SectionEnd

; ============================================
; Uninstall
; ============================================
Function un.onInit
  StrCpy $DeleteData "0"
  ${GetParameters} $R0
  ${GetOptions} $R0 "/DELETE_DATA=" $R1
  !insertmacro Unquote $R1
  ${If} $R1 == "1"
    StrCpy $DeleteData "1"
  ${EndIf}
FunctionEnd

Section "Uninstall"
  Call un.StopServerProcess
  Pop $R0
  ${If} $R0 != 0
    MessageBox MB_ICONSTOP|MB_OK "无法停止正在运行的本地服务。请手动结束 hn-reader.exe 后重试。"
    Abort
  ${EndIf}

  ReadRegStr $DataDir HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DataDir"
  ${If} $DataDir == ""
    StrCpy $DataDir "$LOCALAPPDATA\HN Reader"
  ${EndIf}

  ; Default is to keep the local history and API key. /DELETE_DATA=1 is the
  ; explicit opt-in, and a silent uninstall always keeps data.
  StrCpy $R0 1
  ${If} $DeleteData == "1"
    StrCpy $R0 0
  ${Else}
    ${IfNot} ${Silent}
      MessageBox MB_YESNOCANCEL|MB_ICONQUESTION "是否保留本地数据（数据库与配置）？$\r$\n选择「是」保留，「否」删除；关闭窗口也视为保留。" /SD IDYES IDYES keep_data IDNO delete_data
      Goto keep_data
      delete_data:
        StrCpy $R0 0
      keep_data:
    ${EndIf}
  ${EndIf}

  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "HNReader"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

  Delete "$INSTDIR\hn-reader.exe"
  Delete "$INSTDIR\uninstall.exe"
  Delete "$INSTDIR\legacy-cleanup.exe"
  RMDir "$INSTDIR"

  ${If} $R0 == 0
  ${AndIf} ${FileExists} "$DataDir\.hn-reader-data"
    RMDir /r "$DataDir"
  ${EndIf}
SectionEnd
