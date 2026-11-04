; Little Timer Windows 安装器（按用户安装，免 UAC）
; 构建: cd packaging/windows && makensis -DVERSION=<ver> -XOutFile=LittleTimer-setup.exe little-timer.nsi
; 产物内含: LittleTimer.exe + 图标 + WebView2 evergreen bootstrapper（缺失时静默补装运行时）

Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"

!define APPNAME "Little Timer"
!define APPEXE "LittleTimer.exe"
!define PUBLISHER "Little Timer Maintainers"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\LittleTimer"
!define WEBVIEW2_GUID "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

!ifndef VERSION
  !error "VERSION not defined; pass -DVERSION=<ver>"
!endif
; VIProductVersion 要求严格的 X.X.X.X，而版本号可能带非数字后缀
; （例如临时验收 tag v0.0.0ci）。FILEVER 由调用方算出四段数字；
; 没给时退回 "<ver>.0"，纯数字版本仍按原样工作。
!ifndef FILEVER
  !define FILEVER "${VERSION}.0"
!endif

Name "${APPNAME}"
OutFile "LittleTimer-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\LittleTimer"
InstallDirRegKey HKCU "Software\Little Timer" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma

VIProductVersion "${FILEVER}"
VIAddVersionKey "ProductName" "${APPNAME}"
VIAddVersionKey "FileDescription" "${APPNAME}"
VIAddVersionKey "FileVersion" "${FILEVER}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "LegalCopyright" "${PUBLISHER}"

!define MUI_ICON "little-timer.ico"
!define MUI_UNICON "little-timer.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APPEXE}"
!define MUI_FINISHPAGE_RUN_TEXT "启动 ${APPNAME}"

!insertmacro MUI_PAGE_LICENSE "LICENSE.txt"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  SetShellVarContext current
FunctionEnd

Function un.onInit
  SetShellVarContext current
FunctionEnd

Section "核心组件" SecCore
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "build\${APPEXE}"
  File "build\MicrosoftEdgeWebview2Setup.exe"
  CreateDirectory "$SMPROGRAMS\${APPNAME}"
  CreateShortCut "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk" "$INSTDIR\${APPEXE}"
  CreateShortCut "$SMPROGRAMS\${APPNAME}\卸载.lnk" "$INSTDIR\Uninstall.exe"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "Software\Little Timer" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}.0"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "${UNINSTKEY}" "NoModify" "1"
  WriteRegStr HKCU "${UNINSTKEY}" "NoRepair" "1"

  ; WebView2 运行时：64 位与 32 位视图各查一次，两处都为空才补装。
  ; SetRegView 只能写在 Section/Function 里（顶层是编译错误），且只在这一段切换：
  ; 上面那些 HKCU 写入留在默认视图，与 InstallDirRegKey / 卸载器的删除同视图。
  SetRegView 64
  ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 == ""
    SetRegView 32
    ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${EndIf}
  SetRegView 32
  ${If} $0 == ""
    DetailPrint "未检测到 WebView2 运行时，正在静默安装..."
    ExecWait '"$INSTDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install'
  ${EndIf}
SectionEnd

Section /o "桌面快捷方式" SecDesktop
  CreateShortCut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\${APPEXE}"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\${APPEXE}"
  Delete "$INSTDIR\Uninstall.exe"
  Delete "$INSTDIR\MicrosoftEdgeWebview2Setup.exe"
  RMDir /r "$INSTDIR"
  RMDir /r "$SMPROGRAMS\${APPNAME}"
  Delete "$DESKTOP\${APPNAME}.lnk"
  DeleteRegKey HKCU "${UNINSTKEY}"
  DeleteRegKey HKCU "Software\Little Timer"
  ; 故意不删 %AppData%\LittleTimer：主库与 secret.db 是用户数据
SectionEnd
