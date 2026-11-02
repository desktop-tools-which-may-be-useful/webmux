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
; Exit codes: 0 done, 2 wrong Windows architecture, 3 a running tray could
; not be closed (nothing was written).

!ifndef VERSION | ARCH | TRAY | SERVER | ICON | OUTFILE
  !error "VERSION, ARCH, TRAY, SERVER, ICON and OUTFILE must be defined with -D"
!endif

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

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
; it runs from $INSTDIR; a tray elsewhere is the user's to close.
!macro CloseTray UN EXE
  Function ${UN}closeTray
    retry:
      ClearErrors
      ExecWait '"${EXE}" quit --kill-path "$INSTDIR\webmux-tray.exe"' $0
      ${If} ${Errors}
        StrCpy $0 0 ; no tray executable to ask: nothing of ours can run
      ${EndIf}
      ${If} $0 == 0
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

; 1 writes the login item. Silent installs keep the default.
Var Autostart
Var AutostartBox

Function .onInit
  StrCpy $Autostart ${BST_CHECKED}
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
FunctionEnd

; "安装选项": whether the tray starts when the user signs in to Windows.
Function optionsPage
  !insertmacro MUI_HEADER_TEXT "安装选项" "选择是否开机后自动启动 WebMux。"
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateCheckbox} 0 0 100% 12u "开机后自动启动 WebMux（登录 Windows 时启动托盘）"
  Pop $AutostartBox
  ${NSD_SetState} $AutostartBox $Autostart
  ${NSD_CreateLabel} 12u 18u -12u 36u "托盘启动后会按托盘设置在 WSL 中启动 WebMux。以后可在托盘菜单的“登录时自动启动”中更改。"
  Pop $0
  nsDialogs::Show
FunctionEnd

Function optionsLeave
  ${NSD_GetState} $AutostartBox $Autostart
FunctionEnd

Section "-WebMux tray"
  ; Close the running tray with the new executable: an installed older
  ; tray may not know `quit` yet.
  InitPluginsDir
  File "/oname=$PLUGINSDIR\webmux-tray.exe" "${TRAY}"
  Call closeTray

  SetOutPath "$INSTDIR"
  File "/oname=webmux-tray.exe" "${TRAY}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortcut "$SMPROGRAMS\WebMux.lnk" "$INSTDIR\webmux-tray.exe"

  ; Replace the server an earlier version pre-seeded.
  ReadRegStr $1 HKCU "${UNINST_KEY}" "PreseededServer"
  ${If} $1 != ""
  ${AndIf} $1 != "${CACHE_DIR}\${CACHE_FILE}"
    Delete "$1"
  ${EndIf}
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
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" $0

  ${If} $Autostart == ${BST_CHECKED}
    WriteRegStr HKCU "${RUN_KEY}" "${RUN_VALUE}" '"$INSTDIR\webmux-tray.exe"'
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
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINST_KEY}"
SectionEnd
