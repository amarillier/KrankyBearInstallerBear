package winexe

import (
	"text/template"
)

// nsiTemplate renders an NSIS script covering the "basics": app metadata, an
// optional license page, copying the binary plus payload files/trees under
// $INSTDIR, a registry install-dir marker (for InstallDirRegKey), and an
// uninstaller that kills the running exe before deleting everything. No file
// associations, no custom wizard pages, no signing — matches this tool's
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
RequestExecutionLevel admin
{{- if .IconFile}}
Icon "{{.IconFile}}"
UninstallIcon "{{.IconFile}}"
{{- end}}

{{if .LicenseSource -}}
!insertmacro MUI_PAGE_LICENSE "{{.LicenseSource}}"
{{end -}}
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "MainSection" SEC01
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
  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

Section "Uninstall"
  ExecWait 'taskkill /IM "${APP_EXE}" /F /T'
  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "$INSTDIR"
  DeleteRegKey HKCU "Software\${APP_NAME}"
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
