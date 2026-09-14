package winexe

import (
	"text/template"
)

// nsiTemplate renders an NSIS script covering the "basics": app metadata, an
// optional license page, copying the binary plus payload files/trees under
// $INSTDIR, a registry install-dir marker (for InstallDirRegKey), an
// unconditional Start Menu shortcut (plus, when the project author opts in,
// real end-user-toggleable Desktop shortcut and Run-at-Startup checkboxes
// and/or an optional "launch it now" finish page - see InstallExperience
// below), a Programs and Features registration, file associations (see
// FileAssociations below), and an uninstaller that kills the running exe
// before deleting everything. No signing — matches this tool's
// deliberately-trimmed scope, same as the deb/rpm/macpkg backends.
//
// Uses Modern UI 2 (MUI2.nsh, bundled with every NSIS install — no extra
// tool/resource needed) rather than NSIS's old "classic" bare Page
// directives: classic UI's wizard window is a small fixed size that reads
// as noticeably smaller/cheaper than Inno Setup's default, especially for
// the license page's text box. MUI2's wizard window is bigger and matches
// what most users expect from a modern installer. Note this is a one-time
// pick between NSIS's two built-in UI styles, not a size *this tool* can
// dial up or down — NSIS's own dialog dimensions are fixed at compile time
// in a binary resource template (Contrib\UIs\modern.exe), so there's no
// per-build "small/medium/large" knob to expose; genuinely custom
// dimensions would mean shipping/maintaining a hand-resized resource-hacked
// UI binary, out of proportion to what this gains.
//
// Unicode is left at NSIS's modern default (true) here; see build.go's
// comment on why a local dev-only workaround exists for verifying this
// template compiles at all on this machine's specific makensis build.
var nsiTemplate = template.Must(template.New("app.nsi").Parse(`Unicode true
{{- if .IconFile}}
!define MUI_ICON "{{.IconFile}}"
!define MUI_UNICON "{{.IconFile}}"
{{- end}}
!include "MUI2.nsh"

!define APP_NAME "{{.AppName}}"
!define APP_VERSION "{{.AppVersion}}"
!define APP_PUBLISHER "{{.Publisher}}"
!define APP_EXE "{{.ExeName}}"

Name "${APP_NAME}"
OutFile "{{.OutFile}}"
InstallDir "{{.InstallDir}}"
InstallDirRegKey HKCU "Software\${APP_NAME}" "Install_Dir"
RequestExecutionLevel {{if .PerUser}}user{{else}}admin{{end}}
{{- if .IconFile}}
Icon "{{.IconFile}}"
UninstallIcon "{{.IconFile}}"
{{- end}}

{{if .LicenseSource -}}
!insertmacro MUI_PAGE_LICENSE "{{.LicenseSource}}"
{{end -}}
{{if or .DesktopShortcut .AutostartAtLogin -}}
!insertmacro MUI_PAGE_COMPONENTS
{{end -}}
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
{{if .LaunchAfterInstall -}}
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_CHECKED
!insertmacro MUI_PAGE_FINISH
{{end -}}

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

!include "FileFunc.nsh"
!insertmacro GetParameters
!insertmacro GetOptions
!insertmacro GetSize

; -?/-help/-h/--help/-/?//help all print a usage MessageBox and exit,
; instead of NSIS's default behavior of silently launching the normal
; wizard for any unrecognized argument - matches this project's own CLI
; convention (installerbear -help) and gives Setup.exe the same
; "try -help first" discoverability msiexec's own /? already provides on
; the .msi side. Real motivation, from the person who asked for this:
; "any time I need silent install strings... I try -? or -help first...
; then curse the packager and start hunting repo/home page docs".
!macro CheckHelpFlag flag
  ClearErrors
  ${GetOptions} $R0 "${flag}" $R1
  IfErrors +3
  MessageBox MB_OK "${APP_NAME} ${APP_VERSION} installer$\r$\n$\r$\nUsage: <installer>.exe [/S] [/D=install_dir]$\r$\n$\r$\n  /S           Silent install, no UI{{if .LicenseSource}} (also skips the license page below){{end}}$\r$\n  /D=<path>    Install to <path> instead of the default - must be the LAST argument, unquoted"
  Quit
!macroend

Function .onInit
  ${GetParameters} $R0
  !insertmacro CheckHelpFlag "/?"
  !insertmacro CheckHelpFlag "/help"
  !insertmacro CheckHelpFlag "-?"
  !insertmacro CheckHelpFlag "-h"
  !insertmacro CheckHelpFlag "-help"
  !insertmacro CheckHelpFlag "--help"
FunctionEnd

Section "${APP_NAME}" SEC01
  ; RO (read-only): always installed, can't be unchecked - the Components
  ; page (only shown at all when DesktopShortcut and/or AutostartAtLogin is
  ; enabled, see above) would otherwise let someone uncheck the entire app,
  ; leaving nothing installed at all.
  SectionIn RO
{{- if not .PerUser}}
  ; All-users installs default to the per-user Start Menu/Desktop shell
  ; folders otherwise (NSIS's own default context) even though the install
  ; itself is elevated - this redirects those shell-folder constants to
  ; the all-users ones instead, matching ALLUSERS=1's meaning on the .msi
  ; side. Global for the rest of this run (and the uninstaller's own run,
  ; which sets it again below), not just this Section.
  SetShellVarContext all
{{- end}}
  SetOutPath "$INSTDIR"
  File "{{.BinarySource}}"
{{- if .LicenseSource}}
  File "/oname=License.txt" "{{.LicenseSource}}"
{{- end}}
{{range .Files}}
  SetOutPath "$INSTDIR{{if .DestDir}}\{{.DestDir}}{{end}}"
{{- if .Recursive}}
  File /r "{{.Source}}\*.*"
{{- else}}
  File "{{.Source}}"
{{- end}}
{{end}}
  WriteRegStr HKCU "Software\${APP_NAME}" "Install_Dir" "$INSTDIR"
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  ; Registers with Windows' "Apps & Features"/Programs and Features -
  ; NSIS never does this on its own (unlike MSI, which is always tracked
  ; by the Windows Installer service itself); every properly-built NSIS
  ; installer writes these keys by hand. DisplayIcon points at the
  ; installed exe itself (not IconFile - that .ico only ever sets
  ; Setup.exe's own icon above, it's never copied into $INSTDIR), so
  ; Windows shows whatever icon the app binary itself was built with.
  ; Written under {{.UninstallRegRoot}}: a current-user (non-elevated)
  ; install can't write HKLM at all, so this moves to HKCU for that scope
  ; (Windows' Programs and Features UI reads both hives) - see
  ; packproject.WindowsOptions.InstallScope's own doc comment.
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayName" "${APP_NAME}"
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "Publisher" "${APP_PUBLISHER}"
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "InstallLocation" "$INSTDIR"
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "NoModify" 1
  WriteRegDWORD {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "EstimatedSize" "$0"
{{range .FileAssociations}}
  ; File association for {{.Extension}} - always written to HKCR, not
  ; UninstallRegRoot/scope-dependent: Windows' own registry virtualization
  ; automatically redirects an unprivileged process's HKCR writes to
  ; HKCU\Software\Classes, so this needs no manual scope switching (see
  ; packproject.FileAssociation's own doc comment).
  WriteRegStr HKCR "{{.Extension}}" "" "{{.ProgID}}"
  WriteRegStr HKCR "{{.ProgID}}" "" "{{.Description}}"
  WriteRegStr HKCR "{{.ProgID}}\DefaultIcon" "" "$INSTDIR\${APP_EXE},0"
  WriteRegStr HKCR "{{.ProgID}}\shell\open\command" "" '"$INSTDIR\${APP_EXE}" "%1"'
{{end -}}
SectionEnd

{{if .DesktopShortcut -}}
; Its own Section (rather than an inline CreateShortcut in the main one
; above) specifically so it shows up as a real, individually toggleable
; checkbox on the Components page - selected by default (NSIS's own
; default for any Section not marked RO), the person installing can
; uncheck it. This is genuinely an end-user choice on winexe, unlike
; winmsi's own DesktopShortcut handling: wixl's bundled UI extension has
; no equivalent "choose components" dialog (only WixUI_Minimal, not the
; fuller WixUI_FeatureTree/Mondo variants that would need), so there it
; stays an author-time-only choice - see packproject.InstallExperience's
; own doc comment on this asymmetry.
Section "Desktop Shortcut" SEC_DESKTOP
  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd
{{end -}}

{{if .AutostartAtLogin -}}
; /o: unchecked by default, unlike Desktop Shortcut above - autostart is a
; bigger behavioral change for someone to discover after the fact than an
; extra shortcut is, so this asks explicitly rather than defaulting it on
; (see packproject.InstallExperience's own doc comment). Writes to HKCU,
; not HKLM: this only ever opts in the account running the installer, not
; every account on the machine - a plain per-user Run key is the standard,
; no-extra-tooling way to autostart on login and needs no elevation beyond
; what the installer already has.
Section /o "Run at Startup" SEC_AUTOSTART
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP_NAME}" '"$INSTDIR\${APP_EXE}"'
SectionEnd
{{end -}}

Section "Uninstall"
{{- if not .PerUser}}
  SetShellVarContext all
{{- end}}
  ExecWait 'taskkill /IM "${APP_EXE}" /F /T'
  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  RMDir "$SMPROGRAMS\${APP_NAME}"
{{- if .DesktopShortcut}}
  Delete "$DESKTOP\${APP_NAME}.lnk"
{{- end}}
{{- if .AutostartAtLogin}}
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP_NAME}"
{{- end}}
  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "$INSTDIR"
  DeleteRegKey HKCU "Software\${APP_NAME}"
  DeleteRegKey {{.UninstallRegRoot}} "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
{{range .FileAssociations -}}
  DeleteRegKey HKCR "{{.Extension}}"
  DeleteRegKey HKCR "{{.ProgID}}"
{{end -}}
SectionEnd
`))

// nsiData is the template's render input. Every path is a plain absolute
// filesystem path resolved against the project's BaseDir before rendering —
// NSIS's File/LicenseData directives read from wherever makensis itself
// runs, which may be a different host OS than the eventual installer target.
type nsiData struct {
	AppName, AppVersion, Publisher, ExeName string
	OutFile, InstallDir, IconFile           string
	BinarySource, LicenseSource             string
	Files                                   []nsiFileEntry
	// LaunchAfterInstall adds a checked-by-default "Run <App>" checkbox to
	// a real MUI2 finish page (MUI_FINISHPAGE_RUN) - an end-user choice at
	// install time, opted into here by the project author (see
	// packproject.InstallExperience's own doc comment on this split).
	LaunchAfterInstall bool
	// DesktopShortcut, when the project author opts in, adds a real,
	// checked-by-default Components-page checkbox letting the person
	// installing choose whether to also get a Desktop shortcut alongside
	// the Start Menu one this template always creates unconditionally.
	// Unlike LaunchAfterInstall, winmsi's own DesktopShortcut handling
	// stays author-time-only rather than a real end-user checkbox there
	// too - see packproject.InstallExperience's own doc comment on why.
	DesktopShortcut bool
	// AutostartAtLogin, when the project author opts in, adds a real,
	// *unchecked*-by-default Components-page checkbox (Section /o) letting
	// the person installing opt themselves into launching the app at every
	// login, via a per-user HKCU Run value. Unchecked by default, unlike
	// DesktopShortcut - see packproject.InstallExperience's own doc comment
	// on why. Same winexe-real-checkbox/winmsi-author-only split as
	// DesktopShortcut.
	AutostartAtLogin bool
	// PerUser mirrors packproject.WindowsOptions.InstallScope ==
	// InstallScopeCurrentUser - an author-time-only choice (see that
	// field's own doc comment on why neither backend can offer this as a
	// real end-user runtime pick). Drives RequestExecutionLevel,
	// SetShellVarContext (all-users needs it, current-user relies on
	// NSIS's own per-user default context), and UninstallRegRoot below.
	PerUser bool
	// UninstallRegRoot is "HKLM" for all-users, "HKCU" for current-user -
	// a non-elevated current-user install can't write HKLM at all, so its
	// Programs & Features registration has to live in HKCU instead
	// (Windows' own Programs and Features UI reads both hives).
	UninstallRegRoot string
	// FileAssociations mirrors packproject.Project.FileAssociations -
	// always unconditional (no author toggle, no end-user checkbox),
	// matching the unconditional Start Menu shortcut and Programs &
	// Features registration above: registering a file type is the whole
	// point of setting one, not an optional extra.
	FileAssociations []nsiFileAssociation
}

// nsiFileAssociation is one packproject.FileAssociation translated into
// NSIS/registry terms. ProgID is computed once in build.go (not stored in
// the schema itself - an implementation detail, not something a project
// author needs to see or name), Description already has build.go's own
// "<AppName> File" fallback applied so the template never needs to know
// about that default.
type nsiFileAssociation struct {
	Extension   string
	ProgID      string
	Description string
}

// nsiFileEntry is one payload entry translated into NSIS terms. DestDir is
// Windows-style (backslash-separated, relative to $INSTDIR, "" for the
// install root itself); Source is the absolute build-host path to read from
// — a directory (for File /r) when Recursive, otherwise a single file.
type nsiFileEntry struct {
	DestDir   string
	Source    string
	Recursive bool
}
