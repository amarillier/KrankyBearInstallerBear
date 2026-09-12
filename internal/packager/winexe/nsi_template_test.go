package winexe

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files instead of comparing against them")

func sampleNSIData() nsiData {
	return nsiData{
		AppName:       "Test App",
		AppVersion:    "1.2.3",
		Publisher:     "Someone",
		ExeName:       "TestApp.exe",
		OutFile:       `C:\out\TestAppSetup_1.2.3.exe`,
		InstallDir:    `$PROGRAMFILES64\Test App`,
		IconFile:      `C:\src\icon.ico`,
		BinarySource:  `C:\src\bin\TestApp.exe`,
		LicenseSource: `C:\src\LICENSE`,
		Files: []nsiFileEntry{
			{DestDir: `assets\images`, Source: `C:\src\assets\images`, Recursive: true},
			{DestDir: "", Source: `C:\src\ReleaseNotes.txt`},
		},
	}
}

func TestNSITemplate_Golden(t *testing.T) {
	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, sampleNSIData()); err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "sample.nsi.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if buf.String() != string(want) {
		t.Errorf("rendered .nsi does not match %s (run with -update to see/accept the diff)\n--- got ---\n%s\n--- want ---\n%s", goldenPath, buf.String(), want)
	}
}

func TestNSITemplate_NoLicenseNoIconNoPayload(t *testing.T) {
	data := sampleNSIData()
	data.LicenseSource = ""
	data.IconFile = ""
	data.Files = nil

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	// The Icon/UninstallIcon directives always start their own line in the
	// template (unindented) - checked with a leading newline so this can't
	// false-match "DisplayIcon" (part of the unconditional Programs &
	// Features registration, always present, nothing to do with IconFile)
	// or a descriptive comment mentioning the word "icon" in passing.
	for _, unwanted := range []string{"MUI_PAGE_LICENSE", "MUI_ICON", "MUI_UNICON", "\nIcon \"", "\nUninstallIcon \"", "/oname=License.txt"} {
		if bytes.Contains([]byte(out), []byte(unwanted)) {
			t.Errorf("expected no %q in output when license/icon/payload are all empty:\n%s", unwanted, out)
		}
	}
	if !bytes.Contains([]byte(out), []byte(`File "C:\src\bin\TestApp.exe"`)) {
		t.Error("expected the main binary File directive to still be present")
	}
}

func TestNSITemplate_StartMenuShortcutAlwaysPresent(t *testing.T) {
	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, sampleNSIData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"`,
		`Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"`,
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output (Start Menu shortcut is unconditional):\n%s", want, out)
		}
	}
}

func TestNSITemplate_DesktopShortcutOptIn(t *testing.T) {
	data := sampleNSIData()
	data.DesktopShortcut = true

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	// Unlike LaunchAfterInstall, this is a real end-user checkbox on
	// winexe (unlike winmsi, which stays author-time-only - see
	// packproject.InstallExperience's own doc comment on the asymmetry):
	// a Components page, a separate optional Section the person
	// installing can toggle, and the main Section marked RO so it can't
	// be unchecked once a Components page exists to show it on.
	for _, want := range []string{
		"!insertmacro MUI_PAGE_COMPONENTS",
		`Section "${APP_NAME}" SEC01`,
		"SectionIn RO",
		`Section "Desktop Shortcut" SEC_DESKTOP`,
		`CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"`,
		`Delete "$DESKTOP\${APP_NAME}.lnk"`,
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when DesktopShortcut is true:\n%s", want, out)
		}
	}

	data.DesktopShortcut = false
	buf.Reset()
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out = buf.String()
	if bytes.Contains([]byte(out), []byte(`$DESKTOP`)) {
		t.Errorf("expected no $DESKTOP reference when DesktopShortcut is false:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("MUI_PAGE_COMPONENTS")) {
		t.Errorf("expected no Components page when DesktopShortcut is false (nothing optional to show):\n%s", out)
	}
}

func TestNSITemplate_AutostartAtLoginOptIn(t *testing.T) {
	data := sampleNSIData()
	data.AutostartAtLogin = true

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	// Unchecked by default (Section /o), unlike Desktop Shortcut - a
	// bigger behavioral change to opt into by surprise, so it asks
	// explicitly rather than defaulting on. Writes/removes a per-user
	// HKCU Run value, not HKLM.
	for _, want := range []string{
		"!insertmacro MUI_PAGE_COMPONENTS",
		`Section /o "Run at Startup" SEC_AUTOSTART`,
		`WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP_NAME}" '"$INSTDIR\${APP_EXE}"'`,
		`DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP_NAME}"`,
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when AutostartAtLogin is true:\n%s", want, out)
		}
	}

	data.AutostartAtLogin = false
	buf.Reset()
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out = buf.String()
	if bytes.Contains([]byte(out), []byte("SEC_AUTOSTART")) {
		t.Errorf("expected no autostart section when AutostartAtLogin is false:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte(`CurrentVersion\Run`)) {
		t.Errorf("expected no Run key reference when AutostartAtLogin is false:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("MUI_PAGE_COMPONENTS")) {
		t.Errorf("expected no Components page when neither DesktopShortcut nor AutostartAtLogin is set:\n%s", out)
	}
}

// TestNSITemplate_ComponentsPageShownWhenEitherToggleIsSet confirms the
// Components page appears when just one of DesktopShortcut/AutostartAtLogin
// is set, not only when both are - a plain {{if .DesktopShortcut}} gate
// (forgetting the `or`) would silently hide the page from a project that
// only opts into autostart.
func TestNSITemplate_ComponentsPageShownWhenEitherToggleIsSet(t *testing.T) {
	data := sampleNSIData()
	data.AutostartAtLogin = true
	data.DesktopShortcut = false

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("MUI_PAGE_COMPONENTS")) {
		t.Errorf("expected Components page when AutostartAtLogin alone is set:\n%s", buf.String())
	}
}

func TestNSITemplate_LaunchAfterInstallOptIn(t *testing.T) {
	data := sampleNSIData()
	data.LaunchAfterInstall = true

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"`,
		"!define MUI_FINISHPAGE_RUN_CHECKED",
		"!insertmacro MUI_PAGE_FINISH",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when LaunchAfterInstall is true:\n%s", want, out)
		}
	}

	data.LaunchAfterInstall = false
	buf.Reset()
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("MUI_FINISHPAGE")) {
		t.Errorf("expected no MUI_FINISHPAGE_* directives when LaunchAfterInstall is false:\n%s", buf.String())
	}
}

// TestNSITemplate_HelpFlagHandlerAlwaysPresent covers Setup.exe's -help
// support: unlike LaunchAfterInstall/DesktopShortcut, this isn't gated
// behind any InstallExperience toggle - Allan's own stated motivation was
// specifically about installers that silently ignore -help ("I try -? or
// -help first... then curse the packager"), so every generated installer
// should answer it, not just ones that opt in.
func TestNSITemplate_HelpFlagHandlerAlwaysPresent(t *testing.T) {
	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, sampleNSIData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`!include "FileFunc.nsh"`,
		"Function .onInit",
		`${GetParameters} $R0`,
		`!insertmacro CheckHelpFlag "/?"`,
		`!insertmacro CheckHelpFlag "/help"`,
		`!insertmacro CheckHelpFlag "-?"`,
		`!insertmacro CheckHelpFlag "-h"`,
		`!insertmacro CheckHelpFlag "-help"`,
		`!insertmacro CheckHelpFlag "--help"`,
		"MessageBox MB_OK",
		"/D=<path>",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in the help-flag handler, got:\n%s", want, out)
		}
	}
}

// TestNSITemplate_HelpTextMentionsLicenseOnlyWhenOneExists confirms the
// help MessageBox's "silent installs skip the license page" line only
// appears when there's actually a license page to skip - a silent (/S)
// install with no license configured has no such page to mention.
func TestNSITemplate_HelpTextMentionsLicenseOnlyWhenOneExists(t *testing.T) {
	data := sampleNSIData()
	data.LicenseSource = `C:\src\LICENSE`

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("also skips the license page")) {
		t.Error("expected the help text to mention skipping the license page when one is configured")
	}

	data.LicenseSource = ""
	buf.Reset()
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("license page")) {
		t.Errorf("expected no license-page mention in the help text when no license is configured:\n%s", buf.String())
	}
}

// TestNSITemplate_RegistersWithProgramsAndFeatures is a regression test
// for a real gap found in live Windows testing: unlike a real .msi
// (always tracked by the Windows Installer service), NSIS never registers
// anything in Windows' "Apps & Features"/Programs and Features on its
// own - every properly-built NSIS installer has to write those registry
// keys by hand. Setup.exe previously didn't, so it silently never showed
// up there (nor in Get-Package) at all, regardless of InstallExperience
// settings - this is unconditional, not gated behind any toggle.
func TestNSITemplate_RegistersWithProgramsAndFeatures(t *testing.T) {
	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, sampleNSIData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}`
	for _, want := range []string{
		`WriteRegStr HKLM "` + uninstallKey + `" "DisplayName" "${APP_NAME}"`,
		`WriteRegStr HKLM "` + uninstallKey + `" "DisplayVersion" "${APP_VERSION}"`,
		`WriteRegStr HKLM "` + uninstallKey + `" "Publisher" "${APP_PUBLISHER}"`,
		`WriteRegStr HKLM "` + uninstallKey + `" "UninstallString" '"$INSTDIR\uninstall.exe"'`,
		`WriteRegStr HKLM "` + uninstallKey + `" "DisplayIcon" "$INSTDIR\${APP_EXE}"`,
		`WriteRegDWORD HKLM "` + uninstallKey + `" "NoModify" 1`,
		`WriteRegDWORD HKLM "` + uninstallKey + `" "NoRepair" 1`,
		`${GetSize} "$INSTDIR"`,
		`WriteRegDWORD HKLM "` + uninstallKey + `" "EstimatedSize" "$0"`,
		`DeleteRegKey HKLM "` + uninstallKey + `"`,
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output (Programs & Features registration is unconditional):\n%s", want, out)
		}
	}
}
