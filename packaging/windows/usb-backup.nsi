; SPDX-License-Identifier: GPL-3.0-or-later
; Copyright (C) 2026 Maurice Snoeren
;
; This file is part of USB Backup.
;
; USB Backup is free software: you can redistribute it and/or modify it under
; the terms of the GNU General Public License as published by the Free Software
; Foundation, either version 3 of the License, or (at your option) any later
; version.
;
; USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
; WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
; PARTICULAR PURPOSE. See the GNU General Public License for more details.
;
; You should have received a copy of the GNU General Public License along with
; USB Backup. If not, see <https://www.gnu.org/licenses/>.

; USB Backup - Windows setup (NSIS 3, builds on Linux with makensis).
;   makensis -DVERSION=1.2.3 -DDIST=../../dist -DOUT=../../dist/USB-Backup-Setup.exe packaging/windows/usb-backup.nsi
; Silent install: USB-Backup-Setup.exe /S
; Uninstall keeps the configuration, the vault and everything on the USB
; disks, unless "Remove settings" is ticked in the uninstaller.

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel admin

!ifndef VERSION
  !define VERSION "0.0.0-dev"
!endif
!ifndef DIST
  !define DIST "..\..\dist"
!endif
!ifndef OUT
  !define OUT "USB-Backup-Setup.exe"
!endif

!define APP "USB Backup"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\USB Backup"

Name "${APP} ${VERSION}"
OutFile "${OUT}"
InstallDir "$PROGRAMFILES64\${APP}"
InstallDirRegKey HKLM "${UNINSTKEY}" "InstallLocation"
BrandingText "${APP} ${VERSION}"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!define MUI_ICON "usb-backup.ico"
!define MUI_UNICON "usb-backup.ico"
!define MUI_FINISHPAGE_RUN "$INSTDIR\usb-backup.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Open USB Backup to set it up (sources, disks, keys, schedule)"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${DIST}\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_COMPONENTS
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Dutch"

Var PD ; C:\ProgramData

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "USB Backup needs 64-bit Windows."
    Abort
  ${EndIf}
  SetRegView 64
  SetShellVarContext all
  ReadEnvStr $PD "ProgramData"
  StrCmp $PD "" 0 +2
    StrCpy $PD "C:\ProgramData"
FunctionEnd

Section "USB Backup (required)" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  ; Stop a running window before files are replaced (update).
  nsExec::Exec '"$SYSDIR\taskkill.exe" /IM usb-backup.exe /F'
  File "${DIST}\usb-backup.exe"
  File "${DIST}\usb-backup-cli.exe"
  File "${DIST}\README.md"
  File "${DIST}\LICENSE"
  File "${DIST}\THIRD-PARTY-NOTICES.txt"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Configuration folder: only administrators and SYSTEM may write; users
  ; read. The logs subfolder is writable for users, who run the backups.
  CreateDirectory "$PD\${APP}"
  CreateDirectory "$PD\${APP}\logs"
  nsExec::ExecToLog '"$SYSDIR\icacls.exe" "$PD\${APP}" /inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX'
  nsExec::ExecToLog '"$SYSDIR\icacls.exe" "$PD\${APP}\logs" /grant *S-1-5-32-545:(OI)(CI)M'

  ; Apps and features.
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayName" "${APP}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "Publisher" "USB Backup"
  WriteRegStr HKLM "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\usb-backup.exe"
  WriteRegStr HKLM "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKLM "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoRepair" 1
SectionEnd

Section "Start menu shortcut" SecStartMenu
  CreateShortCut "$SMPROGRAMS\${APP}.lnk" "$INSTDIR\usb-backup.exe"
SectionEnd

Section /o "Desktop shortcut" SecDesktop
  CreateShortCut "$DESKTOP\${APP}.lnk" "$INSTDIR\usb-backup.exe"
SectionEnd

Section "Add usb-backup-cli to PATH" SecPath
  ; PowerShell adds the folder to the machine PATH once (no duplicate).
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -NonInteractive -Command "$$d = $\'$INSTDIR$\'; $$p = [Environment]::GetEnvironmentVariable($\'Path$\', $\'Machine$\'); if (($$p -split $\';$\') -notcontains $$d) { [Environment]::SetEnvironmentVariable($\'Path$\', $$p.TrimEnd($\';$\') + $\';$\' + $$d, $\'Machine$\') }"'
SectionEnd

Function .onInstSuccess
  ; The wizard (settings screen) opens via the finish page; nothing runs
  ; automatically after a silent install.
FunctionEnd

; --- Uninstaller ---

Function un.onInit
  SetRegView 64
  SetShellVarContext all
  ReadEnvStr $PD "ProgramData"
  StrCmp $PD "" 0 +2
    StrCpy $PD "C:\ProgramData"
FunctionEnd

Section "un.USB Backup" UnSecMain
  SectionIn RO
  ; Remove the scheduled tasks first, while the CLI is still there.
  nsExec::ExecToLog '"$INSTDIR\usb-backup-cli.exe" uninstall --trigger all'
  nsExec::Exec '"$SYSDIR\taskkill.exe" /IM usb-backup.exe /F'
  Delete "$INSTDIR\usb-backup.exe"
  Delete "$INSTDIR\usb-backup-cli.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\THIRD-PARTY-NOTICES.txt"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP}.lnk"
  Delete "$DESKTOP\${APP}.lnk"
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -NonInteractive -Command "$$d = $\'$INSTDIR$\'; $$p = [Environment]::GetEnvironmentVariable($\'Path$\', $\'Machine$\'); [Environment]::SetEnvironmentVariable($\'Path$\', (($$p -split $\';$\') | Where-Object { $$_ -ne $$d }) -join $\';$\', $\'Machine$\')"'
  DeleteRegKey HKLM "${UNINSTKEY}"
SectionEnd

Section /o "un.Remove settings, registrations and log (not the vault, not the USB disks)" UnSecConfig
  RMDir /r "$PD\${APP}"
SectionEnd

Section /o "un.Remove my vault (recovery.vault) - only if you have the break-glass key" UnSecVault
  ; The vault is per user (%APPDATA%) of the user running the uninstaller.
  SetShellVarContext current
  Delete "$APPDATA\${APP}\recovery.vault"
  RMDir "$APPDATA\${APP}"
  SetShellVarContext all
SectionEnd
