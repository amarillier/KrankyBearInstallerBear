package packproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const commentedProjectYAML = `# Example InstallerBear project file.
# Adapt before reuse.
identity:
    name: TestApp
    id: com.example.testapp
    # version bump pending review
    version: "1.0.0"
binaries:
    - os: windows
      arch: amd64
      path: bin/testapp.exe
payload:
    - source: assets/i18n  # locale packs
      dest: assets/i18n
      recursive: true
    # - source: assets/images
    #   dest: assets/images
    - source: ReleaseNotes.md
      dest: ""
targets:
    - deb
    - rpm
    # - winmsi disabled for now
`

func writeAndLoad(t *testing.T, content string) *Project {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "installerbear.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	proj, err := LoadLenient(path)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	return proj
}

func TestMarshal_PreservesHeaderComment(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# Example InstallerBear project file.") {
		t.Errorf("expected the header comment to survive, got:\n%s", out)
	}
}

func TestMarshal_PreservesCommentOnEditedField(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)
	proj.Identity.Version = "2.0.0" // simulate a GUI edit

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# version bump pending review") {
		t.Errorf("expected the version comment to survive an edit to the field itself, got:\n%s", out)
	}
	if !strings.Contains(out, `version: 2.0.0`) && !strings.Contains(out, `version: "2.0.0"`) {
		t.Errorf("expected the edited value to actually take effect, got:\n%s", out)
	}
}

func TestMarshal_PreservesPayloadEntryCommentAfterListGrows(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)
	proj.Payload = append(proj.Payload, PayloadEntry{Source: "extra.txt"})

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# locale packs") {
		t.Errorf("expected the payload entry's own comment to survive appending a new entry, got:\n%s", out)
	}
	if !strings.Contains(out, "assets/images") {
		t.Errorf("expected the commented-out payload entry to still be present as a comment, got:\n%s", out)
	}
	if !strings.Contains(out, "extra.txt") {
		t.Errorf("expected the newly appended entry to actually be there, got:\n%s", out)
	}
}

func TestMarshal_PreservesTargetCommentAfterReorder(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)
	proj.Targets = []string{"rpm", "deb"} // reordered

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "winmsi disabled for now") {
		t.Errorf("expected the targets comment to survive reordering (matched by value, not position), got:\n%s", out)
	}
}

func TestMarshal_HandBuiltProjectUnaffected(t *testing.T) {
	// No sourceNode at all (never loaded from a file) - must behave exactly
	// like a plain struct marshal, the same as before this feature existed.
	proj := &Project{
		Identity: Identity{Name: "TestApp", Version: "1.0.0"},
		Targets:  []string{"deb"},
	}
	proj.Defaults()

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "#") {
		t.Errorf("expected no stray comments in a hand-built project's output, got:\n%s", data)
	}
}

// TestMarshal_DeterministicForUnsavedChangesCheck guards the GUI's own
// unsaved-changes mechanism (editor.isDirty/markSaved in projectio.go),
// which works by comparing two separate packproject.Marshal calls against
// the same *Project - one taken as the "last saved" baseline, one taken
// live. That only works if Marshal is deterministic when nothing has
// changed in between, which comment-preservation must not break.
func TestMarshal_DeterministicForUnsavedChangesCheck(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)

	first, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal (1st): %v", err)
	}
	second, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal (2nd): %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("expected two Marshal calls with no changes in between to be byte-identical, got:\n--- 1st ---\n%s\n--- 2nd ---\n%s", first, second)
	}
}

func TestMarshal_CommentOnDeletedListItemIsDroppedNotCrashed(t *testing.T) {
	proj := writeAndLoad(t, commentedProjectYAML)
	proj.Payload = []PayloadEntry{{Source: "onlyone.txt"}} // drops every original entry

	data, err := Marshal(proj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), "onlyone.txt") {
		t.Errorf("expected the surviving entry to be present, got:\n%s", data)
	}
}
