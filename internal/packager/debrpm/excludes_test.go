package debrpm

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

// TestBuildInfo_PayloadFileIntoSubdirectory is a regression test for the
// same Dest-semantics bug fixed for macpkg (see its own test of the same
// name): Dest on a non-recursive entry must always be a destination
// directory, with the installed filename taken from Source's own basename.
func TestBuildInfo_PayloadFileIntoSubdirectory(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)

	assetsDir := filepath.Join(dir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "icon.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	proj.Payload = []packproject.PayloadEntry{
		{Source: "assets/images/icon.png", Dest: "mesa-fallback", Recursive: false},
	}

	info, cleanup, err := buildInfo(proj, "amd64", proj.Binaries[0])
	if err != nil {
		t.Fatalf("buildInfo: %v", err)
	}
	defer cleanup()

	want := filepath.Join(proj.Install.Linux, "mesa-fallback", "icon.png")
	var found bool
	for _, c := range info.Overridables.Contents {
		if c.Destination == want {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a content entry destined for mesa-fallback/icon.png, got %+v", info.Overridables.Contents)
	}
}

func TestBuildInfo_PayloadExcludes(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)

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
	proj.Payload = []packproject.PayloadEntry{
		{Source: "assets/images", Dest: "assets/images", Recursive: true, Excludes: []string{"*.tmp"}},
	}

	info, cleanup, err := buildInfo(proj, "amd64", proj.Binaries[0])
	if err != nil {
		t.Fatalf("buildInfo: %v", err)
	}
	defer cleanup()

	var sawIcon, sawExcluded bool
	for _, c := range info.Overridables.Contents {
		switch filepath.Base(c.Destination) {
		case "icon.png":
			sawIcon = true
		case "excluded.tmp":
			sawExcluded = true
		}
	}
	if !sawIcon {
		t.Error("expected icon.png in the resulting contents")
	}
	if sawExcluded {
		t.Error("excluded.tmp should have been skipped by Excludes")
	}
}
