; HN Reader Server - Windows Installer
; Build: makensis setup.nsi (or use Makefile release target)

Unicode true

!include "MUI2.nsh"
!include "FileFunc.nsh"

!ifndef PRODUCT_VERSION
  !define PRODUCT_VERSION "1.0.0"
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
InstallDir "$PROGRAMFILES\HN Reader"
RequestExecutionLevel admin
ShowInstDetails show

; --- MUI2 Pages ---
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY

; Custom page for LLM configuration
Page custom ConfigPage ConfigPageLeave

!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

; --- Variables for custom page ---
Var ApiKey
Var BaseUrl
Var Model
Var Port
Var ApiKeyCtrl
Var BaseUrlCtrl
Var ModelCtrl
Var PortCtrl

Function ConfigPage
  !insertmacro MUI_HEADER_TEXT "LLM 配置" "配置 AI 摘要服务所需的 API 信息"

  nsDialogs::Create 1018
  Pop $0

  ${If} $0 == error
    Abort
  ${EndIf}

  ; API Key
  ${NSD_CreateLabel} 0 0 100% 12u "API Key (必填):"
  Pop $0
  ${NSD_CreateText} 0 14u 100% 13u ""
  Pop $ApiKeyCtrl

  ; Base URL
  ${NSD_CreateLabel} 0 34u 100% 12u "Base URL:"
  Pop $0
  ${NSD_CreateText} 0 48u 100% 13u "https://api.deepseek.com"
  Pop $BaseUrlCtrl

  ; Model
  ${NSD_CreateLabel} 0 68u 100% 12u "Model:"
  Pop $0
  ${NSD_CreateDropList} 0 82u 100% 24u ""
  Pop $ModelCtrl
  ${NSD_CB_AddString} $ModelCtrl "deepseek-v4-pro"
  ${NSD_CB_AddString} $ModelCtrl "deepseek-v4-flash"
  ${NSD_CB_SelectString} $ModelCtrl "deepseek-v4-pro"

  ; Port
  ${NSD_CreateLabel} 0 114u 100% 12u "Port:"
  Pop $0
  ${NSD_CreateText} 0 128u 100% 13u "8080"
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

; --- Install Section ---
Section "Install"
  SetOutPath "$INSTDIR"

  ; Copy server binary
  File "..\server\hn-reader.exe"

  ; Write .env file
  FileOpen $0 "$INSTDIR\.env" w
  FileWrite $0 "LLM_API_KEY=$ApiKey$\r$\n"
  FileWrite $0 "LLM_BASE_URL=$BaseUrl$\r$\n"
  FileWrite $0 "LLM_MODEL=$Model$\r$\n"
  FileWrite $0 "PORT=$Port$\r$\n"
  FileClose $0

  ; Install Windows service (auto-start)
  DetailPrint "Registering Windows service..."
  nsExec::ExecToLog '"$INSTDIR\hn-reader.exe" -install'
  Pop $0

  ; Start the service
  DetailPrint "Starting service..."
  nsExec::ExecToLog 'sc start ${SERVICE_NAME}'
  Pop $0

  ; Create uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Registry for Add/Remove Programs
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" \
    "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" \
    "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" \
    "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" \
    "Publisher" "${PRODUCT_PUBLISHER}"
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" \
    "EstimatedSize" "$0"
SectionEnd

; --- Uninstall Section ---
Section "Uninstall"
  ; Stop and remove service
  nsExec::ExecToLog '"$INSTDIR\hn-reader.exe" -uninstall'

  Delete "$INSTDIR\hn-reader.exe"
  Delete "$INSTDIR\.env"
  Delete "$INSTDIR\hn-reader.log"
  Delete "$INSTDIR\hn-reader.db"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
SectionEnd
