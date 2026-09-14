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

// TestValidate_WindowsExeNameMustMatchBinaryFilename is a regression test
// for a real, silent bug found via hands-on Windows testing: exe_name set
// to the installer's own output filename (e.g. "AppSetup.exe") instead of
// the actual binary's filename, which NSIS/MSI install unchanged. Nothing
// errors at build time - it just silently breaks the uninstaller's
// taskkill, every shortcut, and launch-after-install (the exact symptom
// that surfaced this: the "launch now" checkbox did nothing, no error).
func TestValidate_WindowsExeNameMustMatchBinaryFilename(t *testing.T) {
	p := validProject()
	p.Targets = []string{TargetWinExe}
	p.Windows.UpgradeGUID = "{4578B785-DB27-44FF-B3F9-2713B327BB90}"
	p.Binaries = []BinaryEntry{{OS: "windows", Arch: "amd64", Path: "bin/TestApp.exe"}}
	p.Windows.ExeName = "TestAppSetup.exe" // mismatch: the real binary is TestApp.exe

	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `exe_name "TestAppSetup.exe" does not match`) {
		t.Fatalf("expected an exe_name mismatch error, got %v", err)
	}
}

func TestValidate_WindowsExeNameMatchingBinaryIsFine(t *testing.T) {
	p := validProject()
	p.Targets = []string{TargetWinExe}
	p.Windows.UpgradeGUID = "{4578B785-DB27-44FF-B3F9-2713B327BB90}"
	p.Binaries = []BinaryEntry{{OS: "windows", Arch: "amd64", Path: "bin/TestApp.exe"}}
	p.Windows.ExeName = "TestApp.exe"

	if err := p.Validate(""); err != nil {
		t.Fatalf("expected no error when exe_name matches the binary's filename: %v", err)
	}
}

func TestValidate_InstallScopeUnknownValueRejected(t *testing.T) {
	p := validProject()
	p.Windows.InstallScope = "everyone-please"

	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `windows.install_scope "everyone-please" is not valid`) {
		t.Fatalf("expected an install_scope validation error, got %v", err)
	}
}

func TestValidate_InstallScopeKnownValuesPass(t *testing.T) {
	for _, scope := range []string{"", InstallScopeAllUsers, InstallScopeCurrentUser} {
		p := validProject()
		p.Windows.InstallScope = scope
		if err := p.Validate(""); err != nil {
			t.Errorf("install_scope %q: expected no error, got %v", scope, err)
		}
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

// TestValidate_PayloadDuplicateDest is a real collision: two different
// source files that would both install as "docs/a.txt" (same Dest
// directory, same final basename) — Dest is always a directory (see
// PayloadEntry's own doc comment), so the collision key is Dest plus the
// installed filename, not Dest alone.
func TestValidate_PayloadDuplicateDest(t *testing.T) {
	p := validProject()
	p.Payload = []PayloadEntry{
		{Source: "sub1/a.txt", Dest: "docs"},
		{Source: "sub2/a.txt", Dest: "docs"},
	}
	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `used by both`) {
		t.Fatalf("expected duplicate dest error, got %v", err)
	}
}

// TestValidate_PayloadSameDestDifferentFilesIsNotACollision is a
// regression test for a real bug hit while migrating this very project:
// three individually-listed mesa-fallback files (opengl32.dll,
// libgallium_wgl.dll, .force-mesa-fallback.sample) all destined for the
// same "mesa-fallback" directory were flagged as duplicate destinations,
// even though each installs under its own distinct filename and none of
// them actually collide. Sharing one destination *directory* across
// several differently-named files is the normal case a non-recursive
// Payload entry is for, not an error.
func TestValidate_PayloadSameDestDifferentFilesIsNotACollision(t *testing.T) {
	p := validProject()
	p.Payload = []PayloadEntry{
		{Source: "assets/mesa-win/opengl32.dll", Dest: "mesa-fallback"},
		{Source: "assets/mesa-win/libgallium_wgl.dll", Dest: "mesa-fallback"},
		{Source: "assets/mesa-win/.force-mesa-fallback.sample", Dest: "mesa-fallback"},
	}
	if err := p.Validate(""); err != nil {
		t.Fatalf("three differently-named files sharing one Dest directory should not error: %v", err)
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

func TestValidate_FileAssociationValidPasses(t *testing.T) {
	p := validProject()
	p.FileAssociations = []FileAssociation{{Extension: ".myp", Description: "My App Project"}}
	if err := p.Validate(""); err != nil {
		t.Fatalf("valid file association should pass: %v", err)
	}
}

func TestValidate_FileAssociationRequiresLeadingDot(t *testing.T) {
	p := validProject()
	p.FileAssociations = []FileAssociation{{Extension: "myp"}}

	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), `must start with a dot`) {
		t.Fatalf("expected a leading-dot error, got %v", err)
	}
}

func TestValidate_FileAssociationEmptyExtensionRejected(t *testing.T) {
	p := validProject()
	p.FileAssociations = []FileAssociation{{Description: "no extension set"}}

	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), "extension is required") {
		t.Fatalf("expected an extension-required error, got %v", err)
	}
}

func TestValidate_FileAssociationDuplicateExtensionRejected(t *testing.T) {
	p := validProject()
	p.FileAssociations = []FileAssociation{
		{Extension: ".myp", Description: "First"},
		{Extension: ".MYP", Description: "Second, different case"},
	}

	err := p.Validate("")
	if err == nil || !strings.Contains(err.Error(), "registered more than once") {
		t.Fatalf("expected a duplicate-extension error (case-insensitive), got %v", err)
	}
}
