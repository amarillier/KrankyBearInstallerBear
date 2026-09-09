package debrpm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goreleaser/nfpm/v2/files"

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

// TestBuildInfo_PayloadExcludesOwnsDirectories is a regression test for a
// real bug found via `rpm -e` on a real machine: every payload file was
// correctly removed on uninstall, but every directory it had lived in was
// left behind, empty, forever. Root cause: excludeFilteredTree (the path
// used whenever a Recursive entry has Excludes set) emitted TypeFile
// entries only, silently dropping every directory it walked over instead
// of emitting a files.TypeDir entry for it - unlike nfpm's own TypeTree
// expansion, which does emit those, and which is exactly why an
// Excludes-free Recursive entry never had this problem. rpm's builder
// specifically needs an explicit TypeDir (not just an implicit
// parent-of-file) to register directory ownership and clean it up again -
// confirmed both via nfpm's own source (rpm.go's createFilesInsideRPM
// skips TypeImplicitDir) and by rebuilding a real .rpm and checking
// `rpm -qlv` actually lists these paths with directory mode bits.
func TestBuildInfo_PayloadExcludesOwnsDirectories(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)

	nestedDir := filepath.Join(dir, "assets", "images", "eggs")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "01.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "images", "excluded.tmp"), []byte("x"), 0o644); err != nil {
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

	wantDir := filepath.Join(proj.Install.Linux, "assets", "images", "eggs")
	var found bool
	for _, c := range info.Overridables.Contents {
		if c.Destination == wantDir {
			if c.Type != files.TypeDir {
				t.Errorf("expected %q to be a TypeDir entry, got type %q", wantDir, c.Type)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("expected an explicit directory entry for %q so rpm/deb actually own and clean it up, got %+v", wantDir, info.Overridables.Contents)
	}
}
