; One-time migration helper for installs that still use the old machine-wide
; Windows service. It is the only executable that asks for administrator rights.
;
; The main installer is user-level. It writes an INI file with the legacy
; directory, the per-user target data directory and a result path, launches this
; helper through ShellExecute "runas", and waits for the result file. Keeping the
; elevated part this small means a normal upgrade never needs UAC.

Unicode true

!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "common.nsh"

!define SERVICE_NAME "HNReader"
!define PRODUCT_NAME "HN Reader Server"

Name "HN Reader legacy cleanup"
OutFile "legacy-cleanup.exe"
RequestExecutionLevel admin
SilentInstall silent
ShowInstDetails nevershow

Var IniFile
Var LegacyDir
Var TargetData
Var ResultFile
Var RestartService

Function .onInit
  StrCpy $IniFile ""
  StrCpy $LegacyDir ""
  StrCpy $TargetData ""
  StrCpy $ResultFile ""
  StrCpy $RestartService 0

  ${GetParameters} $R0
  ${GetOptions} $R0 "/INI=" $R1
  ; GetOptions may keep the surrounding quotes.
  StrCpy $R2 $R1 1
  ${If} $R2 == '"'
    StrCpy $R1 $R1 -1 1
  ${EndIf}
  StrCpy $IniFile $R1

  ${If} $IniFile == ""
    Quit
  ${EndIf}

  ReadINIStr $LegacyDir "$IniFile" "migrate" "LegacyDir"
  ReadINIStr $TargetData "$IniFile" "migrate" "TargetData"
  ReadINIStr $ResultFile "$IniFile" "migrate" "Result"
FunctionEnd

; Write $R0 as the single-line result. The main installer compares the first two
; characters with "ok"; anything else is shown to the user as an error.
Function WriteResult
  ${If} $ResultFile == ""
    Return
  ${EndIf}
  ; Write a temporary file and rename it, so the main installer never observes
  ; a half-written (or still-locked) result file.
  StrCpy $R2 "$ResultFile.tmp"
  Delete "$R2"
  FileOpen $R1 "$R2" w
  ${If} ${Errors}
    Return
  ${EndIf}
  FileWrite $R1 $R0
  FileClose $R1
  Delete "$ResultFile"
  Rename "$R2" "$ResultFile"
FunctionEnd

; $R0 must contain the message. Restart the old service if it was stopped but
; the migration did not finish, so a failed/aborted migration leaves the old
; working install usable.
Function Fail
  Call WriteResult
  ${If} $RestartService == 1
    nsExec::ExecToLog 'sc start ${SERVICE_NAME}'
  ${EndIf}
  Quit
FunctionEnd

Section
  ${If} $LegacyDir == ""
  ${OrIf} $TargetData == ""
    StrCpy $R0 "error: missing migration arguments"
    Call Fail
  ${EndIf}

  DetailPrint "Migrating $LegacyDir to $TargetData"

  ; Stop and remove the old service if it exists. sc query returns 0 only when
  ; the service entry exists; its state is checked by the process waiter below.
  nsExec::ExecToStack 'sc query ${SERVICE_NAME}'
  Pop $R1
  Pop $R2
  ${If} $R1 == 0
    StrCpy $RestartService 1
    nsExec::ExecToLog 'sc stop ${SERVICE_NAME}'
    Pop $R1

    ; sc stop is graceful; wait for it before falling back to taskkill.
    Call WaitServerProcessGone
    Pop $R1
    ${If} $R1 != 0
      Call StopServerProcess
      Pop $R1
    ${EndIf}
    ${If} $R1 != 0
      StrCpy $R0 "error: hn-reader.exe did not stop within 30 seconds"
      Call Fail
    ${EndIf}

  ${EndIf}

  ; Copy first, delete later. The target data directory is per-user; this helper
  ; runs elevated, but can write to the invoking user's profile.
  CreateDirectory "$TargetData"
  CreateDirectory "$TargetData\backups"
  ${GetTime} "" "L" $0 $1 $2 $3 $4 $5 $6
  StrCpy $R4 "$TargetData\backups\legacy-$0$1$2-$3$4$5"
  CreateDirectory "$R4"

  ${If} ${FileExists} "$LegacyDir\hn-reader.db"
    CopyFiles /SILENT "$LegacyDir\hn-reader.db" "$R4\hn-reader.db"
  ${EndIf}
  ${If} ${FileExists} "$LegacyDir\hn-reader.db-wal"
    CopyFiles /SILENT "$LegacyDir\hn-reader.db-wal" "$R4\hn-reader.db-wal"
  ${EndIf}
  ${If} ${FileExists} "$LegacyDir\hn-reader.db-shm"
    CopyFiles /SILENT "$LegacyDir\hn-reader.db-shm" "$R4\hn-reader.db-shm"
  ${EndIf}
  ${If} ${FileExists} "$LegacyDir\.env"
    CopyFiles /SILENT "$LegacyDir\.env" "$R4\.env"
  ${EndIf}

  ; If a target database already exists, keep it authoritative and only keep the
  ; legacy copy in backups. Otherwise copy the legacy data into the new location.
  ${IfNot} ${FileExists} "$TargetData\hn-reader.db"
    ${If} ${FileExists} "$LegacyDir\hn-reader.db"
      ; Copy to a temporary name first: a crash in the middle must not leave a
      ; partial hn-reader.db that the next run would treat as authoritative.
      Delete "$TargetData\hn-reader.db.migrating"
      CopyFiles /SILENT "$LegacyDir\hn-reader.db" "$TargetData\hn-reader.db.migrating"
      ${IfNot} ${FileExists} "$TargetData\hn-reader.db.migrating"
        StrCpy $R0 "error: target database missing after copy"
        Call Fail
      ${EndIf}
      ; CopyFiles does not report a short write; compare the bytes before the
      ; temporary file becomes the authoritative database.
      nsExec::ExecToLog 'fc /b "$LegacyDir\hn-reader.db" "$TargetData\hn-reader.db.migrating"'
      Pop $R5
      ${If} $R5 != 0
        Delete "$TargetData\hn-reader.db.migrating"
        StrCpy $R0 "error: database copy verification failed"
        Call Fail
      ${EndIf}
      Rename "$TargetData\hn-reader.db.migrating" "$TargetData\hn-reader.db"
      ${IfNot} ${FileExists} "$TargetData\hn-reader.db"
        StrCpy $R0 "error: target database rename failed"
        Call Fail
      ${EndIf}
    ${EndIf}
    ${If} ${FileExists} "$LegacyDir\hn-reader.db-wal"
      CopyFiles /SILENT "$LegacyDir\hn-reader.db-wal" "$TargetData\hn-reader.db-wal"
      ${IfNot} ${FileExists} "$TargetData\hn-reader.db-wal"
        Delete "$TargetData\hn-reader.db"
        Delete "$TargetData\hn-reader.db-wal"
        Delete "$TargetData\hn-reader.db-shm"
        StrCpy $R0 "error: WAL copy failed"
        Call Fail
      ${EndIf}
    ${EndIf}
    ${If} ${FileExists} "$LegacyDir\hn-reader.db-shm"
      CopyFiles /SILENT "$LegacyDir\hn-reader.db-shm" "$TargetData\hn-reader.db-shm"
      ${IfNot} ${FileExists} "$TargetData\hn-reader.db-shm"
        Delete "$TargetData\hn-reader.db"
        Delete "$TargetData\hn-reader.db-wal"
        Delete "$TargetData\hn-reader.db-shm"
        StrCpy $R0 "error: SHM copy failed"
        Call Fail
      ${EndIf}
    ${EndIf}
    ${If} ${FileExists} "$LegacyDir\.env"
    ${AndIfNot} ${FileExists} "$TargetData\.env"
      CopyFiles /SILENT "$LegacyDir\.env" "$TargetData\.env"
    ${EndIf}
  ${EndIf}

  ; The data is safely in place. Now remove the old service entry and only then
  ; delete the old program files.
  ${If} $RestartService == 1
    StrCpy $R3 0
    ${Do}
      nsExec::ExecToLog 'sc delete ${SERVICE_NAME}'
      Pop $R1
      ${If} $R1 == 0
      ${OrIf} $R1 == 1060
        ${ExitDo}
      ${EndIf}
      Sleep 1000
      IntOp $R3 $R3 + 1
    ${LoopUntil} $R3 >= 10
    ${If} $R1 != 0
    ${AndIf} $R1 != 1060
      StrCpy $R0 "error: sc delete failed with code $R1"
      Call Fail
    ${EndIf}
    StrCpy $RestartService 0
  ${EndIf}
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

  ; Only after the target database is in place do we remove the old files.
  Delete "$LegacyDir\hn-reader.exe"
  Delete "$LegacyDir\.env"
  Delete "$LegacyDir\hn-reader.db"
  Delete "$LegacyDir\hn-reader.db-wal"
  Delete "$LegacyDir\hn-reader.db-shm"
  Delete "$LegacyDir\hn-reader.log"
  Delete "$LegacyDir\uninstall.exe"
  RMDir "$LegacyDir"

  StrCpy $R0 "ok"
  Call WriteResult
SectionEnd
