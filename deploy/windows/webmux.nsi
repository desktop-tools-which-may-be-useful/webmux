; WebMux Windows installer: a per-user install of the tray plus the Linux
; server of the same build, pre-seeded where the tray looks for downloads.
;
; Built by `make installer`; every input comes from -D:
;   VERSION  release version, x.y.z
;   ARCH     amd64 | arm64 (Windows and pre-seeded Linux server)
;   TRAY     absolute path of webmux-tray-windows-<ARCH>.exe
;   SERVER   absolute path of webmux-linux-<ARCH>
;   ICON     absolute path of the installer icon
;   OUTFILE  absolute output path
;
; Exit codes: 0 done, 2 wrong Windows architecture or a failure while no
; tray had been closed (none ran, or it had not been closed yet), 3 a
; running tray could not be closed (nothing was written), 4 a failure after
; the installed tray was closed (that tray was started again).
;
; Silent mode (/S) over an existing install keeps its directory and the
; login item as they are; internal/tray updates itself this way.
;
; The only administrator prompt is the optional firewall rule of the
; interactive install ("安装选项"): `webmux-tray firewall allow` adds it for
; the installed tray and FirewallRule records that tray's path. Unchecking
; the option on a later interactive install, or uninstalling, removes the
; recorded rule. Silent installs never prompt and leave the rule as it is;
; a silent update in place keeps it, since the rule follows the tray's path.

!ifndef VERSION | ARCH | TRAY | SERVER | ICON | OUTFILE
  !error "VERSION, ARCH, TRAY, SERVER, ICON and OUTFILE must be defined with -D"
!endif

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user
; A file that cannot be written fails the install instead of being skipped,
; so .onInstFailed can restart the tray.
AllowSkipFiles off

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux"
; Same value name and quoting as internal/tray/winui/autostart.go, so the
; tray's "登录时自动启动" check box recognises the entry.
!define RUN_KEY "Software\Microsoft\Windows\CurrentVersion\Run"
!define RUN_VALUE "WebMuxTray"
; Same name as Installer.Install in internal/tray/install.go.
!define CACHE_DIR "$LOCALAPPDATA\WebMux\downloads"
!define CACHE_FILE "webmux-${VERSION}-linux-${ARCH}"

!if "${ARCH}" == "amd64"
  !define OTHER_ARCH "arm64"
!else if "${ARCH}" == "arm64"
  !define OTHER_ARCH "amd64"
!else
  !error "ARCH must be amd64 or arm64"
!endif

Name "WebMux"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\WebMux"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation"
BrandingText "WebMux ${VERSION}"

VIProductVersion "${VERSION}.0"
VIFileVersion "${VERSION}.0"
VIAddVersionKey /LANG=2052 "ProductName" "WebMux"
VIAddVersionKey /LANG=2052 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=2052 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=2052 "FileDescription" "WebMux 安装程序（${ARCH}）"
VIAddVersionKey /LANG=2052 "LegalCopyright" "WebMux contributors"

!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_TEXT "WebMux 托盘已安装并启动。$\r$\n$\r$\n首次使用时，在托盘菜单中选择“安装 WSL 中的 webmux”；安装包已内置 ${ARCH} 版服务端，WSL 为同一架构时无需联网。"
!define MUI_UNFINISHPAGE_TEXT "WebMux 托盘已卸载。$\r$\n$\r$\n以下内容已保留：托盘设置（%APPDATA%\WebMux）、托盘日志（%LOCALAPPDATA%\WebMux）和 WSL 中的 webmux。$\r$\n$\r$\n如果 WebMux 正在运行，它会继续在 WSL 中运行；需要停止时在 WSL 中执行 webmux stop。"

!insertmacro MUI_PAGE_DIRECTORY
Page custom optionsPage optionsLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH
!insertmacro MUI_LANGUAGE "SimpChinese"

; CloseTray asks the tray started from $INSTDIR (and any tray of this user)
; to quit as with "仅退出托盘". `quit` kills an unresponsive tray only when
; it runs from $INSTDIR; a tray elsewhere is the user's to close. It
; returns with $0 = 0 when a running tray was closed and 3 when none ran
; (a tray that predates exit code 3 also reports none as 0).
!macro CloseTray UN EXE
  Function ${UN}closeTray
    retry:
      ClearErrors
      ExecWait '"${EXE}" quit --kill-path "$INSTDIR\webmux-tray.exe"' $0
      ${If} ${Errors}
        StrCpy $0 3 ; no tray executable to ask: nothing of ours can run
      ${EndIf}
      ${If} $0 == 0
      ${OrIf} $0 == 3
        Return
      ${EndIf}
      IfSilent fail
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "另一个 WebMux 托盘仍在运行，无法自动关闭（可能是旧版本或其他位置的托盘）。$\r$\n$\r$\n请在通知区域右键 WebMux 图标，选择“退出 → 仅退出托盘”，然后点击“重试”。" IDRETRY retry
    fail:
      SetErrorLevel 3
      Quit
  FunctionEnd
!macroend
!insertmacro CloseTray "" "$PLUGINSDIR\webmux-tray.exe"
!insertmacro CloseTray "un." "$INSTDIR\webmux-tray.exe"

; ${BST_CHECKED} writes the login item. Silent installs keep the default;
; a silent update leaves the item as it is.
Var Autostart
Var AutostartBox
; ${BST_CHECKED} keeps a firewall rule for the installed tray: the first
; interactive install defaults to it, a later one to the recorded choice.
; Silent installs never touch the rule.
Var Firewall
Var FirewallBox
; The tray path whose rule an earlier install added (FirewallRule), or "".
Var FirewallOld
; 1 once a running installed tray has been closed.
Var TrayClosed
; 1 while webmux-tray.exe.old holds the installed tray, until the install
; succeeds.
Var TrayBackup

Function .onInit
  StrCpy $Autostart ${BST_CHECKED}
  StrCpy $Firewall ${BST_CHECKED}
  ReadRegStr $FirewallOld HKCU "${UNINST_KEY}" "FirewallRule"
  ${If} ${Silent}
    StrCpy $Firewall ${BST_UNCHECKED}
  ${Else}
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
    ${AndIf} $FirewallOld == ""
      StrCpy $Firewall ${BST_UNCHECKED}
    ${EndIf}
  ${EndIf}
  StrCpy $TrayClosed 0
  StrCpy $TrayBackup 0
  !if "${ARCH}" == "amd64"
    ${IfNot} ${IsNativeAMD64}
  !else
    ${IfNot} ${IsNativeARM64}
  !endif
    IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "此安装包适用于 ${ARCH} 版 Windows，当前设备请改用 webmux-setup-windows-${OTHER_ARCH}.exe。"
    SetErrorLevel 2
    Quit
  ${EndIf}
  ; A silent update over an existing install (the tray's self-update)
  ; stays in its directory, whatever /D says, and neither re-adds a login
  ; item the user removed nor changes one that exists.
  ${If} ${Silent}
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
      StrCpy $INSTDIR $0
      StrCpy $Autostart ${BST_UNCHECKED}
    ${EndIf}
  ${EndIf}
FunctionEnd

; A failed write may have left webmux-tray.exe incomplete, so the tray
; comes back from its backup. Any failure after the running tray was
; closed starts it again, so the user keeps a tray; the exit code tells the
; caller the update failed.
Function .onInstFailed
  ${If} $TrayBackup == 1
    Delete "$INSTDIR\webmux-tray.exe"
    Rename "$INSTDIR\webmux-tray.exe.old" "$INSTDIR\webmux-tray.exe"
  ${EndIf}
  ${If} $TrayClosed == 1
    ${If} ${FileExists} "$INSTDIR\webmux-tray.exe"
      SetOutPath "$PROFILE"
      Exec '"$INSTDIR\webmux-tray.exe"'
    ${EndIf}
    SetErrorLevel 4
  ${EndIf}
FunctionEnd

; "安装选项": whether the tray starts when the user signs in to Windows,
; and whether other devices may reach the tray's forwarding.
Function optionsPage
  !insertmacro MUI_HEADER_TEXT "安装选项" "选择开机启动与防火墙设置。"
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateCheckbox} 0 0 100% 12u "开机后自动启动 WebMux（登录 Windows 时启动托盘）"
  Pop $AutostartBox
  ${NSD_SetState} $AutostartBox $Autostart
  ${NSD_CreateLabel} 12u 14u -12u 24u "托盘启动后会按托盘设置在 WSL 中启动 WebMux。以后可在托盘菜单的“登录时自动启动”中更改。"
  Pop $0
  ${NSD_CreateCheckbox} 0 44u 100% 12u "允许其他设备通过托盘转发访问 WebMux（添加 Windows 防火墙规则）"
  Pop $FirewallBox
  ${NSD_SetState} $FirewallBox $Firewall
  ${NSD_CreateLabel} 12u 58u -12u 36u "只放行专用网络与域网络上对托盘的入站连接，不放行公用网络。安装时会弹出一次管理员确认；不使用托盘转发可以取消勾选。"
  Pop $0
  nsDialogs::Show
FunctionEnd

Function optionsLeave
  ${NSD_GetState} $AutostartBox $Autostart
  ${NSD_GetState} $FirewallBox $Firewall
FunctionEnd

Section "-WebMux tray"
  ; Close the running tray with the new executable: an installed older
  ; tray may not know `quit` yet.
  InitPluginsDir
  File "/oname=$PLUGINSDIR\webmux-tray.exe" "${TRAY}"
  Call closeTray
  ; From here on a failure restores the backup and, when a tray was closed,
  ; restarts it (.onInstFailed): File aborts by itself (AllowSkipFiles
  ; off), other writes are checked through the error flag below.
  ${If} $0 == 0
    StrCpy $TrayClosed 1
  ${EndIf}
  ClearErrors

  SetOutPath "$INSTDIR"
  ; File overwrites in place, so the installed tray moves aside first and
  ; stays until the install succeeds. A backup an interrupted update left
  ; behind is the last complete tray when webmux-tray.exe is missing.
  ${If} ${FileExists} "$INSTDIR\webmux-tray.exe"
    Delete "$INSTDIR\webmux-tray.exe.old"
    Rename "$INSTDIR\webmux-tray.exe" "$INSTDIR\webmux-tray.exe.old"
    ${If} ${Errors}
      Abort "无法写入安装目录 $INSTDIR"
    ${EndIf}
    StrCpy $TrayBackup 1
  ${ElseIf} ${FileExists} "$INSTDIR\webmux-tray.exe.old"
    StrCpy $TrayBackup 1
  ${EndIf}
  File "/oname=webmux-tray.exe" "${TRAY}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  ${If} ${Errors}
    Abort "无法写入安装目录 $INSTDIR"
  ${EndIf}
  ; The shortcut and removing the old server are best effort.
  CreateShortcut "$SMPROGRAMS\WebMux.lnk" "$INSTDIR\webmux-tray.exe"

  ; Replace the server an earlier version pre-seeded.
  ReadRegStr $1 HKCU "${UNINST_KEY}" "PreseededServer"
  ${If} $1 != ""
  ${AndIf} $1 != "${CACHE_DIR}\${CACHE_FILE}"
    Delete "$1"
  ${EndIf}
  ClearErrors
  CreateDirectory "${CACHE_DIR}"
  File "/oname=${CACHE_DIR}\${CACHE_FILE}" "${SERVER}"

  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "WebMux"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "WebMux"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\webmux-tray.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegStr HKCU "${UNINST_KEY}" "PreseededServer" "${CACHE_DIR}\${CACHE_FILE}"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  ${If} ${Errors}
    Abort "无法写入下载缓存或注册表"
  ${EndIf}
  StrCpy $TrayBackup 0
  Delete "$INSTDIR\webmux-tray.exe.old"
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" $0

  ${If} $Autostart == ${BST_CHECKED}
    WriteRegStr HKCU "${RUN_KEY}" "${RUN_VALUE}" '"$INSTDIR\webmux-tray.exe"'
  ${EndIf}

  ; Optional and after the install is complete: a declined or failed
  ; prompt only explains how to add the rule later. FirewallRule records
  ; the tray path, so a rule for an earlier directory is removed too.
  ${If} $Firewall == ${BST_CHECKED}
    DetailPrint "添加 Windows 防火墙规则（需要管理员确认）…"
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall allow' $0
    ${If} ${Errors}
      StrCpy $0 1
    ${EndIf}
    ClearErrors
    ${If} $0 == 0
      WriteRegStr HKCU "${UNINST_KEY}" "FirewallRule" "$INSTDIR\webmux-tray.exe"
      ${If} $FirewallOld != ""
      ${AndIf} $FirewallOld != "$INSTDIR\webmux-tray.exe"
        ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$FirewallOld"' $0
        ClearErrors
      ${EndIf}
    ${ElseIf} $0 == 3
      MessageBox MB_OK|MB_ICONINFORMATION "已跳过 Windows 防火墙规则，其他设备暂时无法通过托盘转发访问。$\r$\n$\r$\n需要时重新运行安装程序并保持勾选，或在命令行运行：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall allow" /SD IDOK
    ${Else}
      MessageBox MB_OK|MB_ICONEXCLAMATION "未能添加 Windows 防火墙规则（代码 $0）。托盘已安装，本机使用不受影响。$\r$\n$\r$\n需要时可在命令行重试：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall allow" /SD IDOK
    ${EndIf}
  ${ElseIfNot} ${Silent}
  ${AndIf} $FirewallOld != ""
    ; Unchecked on an interactive reinstall: withdraw the earlier rule.
    DetailPrint "删除 Windows 防火墙规则（需要管理员确认）…"
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$FirewallOld"' $0
    ${If} ${Errors}
      StrCpy $0 1
    ${EndIf}
    ClearErrors
    ${If} $0 == 0
      DeleteRegValue HKCU "${UNINST_KEY}" "FirewallRule"
    ${Else}
      MessageBox MB_OK|MB_ICONINFORMATION "未删除之前添加的 Windows 防火墙规则（代码 $0）。$\r$\n$\r$\n需要时可在命令行运行：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall remove" /SD IDOK
    ${EndIf}
  ${EndIf}
SectionEnd

Section "-Start tray"
  ; The working directory must not be $INSTDIR, or the running tray would
  ; keep it from being removed.
  SetOutPath "$PROFILE"
  Exec '"$INSTDIR\webmux-tray.exe"'
SectionEnd

Section "Uninstall"
  Call un.closeTray

  ; The rule the installer added for this tray; removing it shows the
  ; administrator prompt once.
  ReadRegStr $1 HKCU "${UNINST_KEY}" "FirewallRule"
  ${If} $1 != ""
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$1"' $0
    ${If} ${Errors}
    ${OrIf} $0 != 0
      MessageBox MB_OK|MB_ICONINFORMATION "未删除 WebMux 添加的 Windows 防火墙规则。$\r$\n$\r$\n可在“高级安全 Windows Defender 防火墙”的入站规则中删除以“WebMux tray forwarding”开头的规则。" /SD IDOK
    ${EndIf}
    ClearErrors
  ${EndIf}

  ReadRegStr $1 HKCU "${RUN_KEY}" "${RUN_VALUE}"
  ${If} $1 == '"$INSTDIR\webmux-tray.exe"'
    DeleteRegValue HKCU "${RUN_KEY}" "${RUN_VALUE}"
  ${EndIf}

  ReadRegStr $1 HKCU "${UNINST_KEY}" "PreseededServer"
  ${If} $1 != ""
    Delete "$1"
  ${EndIf}
  RMDir "${CACHE_DIR}"

  Delete "$SMPROGRAMS\WebMux.lnk"
  Delete "$INSTDIR\webmux-tray.exe"
  Delete "$INSTDIR\webmux-tray.exe.old"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINST_KEY}"
SectionEnd
