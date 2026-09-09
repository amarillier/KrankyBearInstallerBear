package packproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validProject() *Project {
	p := &Project{
		Identity: Identity{Name: "TestApp", ID: "com.example.testapp", Version: "1.0.0"},
		Targets:  []string{TargetDEB, TargetRPM},
	}
	p.Defaults()
	return p
}

func TestValidate_RequiredFields(t *testing.T) {
	p := &Project{}
	err := p.Validate("")
	if err == nil {
		t.Fatal("expected error for missing name/version/id")
	}
	for _, want := range []string{"identity.name", "identity.version", "identity.id"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing mention of %q", err, want)
		}
	}
}

func TestValidate_UnknownTarget(t *testing.T) {
	p := validProject()
	p.Targets = []string{"deb", "bogus"}
	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `unknown target "bogus"`) {
		t.Fatalf("expected unknown target error, got %v", err)
	}
}

func TestValidate_WindowsGUIDRequiredOnlyForWindowsTargets(t *testing.T) {
	p := validProject() // deb/rpm only, no GUID
	if err := p.Validate(""); err != nil {
		t.Fatalf("deb/rpm-only project should not require a GUID: %v", err)
	}

	p.Targets = []string{TargetWinExe}
	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), "upgrade_guid is required") {
		t.Fatalf("expected missing GUID error, got %v", err)
	}

	p.Windows.UpgradeGUID = "not-a-guid"
	err = p.Validate("")
	if err == nil || !strings.Contains(err.Error(), "is not a valid GUID") {
		t.Fatalf("expected invalid GUID error, got %v", err)
	}

	p.Windows.UpgradeGUID = "{4578B785-DB27-44FF-B3F9-2713B327BB90}"
	if err := p.Validate(""); err != nil {
		t.Fatalf("valid GUID should pass: %v", err)
	}
}

func TestValidate_PayloadDuplicateDest(t *testing.T) {
	p := validProject()
	p.Payload = []PayloadEntry{
		{Source: "a.txt", Dest: "docs/a.txt"},
		{Source: "b.txt", Dest: "docs/a.txt"},
	}
	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `used by both`) {
		t.Fatalf("expected duplicate dest error, got %v", err)
	}
}

func TestValidate_PayloadPathTraversal(t *testing.T) {
	p := validProject()
	p.Payload = []PayloadEntry{{Source: "a.txt", Dest: "../../etc/passwd"}}
	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), "escapes the install root") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}

func TestValidate_PayloadMissingSource(t *testing.T) {
	dir := t.TempDir()
	p := validProject()
	p.Payload = []PayloadEntry{{Source: "does-not-exist.txt", Dest: "does-not-exist.txt"}}
	err := p.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing source error, got %v", err)
	}
}

func TestValidate_PayloadCircularSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "circular")
	if err := os.Symlink("circular", link); err != nil {
		t.Fatal(err)
	}

	p := validProject()
	p.Payload = []PayloadEntry{{Source: "circular", Dest: "circular"}}
	err := p.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "circular or broken symlink") {
		t.Fatalf("expected circular symlink error, got %v", err)
	}
}

// TestValidate_PayloadRecursiveFileFailsWithClearError is a regression test
// for a real config a hand-edited installerbear.yaml produced: a plain
// file (LICENSE) marked recursive: true. Every backend either failed
// outright (winexe's NSIS "File /r LICENSE\*.*" -> "no files found") or
// silently produced a wrong nested layout (winmsi et al.) instead of
// erroring — Validate should catch this up front with a clear message
// instead of letting it reach a backend at all.
func TestValidate_PayloadRecursiveFileFailsWithClearError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := validProject()
	p.Payload = []PayloadEntry{{Source: "LICENSE", Dest: "LICENSE", Recursive: true}}

	err := p.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "marked recursive but is a file") {
		t.Fatalf("expected a recursive-but-file error, got %v", err)
	}
}

func TestValidate_PayloadDirectoryNotMarkedRecursiveFailsWithClearError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := validProject()
	p.Payload = []PayloadEntry{{Source: "images", Dest: "images", Recursive: false}}

	err := p.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "is a directory but not marked recursive") {
		t.Fatalf("expected a directory-not-recursive error, got %v", err)
	}
}

func TestValidate_PayloadValidSourcePasses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := validProject()
	p.Payload = []PayloadEntry{{Source: "a.txt", Dest: "a.txt"}}
	if err := p.Validate(dir); err != nil {
		t.Fatalf("valid payload should pass: %v", err)
	}
}
