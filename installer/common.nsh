; Shared installer helpers for the per-user HN Reader install.

!include "LogicLib.nsh"
!include "FileFunc.nsh"

; Wait for hn-reader.exe to exit, without killing it.
; Pushes 0 when the process is gone, 1 after a 30 second timeout.
!macro WaitProcessBody
  StrCpy $R1 0
  ${Do}
    Sleep 500
    ; tasklist prints the image name when the process exists. find exits 0 only
    ; when it matched, which is independent of the Windows display language.
    nsExec::ExecToStack 'cmd /c tasklist /FI "IMAGENAME eq hn-reader.exe" /NH | find /I "hn-reader.exe" >nul'
    Pop $R0
    Pop $R2
    ${If} $R0 != 0
      Push 0
      Return
    ${EndIf}
    IntOp $R1 $R1 + 1
  ${LoopUntil} $R1 >= 60
  Push 1
!macroend

Function WaitServerProcessGone
  !insertmacro WaitProcessBody
FunctionEnd

; Force-stop the process (used for the per-user app, which has no service stop
; control) and wait until it is really gone. Pushes 0 on success, 1 on timeout.
Function StopServerProcess
  nsExec::ExecToLog 'taskkill /IM hn-reader.exe /F'
  Pop $R0
  Call WaitServerProcessGone
  Pop $R0
  Push $R0
FunctionEnd

; Strip surrounding double quotes from a /NAME="value" option.
!macro Unquote VAR
  StrCpy $R9 ${VAR} 1
  ${If} $R9 == '"'
    StrCpy ${VAR} ${VAR} -1 1
  ${EndIf}
!macroend
