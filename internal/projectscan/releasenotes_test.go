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

// TestParseReleaseNotesMarkdownConvention covers this project's own
// ReleaseNotes.md convention as of 0.5.0 - real Markdown "#"/"##" headings
// instead of the older bare-heading + decorative underline style - to
// confirm the switch to proper Markdown didn't regress the smart-scan
// prefill this same parser provides for "New Project".
func TestParseReleaseNotesMarkdownConvention(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ReleaseNotes.md"), `# Release Notes
KrankyBear InstallerBear: cross-platform GUI + CLI for packaging pre-built binaries
into native Windows, macOS, and Linux installers, from one shared project file.

## Future Ideas

- something not a version line

## Version 0.2.0 - September 04, 2026

- stuff

## Version 0.1.0 - September 03, 2026

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

// TestParseReleaseNotesRealFixture confirms parseReleaseNotes against this
// repo's own real, committed ReleaseNotes.md - not just a synthetic
// fixture - the same "verify against the real file" convention this
// project's other parsers (innoimport in particular) already follow.
func TestParseReleaseNotesRealFixture(t *testing.T) {
	name, version, description := parseReleaseNotes("../..")
	if name != "KrankyBear InstallerBear" {
		t.Errorf("name = %q, want %q", name, "KrankyBear InstallerBear")
	}
	if version == "" {
		t.Error("expected a non-empty version from the real ReleaseNotes.md")
	}
	if description == "" {
		t.Error("expected a non-empty description from the real ReleaseNotes.md")
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
