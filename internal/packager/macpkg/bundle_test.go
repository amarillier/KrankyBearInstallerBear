package macpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"installerbear/internal/packproject"
)

func sampleProject(t *testing.T, dir string) *packproject.Project {
	t.Helper()

	binPath := filepath.Join(dir, "testapp-macos-arm64")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(licensePath, []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(dir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "icon.png"), []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &packproject.Project{
		BaseDir: dir,
		Identity: packproject.Identity{
			Name:        "TestApp",
			ID:          "com.example.testapp",
			Version:     "1.2.3",
			LicenseFile: "LICENSE",
		},
		Binaries: []packproject.BinaryEntry{{OS: "darwin", Arch: "arm64", Path: binPath}},
		Payload: []packproject.PayloadEntry{
			{Source: "assets/images", Dest: "assets/images", Recursive: true},
		},
	}
	p.Defaults()
	return p
}

func TestBuildAppBundle_Layout(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	destDir := filepath.Join(dir, "out")

	appPath, err := BuildAppBundle(proj, "arm64", destDir)
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	if appPath != filepath.Join(destDir, "TestApp.app") {
		t.Fatalf("appPath = %q, want TestApp.app under destDir", appPath)
	}

	mustExist := []string{
		"Contents/MacOS/TestApp",
		"Contents/MacOS/License.txt",
		"Contents/MacOS/assets/images/icon.png",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(appPath, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
	// No Info.plist in the project dir (sampleProject doesn't create one) -
	// see TestBuildAppBundle_NoInfoPlistByDefault and
	// TestBuildAppBundle_CopiesRealInfoPlist for the actual behavior this
	// asserts.
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		t.Error("expected no Contents/Info.plist when the project dir has none")
	}

	// "TestApp" and its slug "testapp" are case-insensitively identical, so
	// on macOS's (and Windows') default case-insensitive filesystem,
	// Lstat("testapp") resolves right back to the "TestApp" regular file —
	// its absence isn't detectable by Lstat failing, only by confirming
	// that path is the binary itself, not a symlink pointing at it. See
	// TestBuildAppBundle_CLISymlink below for the case where the two
	// genuinely differ and a real symlink is expected.
	info, err := os.Lstat(filepath.Join(appPath, "Contents", "MacOS", "testapp"))
	if err != nil {
		t.Fatalf("Lstat(testapp): %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("no separate CLI symlink should be created when it would case-insensitively collide with the executable")
	}
}

func TestBuildAppBundle_CLISymlink(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Identity.Name = "KrankyBear InstallerBear" // slug -> "krankybear-installerbear", distinct from BundleExecutable below
	proj.MacOS.BundleExecutable = "KrankyBearInstallerBear"
	proj.Binaries = []packproject.BinaryEntry{{OS: "darwin", Arch: "arm64", Path: proj.Binaries[0].Path}}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}

	link := filepath.Join(appPath, "Contents", "MacOS", "krankybear-installerbear")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("expected a CLI-name symlink %q: %v", link, err)
	}
	if target != "KrankyBearInstallerBear" {
		t.Errorf("CLI symlink target = %q, want KrankyBearInstallerBear", target)
	}
}

func TestBuildAppBundle_PayloadOSFilter(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Payload = []packproject.PayloadEntry{
		{Source: "assets/images", Dest: "assets/images", Recursive: true, OS: []string{"windows"}},
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "MacOS", "assets", "images")); err == nil {
		t.Error("a windows-only payload entry should not be copied into the macOS bundle")
	}
}

// TestBuildAppBundle_PayloadFileIntoSubdirectory is a regression test: Dest
// on a non-recursive entry used to be treated as the exact destination file
// path, so a subdirectory Dest (distinct from Source's own layout, exactly
// what an imported Inno [Files] remap like "mesa-win\opengl32.dll" ->
// "mesa-fallback" needs) silently wrote a file literally named after the
// directory instead of preserving the real filename. Dest must always be a
// directory — see packproject.PayloadEntry's own doc comment.
func TestBuildAppBundle_PayloadFileIntoSubdirectory(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Payload = []packproject.PayloadEntry{
		{Source: "assets/images/icon.png", Dest: "mesa-fallback", Recursive: false},
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	want := filepath.Join(appPath, "Contents", "MacOS", "mesa-fallback", "icon.png")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to exist: %v", want, err)
	}
}

func TestBuildAppBundle_PayloadExcludes(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "assets", "images", "excluded.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	proj.Payload = []packproject.PayloadEntry{
		{Source: "assets/images", Dest: "assets/images", Recursive: true, Excludes: []string{"*.tmp"}},
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "MacOS", "assets", "images", "excluded.tmp")); err == nil {
		t.Error("excluded.tmp should have been skipped by Excludes")
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "MacOS", "assets", "images", "icon.png")); err != nil {
		t.Errorf("icon.png should still have been copied: %v", err)
	}
}

func TestBuildAppBundle_MissingBinaryFailsWithClearError(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Binaries = nil

	_, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "no darwin/arm64 binary registered") {
		t.Fatalf("expected a clear missing-binary error, got %v", err)
	}
}

// TestBuildAppBundle_NoInfoPlistByDefault is a regression test for a real
// bug: this backend used to always synthesize a real Info.plist from
// Identity fields, but this project's own historical package.sh/fpm
// convention deliberately ships every app with no real Info.plist at all
// (verified against a real installed sibling app, /Applications/
// TaniumMigrator.app/Contents/ - only Info-plist.txt, no Info.plist) -
// some IT security scanning apparently flags a real one. pkgbuild's --root
// mode (see build.go) doesn't require one the way --component mode does
// (confirmed empirically: --component hard-refuses a bundle with no
// Info.plist as "not a valid bundle component"), so there's no technical
// reason to force it either.
func TestBuildAppBundle_NoInfoPlistByDefault(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		t.Error("expected no Contents/Info.plist to be created by default")
	}
}

// TestBuildAppBundle_CopiesRealInfoPlist covers the opt-in: a user who
// wants a real Info.plist gets one by deliberately placing a file literally
// named Info.plist in the project directory (e.g. renaming their own
// Info-plist.txt placeholder) - copied verbatim, not regenerated from
// Identity fields, so their own custom content is respected exactly.
func TestBuildAppBundle_CopiesRealInfoPlist(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	const plistContent = "<?xml version=\"1.0\"?>\n<!-- a hand-authored plist -->\n"
	if err := os.WriteFile(filepath.Join(dir, "Info.plist"), []byte(plistContent), 0o644); err != nil {
		t.Fatal(err)
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		t.Fatalf("expected Contents/Info.plist to exist: %v", err)
	}
	if string(data) != plistContent {
		t.Errorf("expected the real Info.plist to be copied verbatim, got:\n%s", data)
	}
}

// TestBuildAppBundle_CopiesPlaceholderDocs covers Info-plist.txt/
// Readme-plist.txt: this project's own historical package.sh copies these
// straight into Contents/ (a sibling of MacOS/) as harmless documentation,
// and a Payload entry can't reach that location (Payload always lands
// under Contents/MacOS/), so macpkg auto-copies them itself when present -
// with no effect on Info.plist handling, which stays independent.
func TestBuildAppBundle_CopiesPlaceholderDocs(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	const infoTxt = "placeholder - rename to Info.plist to activate\n"
	const readmeTxt = "placeholder - rename to Readme.plist to activate\n"
	if err := os.WriteFile(filepath.Join(dir, "Info-plist.txt"), []byte(infoTxt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Readme-plist.txt"), []byte(readmeTxt), 0o644); err != nil {
		t.Fatal(err)
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info-plist.txt"))
	if err != nil || string(got) != infoTxt {
		t.Errorf("expected Contents/Info-plist.txt to be copied verbatim, got %q, err %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(appPath, "Contents", "Readme-plist.txt"))
	if err != nil || string(got) != readmeTxt {
		t.Errorf("expected Contents/Readme-plist.txt to be copied verbatim, got %q, err %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		t.Error("expected copying the placeholder docs to have no effect on real Info.plist handling")
	}
}

// TestBuildAppBundle_FileAssociationsInjectedIntoRealInfoPlist covers the
// main new case: a project with a real, hand-placed Info.plist AND
// FileAssociations configured gets CFBundleDocumentTypes merged into the
// copy that lands in Contents/Info.plist - real file-type association
// requires this, and this backend can now provide it without abandoning
// the "never synthesize from scratch" policy, since it's built from the
// project's own existing plist content.
func TestBuildAppBundle_FileAssociationsInjectedIntoRealInfoPlist(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp", Description: "My App Project"}}
	if err := os.WriteFile(filepath.Join(dir, "Info.plist"), []byte(testPlist), 0o644); err != nil {
		t.Fatal(err)
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		t.Fatalf("expected Contents/Info.plist to exist: %v", err)
	}
	if !strings.Contains(string(data), "CFBundleDocumentTypes") {
		t.Errorf("expected CFBundleDocumentTypes to be merged in, got:\n%s", data)
	}
	if !strings.Contains(string(data), "<key>CFBundleName</key>") || !strings.Contains(string(data), "<string>TestApp</string>") {
		t.Errorf("expected the original plist content to still be present, got:\n%s", data)
	}
}

// TestBuildAppBundle_FileAssociationsPromoteInfoPlistTxtToFunctional covers
// the case Allan asked for directly: no real Info.plist exists yet, only
// the inert Info-plist.txt placeholder - but FileAssociations are
// configured, so a real functional Contents/Info.plist (built from
// Info-plist.txt's own content, plus the association entries) should be
// produced anyway, making file associations reachable without forcing a
// permanent rename. The separate, unmodified documentation copy at
// Contents/Info-plist.txt must still happen too, untouched.
func TestBuildAppBundle_FileAssociationsPromoteInfoPlistTxtToFunctional(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp", Description: "My App Project"}}
	if err := os.WriteFile(filepath.Join(dir, "Info-plist.txt"), []byte(testPlist), 0o644); err != nil {
		t.Fatal(err)
	}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		t.Fatalf("expected Contents/Info.plist to be synthesized from Info-plist.txt: %v", err)
	}
	if !strings.Contains(string(data), "CFBundleDocumentTypes") {
		t.Errorf("expected CFBundleDocumentTypes to be merged in, got:\n%s", data)
	}

	docCopy, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info-plist.txt"))
	if err != nil {
		t.Fatalf("expected Contents/Info-plist.txt documentation copy to still exist: %v", err)
	}
	if string(docCopy) != testPlist {
		t.Error("expected the Info-plist.txt documentation copy to remain the original, unmodified content")
	}
}

// TestBuildAppBundle_FileAssociationsNoOpWithoutAnyPlistSource confirms the
// existing "no Info.plist unless one is deliberately provided" policy still
// holds when FileAssociations are configured but the project directory has
// neither a real Info.plist nor an Info-plist.txt to build one from -
// there's nothing to inject into, so nothing is synthesized from scratch.
func TestBuildAppBundle_FileAssociationsNoOpWithoutAnyPlistSource(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp"}}

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appPath, "Contents", "Info.plist")); err == nil {
		t.Error("expected no Contents/Info.plist when neither Info.plist nor Info-plist.txt exists")
	}
}
