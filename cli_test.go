package main

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

// writeSourceFixture lays out a minimal but real KrankyBear-template
// source directory (build-config.sh + a matching Inno/*.iss) for cmdImport
// to import from.
func writeSourceFixture(t *testing.T, dir string) {
	t.Helper()
	buildConfig := `export KB_PROJECT_TITLE="Test App"
export KB_VERSION_DEFAULT="1.0.0"
export KB_HOMEPAGE="https://example.com/testapp"
export KB_INNO_ISS="Inno/app.iss"
`
	if err := os.WriteFile(filepath.Join(dir, "build-config.sh"), []byte(buildConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Inno"), 0o755); err != nil {
		t.Fatal(err)
	}
	iss := `[Setup]
AppId={{11111111-2222-3333-4444-555555555555}
AppName=Test App
`
	if err := os.WriteFile(filepath.Join(dir, "Inno", "app.iss"), []byte(iss), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCmdImport_CreatesNewProjectFile(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	projectPath := filepath.Join(dir, "installerbear.yaml")

	code := cmdImport([]string{"-source", dir, "-p", projectPath})
	if code != 0 {
		t.Fatalf("cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(projectPath)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	if proj.Identity.Name != "Test App" {
		t.Errorf("Identity.Name = %q, want Test App", proj.Identity.Name)
	}
	if proj.Identity.Version != "1.0.0" {
		t.Errorf("Identity.Version = %q, want 1.0.0", proj.Identity.Version)
	}
	if proj.Windows.UpgradeGUID != "{11111111-2222-3333-4444-555555555555}" {
		t.Errorf("Windows.UpgradeGUID = %q, want the unescaped AppId", proj.Windows.UpgradeGUID)
	}
}

// TestCmdImport_RelativeSourceDirResolvesCorrectly is a regression test: a
// relative -source (the natural thing to type, e.g. "-source .") used to
// silently produce zero imported Payload/LicenseFile/Icon paths — every one
// got misreported as "outside the project" — because joining a relative
// searchDir with an Inno-relative path produced a relative "absolute" path,
// and comparing that against BaseDir (always absolute) makes filepath.Rel
// fail outright rather than just returning an unexpected answer.
func TestCmdImport_RelativeSourceDirResolvesCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	iss, err := os.ReadFile(filepath.Join(dir, "Inno", "app.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss = append(iss, []byte("LicenseFile=..\\LICENSE\n")...)
	if err := os.WriteFile(filepath.Join(dir, "Inno", "app.iss"), iss, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT"), 0o644); err != nil {
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

	// The relative form is the point of this test: -source "." rather than
	// an absolute path.
	if code := cmdImport([]string{"-source", ".", "-p", "installerbear.yaml"}); code != 0 {
		t.Fatalf("cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(filepath.Join(dir, "installerbear.yaml"))
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	if proj.Identity.LicenseFile != "LICENSE" {
		t.Errorf("Identity.LicenseFile = %q, want LICENSE (relative -source must still resolve correctly)", proj.Identity.LicenseFile)
	}
}

func TestCmdImport_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	projectPath := filepath.Join(dir, "installerbear.yaml")

	code := cmdImport([]string{"-source", dir, "-p", projectPath, "-dry-run"})
	if code != 0 {
		t.Fatalf("cmdImport exit code = %d, want 0", code)
	}
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		t.Errorf("expected -dry-run to leave no project file behind, stat err = %v", err)
	}
}

// TestCmdImport_DefaultNeverOverwritesExistingValue confirms the CLI's
// unattended default (no human to click a checkbox) is the conservative
// one: fill blanks only, unless -overwrite is passed.
func TestCmdImport_DefaultNeverOverwritesExistingValue(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	projectPath := filepath.Join(dir, "installerbear.yaml")

	existing := &packproject.Project{
		Identity: packproject.Identity{Name: "Kept Name", ID: "com.example.testapp", Version: "9.9.9"},
	}
	if err := packproject.Save(existing, projectPath); err != nil {
		t.Fatal(err)
	}

	if code := cmdImport([]string{"-source", dir, "-p", projectPath}); code != 0 {
		t.Fatalf("cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(projectPath)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	if proj.Identity.Name != "Kept Name" {
		t.Errorf("Identity.Name = %q, want unchanged %q (no -overwrite)", proj.Identity.Name, "Kept Name")
	}
	if proj.Identity.Version != "9.9.9" {
		t.Errorf("Identity.Version = %q, want unchanged %q (no -overwrite)", proj.Identity.Version, "9.9.9")
	}
	// The GUID wasn't set before, so it should still be filled in.
	if proj.Windows.UpgradeGUID != "{11111111-2222-3333-4444-555555555555}" {
		t.Errorf("Windows.UpgradeGUID = %q, want it filled in (was blank)", proj.Windows.UpgradeGUID)
	}
}

func TestCmdImport_OverwriteReplacesExistingValue(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	projectPath := filepath.Join(dir, "installerbear.yaml")

	existing := &packproject.Project{
		Identity: packproject.Identity{Name: "Old Name", ID: "com.example.testapp", Version: "9.9.9"},
	}
	if err := packproject.Save(existing, projectPath); err != nil {
		t.Fatal(err)
	}

	if code := cmdImport([]string{"-source", dir, "-p", projectPath, "-overwrite"}); code != 0 {
		t.Fatalf("cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(projectPath)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	if proj.Identity.Name != "Test App" {
		t.Errorf("Identity.Name = %q, want overwritten to %q", proj.Identity.Name, "Test App")
	}
	if proj.Identity.Version != "1.0.0" {
		t.Errorf("Identity.Version = %q, want overwritten to 1.0.0", proj.Identity.Version)
	}
}

func TestCmdImport_RequiresSourceAndProject(t *testing.T) {
	if code := cmdImport(nil); code != 2 {
		t.Errorf("cmdImport with no flags = %d, want 2", code)
	}
	if code := cmdImport([]string{"-source", "."}); code != 2 {
		t.Errorf("cmdImport with no -project = %d, want 2", code)
	}
}

// TestCmdImport_RerunIsIdempotentForPayload confirms running import twice
// against the same project doesn't duplicate Payload entries — the CLI has
// no review step where a human would notice and skip a repeat, so this has
// to hold on its own for a script that re-runs import unattended.
func TestCmdImport_RerunIsIdempotentForPayload(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ReleaseNotes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	iss, err := os.ReadFile(filepath.Join(dir, "Inno", "app.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss = append(iss, []byte("[Files]\nSource: \"..\\ReleaseNotes.txt\"; DestDir: \"{app}\"\n")...)
	if err := os.WriteFile(filepath.Join(dir, "Inno", "app.iss"), iss, 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(dir, "installerbear.yaml")
	if code := cmdImport([]string{"-source", dir, "-p", projectPath}); code != 0 {
		t.Fatalf("first cmdImport exit code = %d, want 0", code)
	}
	if code := cmdImport([]string{"-source", dir, "-p", projectPath}); code != 0 {
		t.Fatalf("second cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(projectPath)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	count := 0
	for _, p := range proj.Payload {
		if p.Source == "ReleaseNotes.txt" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ReleaseNotes.txt appears %d times in Payload after two imports, want 1", count)
	}
}

// TestCmdImport_FileAssociationImportsAndIsIdempotentOnRerun confirms the
// whole path end to end via the real cmdImport CLI entry point (not just
// buildImportFields directly, which importconfig_test.go already covers):
// a [Registry] file-association block in the source .iss is imported on
// the first run, and running import again doesn't duplicate it - the
// same re-run safety TestCmdImport_RerunIsIdempotentForPayload already
// established for Payload, now covered for FileAssociations too.
func TestCmdImport_FileAssociationImportsAndIsIdempotentOnRerun(t *testing.T) {
	dir := t.TempDir()
	writeSourceFixture(t, dir)
	iss, err := os.ReadFile(filepath.Join(dir, "Inno", "app.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss = append(iss, []byte(`[Registry]
Root: HKCR; Subkey: ".myp"; ValueType: string; ValueName: ""; ValueData: "TestAppMyp"; Flags: uninsdeletevalue
Root: HKCR; Subkey: "TestAppMyp"; ValueType: string; ValueName: ""; ValueData: "Test App Project"; Flags: uninsdeletekey
Root: HKCR; Subkey: "TestAppMyp\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\TestApp.exe"" ""%1"""
`)...)
	if err := os.WriteFile(filepath.Join(dir, "Inno", "app.iss"), iss, 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(dir, "installerbear.yaml")
	if code := cmdImport([]string{"-source", dir, "-p", projectPath}); code != 0 {
		t.Fatalf("first cmdImport exit code = %d, want 0", code)
	}
	if code := cmdImport([]string{"-source", dir, "-p", projectPath}); code != 0 {
		t.Fatalf("second cmdImport exit code = %d, want 0", code)
	}

	proj, err := packproject.LoadLenient(projectPath)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	count := 0
	for _, a := range proj.FileAssociations {
		if a.Extension == ".myp" {
			count++
			if a.Description != "Test App Project" {
				t.Errorf("Description = %q, want %q", a.Description, "Test App Project")
			}
		}
	}
	if count != 1 {
		t.Errorf(".myp appears %d times in FileAssociations after two imports, want 1", count)
	}
}
