package innoimport

import "testing"

func TestParseTasksSection_RecognizesDesktopIconAndStartup(t *testing.T) {
	lines := []string{
		`Name: "desktopicon"; Description: "Create a &desktop icon"; GroupDescription: "Additional icons:"`,
		`Name: "startup"; Description: "Automatically start on login"; GroupDescription: "Additional icons:"; Flags: unchecked`,
	}
	desktop, autostart, skipped := parseTasksSection(lines, noExpand)
	if !desktop {
		t.Error("expected DesktopShortcut = true for a \"desktopicon\" task")
	}
	if !autostart {
		t.Error("expected AutostartAtLogin = true for a \"startup\" task")
	}
	if len(skipped) != 0 {
		t.Errorf("expected nothing skipped, got %v", skipped)
	}
}

func TestParseTasksSection_UnrecognizedTaskIsSkipped(t *testing.T) {
	lines := []string{`Name: "quicklaunchicon"; Description: "Create a &Quick Launch icon"`}
	desktop, autostart, skipped := parseTasksSection(lines, noExpand)
	if desktop || autostart {
		t.Errorf("expected no toggles recognized, got desktop=%v autostart=%v", desktop, autostart)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}

func TestParseIconsSection_RecognizesStartMenuAndDesktopShortcuts(t *testing.T) {
	lines := []string{
		`Name: "{autoprograms}\MyApp"; Filename: "{app}\MyApp.exe"`,
		`Name: "{autodesktop}\MyApp"; Filename: "{app}\MyApp.exe"; Tasks: desktopicon`,
	}
	skipped := parseIconsSection(lines, noExpand)
	if len(skipped) != 0 {
		t.Errorf("expected both recognized as already-covered, got skipped=%v", skipped)
	}
}

func TestParseIconsSection_UnrecognizedIconIsSkipped(t *testing.T) {
	lines := []string{`Name: "{userappdata}\MyApp\Shortcut"; Filename: "{app}\MyApp.exe"`}
	skipped := parseIconsSection(lines, noExpand)
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}

func TestParseRunSection_RecognizesPostinstallLaunchOfOwnExe(t *testing.T) {
	lines := []string{`Filename: "{app}\MyApp.exe"; Description: "Launch MyApp"; Flags: nowait postinstall skipifsilent`}
	launch, skipped := parseRunSection(lines, "MyApp.exe", noExpand)
	if !launch {
		t.Error("expected LaunchAfterInstall = true")
	}
	if len(skipped) != 0 {
		t.Errorf("expected nothing skipped, got %v", skipped)
	}
}

func TestParseRunSection_OtherRunEntryIsSkipped(t *testing.T) {
	lines := []string{`Filename: "{tmp}\vcredist_x64.exe"; Parameters: "/quiet"; Flags: waituntilterminated`}
	launch, skipped := parseRunSection(lines, "MyApp.exe", noExpand)
	if launch {
		t.Error("expected LaunchAfterInstall = false for an unrelated Run entry")
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}

func TestParseRunSection_NoExeNameNeverMatches(t *testing.T) {
	lines := []string{`Filename: "{app}\MyApp.exe"; Flags: nowait postinstall skipifsilent`}
	launch, skipped := parseRunSection(lines, "", noExpand)
	if launch {
		t.Error("expected LaunchAfterInstall = false when ExeName is unknown")
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}

func TestParseUninstallRunSection_RecognizesTaskkillOfOwnExe(t *testing.T) {
	lines := []string{`Filename: "{cmd}"; Parameters: "/C ""taskkill /im MyApp.exe /f /t"""; Flags: runhidden`}
	skipped := parseUninstallRunSection(lines, "MyApp.exe", noExpand)
	if len(skipped) != 0 {
		t.Errorf("expected the taskkill line to be recognized as already covered, got skipped=%v", skipped)
	}
}

func TestParseUninstallRunSection_OtherCommandIsSkipped(t *testing.T) {
	lines := []string{`Filename: "{app}\cleanup.exe"; Flags: runhidden`}
	skipped := parseUninstallRunSection(lines, "MyApp.exe", noExpand)
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}

func TestParseUninstallDeleteSection_CountsPathsInsideApp(t *testing.T) {
	lines := []string{
		`Type: filesandordirs; Name: "{app}\mesa-fallback"`,
		`Type: files; Name: "{app}\opengl32.dll"`,
	}
	insideApp, skipped := parseUninstallDeleteSection(lines, noExpand)
	if insideApp != 2 {
		t.Errorf("insideApp = %d, want 2", insideApp)
	}
	if len(skipped) != 0 {
		t.Errorf("expected nothing skipped, got %v", skipped)
	}
}

func TestParseUninstallDeleteSection_OutsideAppIsSkipped(t *testing.T) {
	lines := []string{`Type: filesandordirs; Name: "{userappdata}\MyApp\cache"`}
	insideApp, skipped := parseUninstallDeleteSection(lines, noExpand)
	if insideApp != 0 {
		t.Errorf("insideApp = %d, want 0", insideApp)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped line, got %v", skipped)
	}
}
