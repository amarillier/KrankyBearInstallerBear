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
