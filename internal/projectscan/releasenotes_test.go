package projectscan

import (
	"path/filepath"
	"testing"
)

func TestParseReleaseNotesFullConvention(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ReleaseNotes.txt"), `Release notes
KrankyBear InstallerBear: cross-platform GUI + CLI for packaging pre-built binaries
into native Windows, macOS, and Linux installers, from one shared project file.

Future ideas:
- something not a version line

Version 0.2.0 - September 04, 2026
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- stuff

Version 0.1.0 - September 03, 2026
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- older stuff
`)

	name, version, description := parseReleaseNotes(dir)
	wantName := "KrankyBear InstallerBear"
	wantVersion := "0.2.0"
	wantDesc := "cross-platform GUI + CLI for packaging pre-built binaries into native Windows, macOS, and Linux installers, from one shared project file."
	if name != wantName {
		t.Errorf("name = %q, want %q", name, wantName)
	}
	if version != wantVersion {
		t.Errorf("version = %q, want %q", version, wantVersion)
	}
	if description != wantDesc {
		t.Errorf("description = %q, want %q", description, wantDesc)
	}
}

func TestParseReleaseNotesNoFile(t *testing.T) {
	dir := t.TempDir()
	name, version, description := parseReleaseNotes(dir)
	if name != "" || version != "" || description != "" {
		t.Errorf("parseReleaseNotes() = (%q, %q, %q), want all empty", name, version, description)
	}
}

func TestParseReleaseNotesNoNameColonLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ReleaseNotes.txt"), `Some tool without the naming convention this template expects

Version 1.0.0 - January 01, 2026
`)

	name, version, description := parseReleaseNotes(dir)
	if name != "" || description != "" {
		t.Errorf("name/description = %q/%q, want both empty (no ': ' in intro)", name, description)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", version)
	}
}
