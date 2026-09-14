package main

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/innoimport"
	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

func fieldByLabel(t *testing.T, fields []importField, label string) *importField {
	t.Helper()
	for i := range fields {
		if fields[i].label == label {
			return &fields[i]
		}
	}
	return nil
}

func TestBuildImportFields_PrefersISSOverPackageConfig(t *testing.T) {
	proj := &packproject.Project{}
	pkg := innoimport.PkgConfigResult{Name: "From PackageConfig", Vendor: "Acme"}
	iss := innoimport.ISSResult{Name: "From ISS"}

	fields := buildImportFields(proj, pkg, iss)

	name := fieldByLabel(t, fields, "Name")
	if name == nil || name.proposed != "From ISS" {
		t.Errorf("Name field = %+v, want proposed %q (iss should win over pkg)", name, "From ISS")
	}
	vendor := fieldByLabel(t, fields, "Vendor")
	if vendor == nil || vendor.proposed != "Acme" {
		t.Errorf("Vendor field = %+v, want proposed %q (only pkg has a Vendor)", vendor, "Acme")
	}
}

func TestBuildImportFields_ProposesLicenseFromKBLicenseDefault(t *testing.T) {
	proj := &packproject.Project{}
	pkg := innoimport.PkgConfigResult{License: "GPL v3"}

	fields := buildImportFields(proj, pkg, innoimport.ISSResult{})
	f := fieldByLabel(t, fields, "License")
	if f == nil || f.proposed != "GPL v3" {
		t.Errorf("License field = %+v, want proposed %q", f, "GPL v3")
	}

	f.apply(proj)
	if proj.Identity.License != "GPL v3" {
		t.Errorf("Identity.License = %q, want GPL v3", proj.Identity.License)
	}
}

func TestBuildImportFields_SkipsFieldsThatMatchCurrent(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Same Name"}}
	iss := innoimport.ISSResult{Name: "Same Name", Version: "9.9.9"}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)

	if f := fieldByLabel(t, fields, "Name"); f != nil {
		t.Errorf("expected no Name field when proposed equals current, got %+v", f)
	}
	if f := fieldByLabel(t, fields, "Version"); f == nil {
		t.Error("expected a Version field to be proposed")
	}
}

func TestBuildImportFields_WindowsBinaryUpsertsOnApply(t *testing.T) {
	proj := &packproject.Project{}
	iss := innoimport.ISSResult{WindowsBinary: "bin/TestApp.exe"}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)
	f := fieldByLabel(t, fields, "Windows binary (amd64)")
	if f == nil {
		t.Fatal("expected a Windows binary field")
	}

	f.apply(proj)

	bin, ok := proj.BinaryFor("windows", "amd64")
	if !ok || bin.Path != "bin/TestApp.exe" {
		t.Errorf("BinaryFor(windows, amd64) = %+v, ok=%v, want bin/TestApp.exe", bin, ok)
	}
}

func TestBuildImportFields_ProposesIDFromBundleID(t *testing.T) {
	proj := &packproject.Project{}
	pkg := innoimport.PkgConfigResult{BundleID: "com.github.owner.testapp"}

	fields := buildImportFields(proj, pkg, innoimport.ISSResult{})
	f := fieldByLabel(t, fields, "ID")
	if f == nil || f.proposed != "com.github.owner.testapp" {
		t.Errorf("ID field = %+v, want proposed %q", f, "com.github.owner.testapp")
	}
}

func TestUpsertBinary_ReplacesExistingSlotOnly(t *testing.T) {
	proj := &packproject.Project{
		Binaries: []packproject.BinaryEntry{
			{OS: "windows", Arch: "amd64", Path: "old.exe"},
			{OS: "darwin", Arch: "arm64", Path: "app-macos"},
		},
	}
	upsertBinary(proj, "windows", "amd64", "new.exe")

	if len(proj.Binaries) != 2 {
		t.Fatalf("Binaries = %+v, want still 2 entries", proj.Binaries)
	}
	winBin, _ := proj.BinaryFor("windows", "amd64")
	if winBin.Path != "new.exe" {
		t.Errorf("windows/amd64 = %q, want new.exe", winBin.Path)
	}
	macBin, _ := proj.BinaryFor("darwin", "arm64")
	if macBin.Path != "app-macos" {
		t.Errorf("darwin/arm64 should be untouched, got %q", macBin.Path)
	}
}

// TestParseProjectISS_PathsRelativeToBaseDirNotSearchDir confirms searchDir
// (where the .iss is found) and baseDir (what its paths are expressed
// relative to) are independent — the CLI's -source and -project can
// legitimately point at different directories, and the resulting Payload
// Source must resolve correctly against baseDir wherever it ends up saved,
// not against wherever it happened to be imported from.
func TestParseProjectISS_PathsRelativeToBaseDirNotSearchDir(t *testing.T) {
	// baseDir is searchDir's own parent, so the resulting Source is
	// relative-but-not-identical to what it'd be if searchDir were used for
	// both (a real path, not just "outside the project" rejected outright).
	baseDir := t.TempDir()
	searchDir := filepath.Join(baseDir, "old-project")

	if err := os.MkdirAll(filepath.Join(searchDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(searchDir, "Inno"), 0o755); err != nil {
		t.Fatal(err)
	}
	issContent := "[Setup]\nAppName=TestApp\n[Files]\n" +
		`Source: "..\assets\ReleaseNotes.txt"; DestDir: "{app}"` + "\n"
	if err := os.WriteFile(filepath.Join(searchDir, "Inno", "app.iss"), []byte(issContent), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := parseProjectISS(searchDir, baseDir, innoimport.PkgConfigResult{})
	if err != nil {
		t.Fatalf("parseProjectISS: %v", err)
	}

	var found bool
	for _, c := range res.Payload {
		if c.Source == filepath.ToSlash(mustRel(t, baseDir, filepath.Join(searchDir, "assets", "ReleaseNotes.txt"))) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a Payload candidate with Source relative to baseDir, got %+v", res.Payload)
	}
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestNewPayloadCandidates_SkipsAlreadyPresentSources(t *testing.T) {
	existing := []packproject.PayloadEntry{{Source: "assets/images", Dest: "assets/images"}}
	candidates := []projectscan.PayloadCandidate{
		{Source: "assets/images", Dest: "assets/images"}, // already present -> skipped
		{Source: "ReleaseNotes.txt", Dest: ""},           // new -> kept
	}

	got := newPayloadCandidates(existing, candidates)
	if len(got) != 1 || got[0].Source != "ReleaseNotes.txt" {
		t.Errorf("newPayloadCandidates() = %+v, want only the ReleaseNotes.txt candidate", got)
	}
}

func TestNewFileAssociationCandidates_SkipsAlreadyRegisteredExtensions(t *testing.T) {
	existing := []packproject.FileAssociation{{Extension: ".myp"}}
	candidates := []packproject.FileAssociation{
		{Extension: ".MYP", Description: "already present, different case -> skipped"},
		{Extension: ".prj", Description: "new -> kept"},
	}

	got := newFileAssociationCandidates(existing, candidates)
	if len(got) != 1 || got[0].Extension != ".prj" {
		t.Errorf("newFileAssociationCandidates() = %+v, want only the .prj candidate", got)
	}
}

// TestBuildImportFields_ProposesFileAssociation confirms a FileAssociation
// found in the .iss flows all the way through buildImportFields as a
// real importField - both the GUI review dialog and the CLI's cmdImport
// apply *any* importField generically, so this one test covers both call
// sites without needing separate GUI/CLI-specific wiring or tests.
func TestBuildImportFields_ProposesFileAssociation(t *testing.T) {
	proj := &packproject.Project{}
	iss := innoimport.ISSResult{
		FileAssociations: []packproject.FileAssociation{
			{Extension: ".myp", Description: "My App Project"},
		},
	}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)

	f := fieldByLabel(t, fields, "File association")
	if f == nil {
		t.Fatal("expected a \"File association\" field")
	}
	if f.current != "" {
		t.Errorf("current = %q, want \"\" (a new list item, not an overwrite) so both the GUI defaults it checked and the CLI treats it as fill-not-overwrite", f.current)
	}
	if f.proposed != ".myp (My App Project)" {
		t.Errorf("proposed = %q, want %q", f.proposed, ".myp (My App Project)")
	}

	f.apply(proj)
	if len(proj.FileAssociations) != 1 || proj.FileAssociations[0].Extension != ".myp" {
		t.Errorf("apply() did not append the association, got %+v", proj.FileAssociations)
	}
}

func TestBuildImportFields_AlreadyRegisteredFileAssociationIsNotProposedAgain(t *testing.T) {
	proj := &packproject.Project{FileAssociations: []packproject.FileAssociation{{Extension: ".myp"}}}
	iss := innoimport.ISSResult{FileAssociations: []packproject.FileAssociation{{Extension: ".myp"}}}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)

	if f := fieldByLabel(t, fields, "File association"); f != nil {
		t.Errorf("expected no \"File association\" field when the extension is already registered, got %+v", f)
	}
}

// TestBuildImportFields_ProposesInstallExperienceToggles confirms the
// three [Tasks]/[Run]-derived InstallExperience toggles (see
// innoimport/installexperience.go) flow through buildImportFields as real
// importFields too, the same generic GUI/CLI-shared mechanism the
// FileAssociation fields above already rely on.
func TestBuildImportFields_ProposesInstallExperienceToggles(t *testing.T) {
	proj := &packproject.Project{}
	iss := innoimport.ISSResult{
		DesktopShortcut:    true,
		AutostartAtLogin:   true,
		LaunchAfterInstall: true,
	}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)

	for _, label := range []string{
		"Desktop shortcut (Install Experience)",
		"Run at startup (Install Experience)",
		"Launch after install (Install Experience)",
	} {
		f := fieldByLabel(t, fields, label)
		if f == nil {
			t.Fatalf("expected a %q field", label)
		}
		if f.current != "" {
			t.Errorf("%s: current = %q, want \"\" (so the CLI's default fill-blanks-only policy applies it without needing -overwrite)", label, f.current)
		}
		if f.proposed != "enable" {
			t.Errorf("%s: proposed = %q, want %q", label, f.proposed, "enable")
		}
	}

	for _, f := range fields {
		f.apply(proj)
	}
	if !proj.InstallExperience.DesktopShortcut || !proj.InstallExperience.AutostartAtLogin || !proj.InstallExperience.LaunchAfterInstall {
		t.Errorf("apply() did not set all three toggles, got %+v", proj.InstallExperience)
	}
}

// TestBuildImportFields_AlreadyEnabledInstallExperienceIsNotProposedAgain
// mirrors the FileAssociation "already registered" test above but for the
// boolean toggles - importing shouldn't nag about a toggle the project
// already has enabled.
func TestBuildImportFields_AlreadyEnabledInstallExperienceIsNotProposedAgain(t *testing.T) {
	proj := &packproject.Project{InstallExperience: packproject.InstallExperience{DesktopShortcut: true}}
	iss := innoimport.ISSResult{DesktopShortcut: true}

	fields := buildImportFields(proj, innoimport.PkgConfigResult{}, iss)

	if f := fieldByLabel(t, fields, "Desktop shortcut (Install Experience)"); f != nil {
		t.Errorf("expected no field when DesktopShortcut is already enabled, got %+v", f)
	}
}

func TestParseProjectISS_UsesKBInnoISSPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Custom"), 0o755); err != nil {
		t.Fatal(err)
	}
	issContent := "[Setup]\nAppName=TestApp\n"
	if err := os.WriteFile(filepath.Join(dir, "Custom", "app.iss"), []byte(issContent), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := parseProjectISS(dir, dir, innoimport.PkgConfigResult{ISSPath: "Custom/app.iss"})
	if err != nil {
		t.Fatalf("parseProjectISS: %v", err)
	}
	if res.Name != "TestApp" {
		t.Errorf("Name = %q, want TestApp (should have followed KB_INNO_ISS's path)", res.Name)
	}
}

func TestParseProjectISS_FallsBackToInnoGlob(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Inno"), 0o755); err != nil {
		t.Fatal(err)
	}
	issContent := "[Setup]\nAppName=GlobFound\n"
	if err := os.WriteFile(filepath.Join(dir, "Inno", "SomeApp.iss"), []byte(issContent), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := parseProjectISS(dir, dir, innoimport.PkgConfigResult{}) // no ISSPath set
	if err != nil {
		t.Fatalf("parseProjectISS: %v", err)
	}
	if res.Name != "GlobFound" {
		t.Errorf("Name = %q, want GlobFound (should have found Inno/*.iss)", res.Name)
	}
}

// TestImportPipeline_RealRepoFixture runs the exact pipeline
// importExistingConfig drives (ParseTemplatePackageConfig -> parseProjectISS
// -> buildImportFields) against this repo's own real root — the closest
// automated substitute for actually clicking "Import Existing Config..." in
// the running app and confirming the real AppId GUID and Payload dest
// remapping (mesa-win -> mesa-fallback) show up correctly end to end.
func TestImportPipeline_RealRepoFixture(t *testing.T) {
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	pkgRes, err := innoimport.ParseTemplatePackageConfig(dir)
	if err != nil {
		t.Fatalf("ParseTemplatePackageConfig: %v", err)
	}
	issRes, err := parseProjectISS(dir, dir, pkgRes)
	if err != nil {
		t.Fatalf("parseProjectISS: %v", err)
	}

	proj := &packproject.Project{} // a brand new, empty project
	fields := buildImportFields(proj, pkgRes, issRes)

	guid := fieldByLabel(t, fields, "Windows Upgrade GUID")
	if guid == nil {
		t.Fatal("expected a Windows Upgrade GUID field")
	}
	guid.apply(proj)
	if proj.Windows.UpgradeGUID == "" || proj.Windows.UpgradeGUID[0] != '{' {
		t.Errorf("UpgradeGUID = %q, want a real single-braced GUID", proj.Windows.UpgradeGUID)
	}

	var foundMesaDLL bool
	for _, c := range issRes.Payload {
		if c.Source == "assets/mesa-win/opengl32.dll" && c.Dest == "mesa-fallback" {
			foundMesaDLL = true
		}
	}
	if !foundMesaDLL {
		t.Errorf("expected a mesa-win opengl32.dll candidate remapped to mesa-fallback, got %+v", issRes.Payload)
	}

	bin := fieldByLabel(t, fields, "Windows binary (amd64)")
	if bin == nil {
		t.Fatal("expected a Windows binary field")
	}
	bin.apply(proj)
	if got, ok := proj.BinaryFor("windows", "amd64"); !ok || got.Path != "bin/KrankyBearInstallerBear.exe" {
		t.Errorf("windows/amd64 binary = %+v, ok=%v, want bin/KrankyBearInstallerBear.exe", got, ok)
	}

	id := fieldByLabel(t, fields, "ID")
	if id == nil {
		t.Fatal("expected an ID field from Info-plist.txt's CFBundleIdentifier")
	}
	id.apply(proj)
	if proj.Identity.ID != "com.github.amarillier.KrankyBearInstallerBear" {
		t.Errorf("Identity.ID = %q, want com.github.amarillier.KrankyBearInstallerBear", proj.Identity.ID)
	}

	var mesaOS []string
	for _, c := range issRes.Payload {
		if c.Source == "assets/mesa-win/opengl32.dll" {
			mesaOS = c.OS
		}
	}
	if len(mesaOS) != 1 || mesaOS[0] != "windows" {
		t.Errorf("mesa DLL OS = %v, want [windows]", mesaOS)
	}
	var notesOS []string
	var sawNotes bool
	for _, c := range issRes.Payload {
		if c.Source == "ReleaseNotes.md" {
			notesOS = c.OS
			sawNotes = true
		}
	}
	if !sawNotes {
		t.Fatal("expected a ReleaseNotes.md candidate")
	}
	if len(notesOS) != 0 {
		t.Errorf("ReleaseNotes.md OS = %v, want no restriction", notesOS)
	}
}

func TestParseProjectISS_NoISSAtAll(t *testing.T) {
	dir := t.TempDir()
	res, err := parseProjectISS(dir, dir, innoimport.PkgConfigResult{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Name != "" {
		t.Errorf("expected a zero result, got %+v", res)
	}
}
