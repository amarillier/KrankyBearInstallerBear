package projectscan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanEmptyDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	got := Scan(dir)
	if got != (Defaults{}) {
		t.Errorf("expected zero Defaults for empty dir, got %+v", got)
	}
}

func TestScanFullConvention(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".git", "config"), `
[remote "origin"]
	url = https://github.com/amarillier/MyApp.git
[user]
	name = Allan Marillier
`)
	writeFile(t, filepath.Join(dir, "LICENSE.md"), "MIT")
	writeFile(t, filepath.Join(dir, "assets", "images", "icon.ico"), "x")
	writeFile(t, filepath.Join(dir, "assets", "images", "icon.icns"), "x")
	writeFile(t, filepath.Join(dir, "assets", "images", "icon.png"), "x")

	got := Scan(dir)
	want := Defaults{
		URL:         "https://github.com/amarillier/MyApp",
		Publisher:   "Allan Marillier",
		LicenseFile: "LICENSE.md",
		IconICO:     filepath.Join("assets", "images", "icon.ico"),
		IconICNS:    filepath.Join("assets", "images", "icon.icns"),
		IconPNG:     filepath.Join("assets", "images", "icon.png"),
	}
	if got != want {
		t.Errorf("Scan() = %+v, want %+v", got, want)
	}
}

func TestFindLicenseFilePriorityOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "LICENSE.txt"), "x")
	writeFile(t, filepath.Join(dir, "LICENSE.md"), "x")

	got := findLicenseFile(dir)
	if got != "LICENSE.md" {
		t.Errorf("findLicenseFile() = %q, want LICENSE.md (higher priority than .txt)", got)
	}
}

func TestFindLicenseFileCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "license"), "x")

	got := findLicenseFile(dir)
	if got != "license" {
		t.Errorf("findLicenseFile() = %q, want license", got)
	}
}
