package innoimport

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"installerbear/internal/projectscan"
)

// findCandidate returns the first Payload candidate whose Source matches,
// or nil.
func findCandidate(t *testing.T, res ISSResult, source string) *projectscan.PayloadCandidate {
	t.Helper()
	for i := range res.Payload {
		if res.Payload[i].Source == source {
			return &res.Payload[i]
		}
	}
	return nil
}

// TestParseISS_RealFixture parses this repo's own real, committed .iss
// (Inno/KrankyBearInstallerBear.iss) — deliberately not a synthetic
// fixture, since it already exercises every real-world quirk this parser
// needs to handle: a doubled-brace AppId, {#Macro} substitution, an
// unresolvable Pascal-Script #define (MyAppAssocKey), a recursive
// [Files] wildcard with Excludes, several unsupported sections, and a
// SetupIconFile that also appears as its own [Files] line (which must be
// recognized as already-covered, not proposed twice).
func TestParseISS_RealFixture(t *testing.T) {
	baseDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	issPath := filepath.Join(baseDir, "Inno", "KrankyBearInstallerBear.iss")

	res, err := ParseISS(issPath, baseDir)
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}

	if res.Name != "KrankyBearInstallerBear" {
		t.Errorf("Name = %q", res.Name)
	}
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(res.Version) {
		t.Errorf("Version = %q, want an X.Y.Z version", res.Version)
	}
	if res.Publisher == "" {
		t.Error("Publisher should not be empty")
	}
	if res.URL != "https://github.com/amarillier/KrankyBearInstallerBear" {
		t.Errorf("URL = %q", res.URL)
	}
	if res.ExeName != "KrankyBearInstallerBear.exe" {
		t.Errorf("ExeName = %q", res.ExeName)
	}
	if !regexp.MustCompile(`^\{[0-9A-Fa-f-]{36}\}$`).MatchString(res.UpgradeGUID) {
		t.Errorf("UpgradeGUID = %q, want a single-braced GUID (doubled-brace escaping stripped)", res.UpgradeGUID)
	}
	if res.LicenseFile != "LICENSE" {
		t.Errorf("LicenseFile = %q, want LICENSE", res.LicenseFile)
	}
	if res.IconICO != "assets/images/KrankyBearInstallerBear.ico" {
		t.Errorf("IconICO = %q", res.IconICO)
	}
	if res.WindowsBinary != "bin/KrankyBearInstallerBear.exe" {
		t.Errorf("WindowsBinary = %q, want bin/KrankyBearInstallerBear.exe", res.WindowsBinary)
	}

	// The icon is already modeled via IconICO — its [Files] line must not
	// also turn into a Payload candidate.
	if c := findCandidate(t, res, "assets/images/KrankyBearInstallerBear.ico"); c != nil {
		t.Errorf("icon should not also be proposed as Payload: %+v", c)
	}

	assets := findCandidate(t, res, "assets")
	if assets == nil {
		t.Fatal("expected a recursive Payload candidate for assets/*")
	}
	if !assets.Recursive || assets.Dest != "assets" {
		t.Errorf("assets candidate = %+v, want Recursive=true Dest=%q", assets, "assets")
	}
	if len(assets.Excludes) != 1 || assets.Excludes[0] != "mesa-win/*" {
		t.Errorf("assets Excludes = %v, want [mesa-win/*]", assets.Excludes)
	}
	if len(assets.OS) != 0 {
		t.Errorf("assets candidate OS = %v, want no restriction (package.sh bundles it into every platform too)", assets.OS)
	}

	notes := findCandidate(t, res, "ReleaseNotes.txt")
	if notes == nil || notes.Recursive || notes.Dest != "" {
		t.Errorf("ReleaseNotes.txt candidate = %+v, want a non-recursive root-dest entry", notes)
	}
	if notes != nil && len(notes.OS) != 0 {
		t.Errorf("ReleaseNotes.txt candidate OS = %v, want no restriction — it's not Windows-specific content", notes.OS)
	}

	for _, mesaFile := range []string{
		"assets/mesa-win/opengl32.dll",
		"assets/mesa-win/libgallium_wgl.dll",
		"assets/mesa-win/.force-mesa-fallback.sample",
	} {
		c := findCandidate(t, res, mesaFile)
		if c == nil {
			t.Errorf("expected a Payload candidate for %s", mesaFile)
			continue
		}
		if c.Dest != "mesa-fallback" {
			t.Errorf("%s Dest = %q, want mesa-fallback", mesaFile, c.Dest)
		}
		if len(c.OS) != 1 || c.OS[0] != "windows" {
			t.Errorf("%s OS = %v, want [windows] (the Mesa3D fallback is genuinely Windows-only)", mesaFile, c.OS)
		}
	}

	if len(res.Skipped) == 0 {
		t.Error("expected Skipped to report the unsupported [Registry]/[Icons]/[Tasks]/[Run]/... sections")
	}
}

// writeISS is a small test helper for the synthetic-fixture cases below,
// where the exact minimal .iss content matters more than reusing the real
// project's own file.
func writeISS(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "app.iss")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestParseISS_RelativeIssPathStillResolves is a regression test: ParseISS
// used to derive issDir via a bare filepath.Dir(issPath), so a relative
// issPath (the CLI's own "-source ." is the natural thing to type) meant
// every single [Files]/LicenseFile/SetupIconFile path got misreported as
// "outside the project" — filepath.Rel refuses to compare a relative
// "resolved" path against baseDir, which is always absolute.
func TestParseISS_RelativeIssPathStillResolves(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Inno"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Inno", "app.iss"), []byte("[Setup]\nLicenseFile=..\\LICENSE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Both issPath and baseDir are relative here, deliberately.
	res, err := ParseISS(filepath.Join("Inno", "app.iss"), ".")
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}
	if res.LicenseFile != "LICENSE" {
		t.Errorf("LicenseFile = %q, want LICENSE (relative issPath/baseDir must still resolve)", res.LicenseFile)
	}
}

func TestParseISS_UnsupportedDestDirTokenIsSkipped(t *testing.T) {
	dir := t.TempDir()
	issPath := writeISS(t, dir, `[Setup]
AppName=TestApp
[Files]
Source: "..\extra.txt"; DestDir: "{autopf}\Shared"; Flags: ignoreversion
`)
	res, err := ParseISS(issPath, dir)
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}
	if len(res.Payload) != 0 {
		t.Errorf("expected no Payload candidates for an unsupported DestDir token, got %+v", res.Payload)
	}
	if len(res.Skipped) == 0 {
		t.Error("expected the {autopf} DestDir to be reported in Skipped")
	}
}

func TestParseISS_ExtensionWildcardIsSkipped(t *testing.T) {
	dir := t.TempDir()
	issPath := writeISS(t, dir, `[Setup]
AppName=TestApp
[Files]
Source: "..\assets\*.dll"; DestDir: "{app}"; Flags: ignoreversion
`)
	res, err := ParseISS(issPath, dir)
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}
	if len(res.Payload) != 0 {
		t.Errorf("expected no Payload candidates for an extension-filtered wildcard, got %+v", res.Payload)
	}
	if len(res.Skipped) == 0 {
		t.Error("expected the *.dll wildcard to be reported in Skipped")
	}
}

func TestParseISS_UnresolvedMacroDefineDoesNotCrash(t *testing.T) {
	dir := t.TempDir()
	issPath := writeISS(t, dir, `#define MyAppName "TestApp"
#define MyAppAssocKey StringChange(MyAppName, " ", "") + ".exe"
[Setup]
AppName={#MyAppName}
AppId={#MyAppAssocKey}
`)
	res, err := ParseISS(issPath, dir)
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}
	if res.Name != "TestApp" {
		t.Errorf("Name = %q, want TestApp", res.Name)
	}
	// MyAppAssocKey isn't a plain quoted #define, so it's left unresolved —
	// AppId should come through as the literal, un-substituted token rather
	// than something guessed at.
	if res.UpgradeGUID != "{#MyAppAssocKey}" {
		t.Errorf("UpgradeGUID = %q, want the unresolved macro token verbatim", res.UpgradeGUID)
	}
}
