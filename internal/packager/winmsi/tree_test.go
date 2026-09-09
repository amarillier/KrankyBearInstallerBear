package winmsi

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

// allFileNames flattens every fileNode's Name across the whole tree, for
// asserting on presence/absence without caring which directory a file
// landed in.
func allFileNames(d *dirNode) []string {
	var names []string
	for _, f := range d.Files {
		names = append(names, f.Name)
	}
	for _, child := range d.Dirs {
		names = append(names, allFileNames(child)...)
	}
	return names
}

func TestBuildDirTree_PayloadExcludes(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "testapp.exe")
	if err := os.WriteFile(binPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(dir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "icon.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "excluded.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj := &packproject.Project{
		BaseDir:  dir,
		Identity: packproject.Identity{Name: "TestApp"},
		Windows:  packproject.WindowsOptions{ExeName: "TestApp.exe"},
		Payload: []packproject.PayloadEntry{
			{Source: "assets/images", Dest: "assets/images", Recursive: true, Excludes: []string{"*.tmp"}},
		},
	}
	bin := packproject.BinaryEntry{OS: "windows", Arch: "amd64", Path: binPath}

	root, _, err := buildDirTree(proj, bin)
	if err != nil {
		t.Fatalf("buildDirTree: %v", err)
	}

	names := allFileNames(root)
	for _, want := range []string{"TestApp.exe", "icon.png"} {
		if !containsStr(names, want) {
			t.Errorf("expected %q in tree, got %v", want, names)
		}
	}
	if containsStr(names, "excluded.tmp") {
		t.Errorf("excluded.tmp should have been skipped by Excludes, got %v", names)
	}
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
