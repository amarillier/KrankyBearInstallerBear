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
		"Contents/Info.plist",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(appPath, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
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

func TestBuildAppBundle_MissingBinaryFailsWithClearError(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Binaries = nil

	_, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "no darwin/arm64 binary registered") {
		t.Fatalf("expected a clear missing-binary error, got %v", err)
	}
}

func TestBuildAppBundle_InfoPlistContent(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.MacOS.MinSystemVersion = "10.13"
	proj.MacOS.Category = "public.app-category.utilities"

	appPath, err := BuildAppBundle(proj, "arm64", filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("BuildAppBundle: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(appPath, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		"<key>CFBundleExecutable</key>\n\t<string>TestApp</string>",
		"<key>CFBundleIdentifier</key>\n\t<string>com.example.testapp</string>",
		"<key>CFBundleShortVersionString</key>\n\t<string>1.2.3</string>",
		"<key>LSMinimumSystemVersion</key>\n\t<string>10.13</string>",
		"<key>LSApplicationCategoryType</key>\n\t<string>public.app-category.utilities</string>",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Info.plist missing expected content: %q\nfull plist:\n%s", want, content)
		}
	}
}
