package main

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

func TestEditor_IsDirty_CleanRightAfterConstruction(t *testing.T) {
	e := newTestEditor(t)
	if e.isDirty() {
		t.Error("a freshly constructed editor should not be dirty")
	}
}

func TestEditor_IsDirty_TracksEditsAndMarkSaved(t *testing.T) {
	e := newTestEditor(t)

	e.nameEntry.SetText("Changed Name")
	if !e.isDirty() {
		t.Error("editing a field should make the project dirty")
	}

	e.markSaved()
	if e.isDirty() {
		t.Error("markSaved should reset the dirty baseline to the current state")
	}
}

func TestEditor_ConfirmDiscardIfDirty_ProceedsImmediatelyWhenClean(t *testing.T) {
	e := newTestEditor(t)

	var called bool
	e.confirmDiscardIfDirty(func() { called = true })

	if !called {
		t.Error("a clean project should proceed immediately, with no confirmation needed")
	}
}

// TestEditor_ConfirmDiscardIfDirty_AsksFirstWhenDirty confirms proceed is
// NOT called synchronously for a dirty project — it should only run after
// the user confirms via the dialog, never as a side effect of just calling
// confirmDiscardIfDirty itself (which would defeat the whole point: New/
// Open silently discarding real unsaved work).
func TestEditor_ConfirmDiscardIfDirty_AsksFirstWhenDirty(t *testing.T) {
	e := newTestEditor(t)
	e.nameEntry.SetText("Changed Name")

	var called bool
	e.confirmDiscardIfDirty(func() { called = true })

	if called {
		t.Error("proceed must not run synchronously for a dirty project — it should wait for dialog confirmation")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestApplyScannedDefaults_FullConvention exercises the whole New Project
// smart-default chain end to end: ReleaseNotes.txt's Name/Version/
// Description, a conventional bin/ folder auto-filling Binaries, and the
// Windows.ExeName/MacOS.BundleExecutable defaults derived from those.
func TestApplyScannedDefaults_FullConvention(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "ReleaseNotes.txt"), `Release notes
Test App: does a thing.

Version 1.2.3 - January 01, 2026
`)
	writeTestFile(t, filepath.Join(dir, "bin", "TestApp.exe"), "x")
	writeTestFile(t, filepath.Join(dir, "bin", "testapp-macos-amd64"), "x")

	e := newTestEditor(t)
	e.proj = &packproject.Project{BaseDir: dir}
	e.applyScannedDefaults(dir)

	if e.proj.Identity.Name != "Test App" {
		t.Errorf("Identity.Name = %q, want %q", e.proj.Identity.Name, "Test App")
	}
	if e.proj.Identity.Version != "1.2.3" {
		t.Errorf("Identity.Version = %q, want %q", e.proj.Identity.Version, "1.2.3")
	}
	if e.proj.Identity.Description != "does a thing." {
		t.Errorf("Identity.Description = %q, want %q", e.proj.Identity.Description, "does a thing.")
	}

	winBin, ok := e.proj.BinaryFor("windows", "amd64")
	if !ok || winBin.Path != filepath.Join(dir, "bin", "TestApp.exe") {
		t.Errorf("windows/amd64 binary = %+v, ok=%v, want the scanned bin/TestApp.exe", winBin, ok)
	}
	macBin, ok := e.proj.BinaryFor("darwin", "amd64")
	if !ok || macBin.Path != filepath.Join(dir, "bin", "testapp-macos-amd64") {
		t.Errorf("darwin/amd64 binary = %+v, ok=%v, want the scanned bin/testapp-macos-amd64", macBin, ok)
	}

	if e.proj.Windows.ExeName != "TestApp.exe" {
		t.Errorf("Windows.ExeName = %q, want %q (basename of the scanned Windows binary)", e.proj.Windows.ExeName, "TestApp.exe")
	}
	if e.proj.MacOS.BundleExecutable != "TestApp" {
		t.Errorf("MacOS.BundleExecutable = %q, want %q (Identity.Name with spaces stripped)", e.proj.MacOS.BundleExecutable, "TestApp")
	}
}

// TestApplyScannedDefaults_NeverOverwritesExisting confirms every field
// this scan can fill is skipped once already set — the same "only fill
// blanks" guarantee the git/license/icon scan already had.
func TestApplyScannedDefaults_NeverOverwritesExisting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "ReleaseNotes.txt"), `Release notes
Scanned Name: scanned description.

Version 9.9.9 - January 01, 2026
`)
	writeTestFile(t, filepath.Join(dir, "bin", "ScannedApp.exe"), "x")

	e := newTestEditor(t)
	e.proj = &packproject.Project{
		BaseDir: dir,
		Identity: packproject.Identity{
			Name:        "Kept Name",
			Version:     "1.0.0",
			Description: "kept description",
		},
		Binaries: []packproject.BinaryEntry{
			{OS: "windows", Arch: "amd64", Path: "already-set.exe"},
		},
		Windows: packproject.WindowsOptions{ExeName: "AlreadySet.exe"},
		MacOS:   packproject.MacOSOptions{BundleExecutable: "AlreadySet"},
	}
	e.applyScannedDefaults(dir)

	if e.proj.Identity.Name != "Kept Name" {
		t.Errorf("Identity.Name = %q, want unchanged %q", e.proj.Identity.Name, "Kept Name")
	}
	if e.proj.Identity.Version != "1.0.0" {
		t.Errorf("Identity.Version = %q, want unchanged %q", e.proj.Identity.Version, "1.0.0")
	}
	if e.proj.Identity.Description != "kept description" {
		t.Errorf("Identity.Description = %q, want unchanged %q", e.proj.Identity.Description, "kept description")
	}
	bin, ok := e.proj.BinaryFor("windows", "amd64")
	if !ok || bin.Path != "already-set.exe" {
		t.Errorf("windows/amd64 binary = %+v, ok=%v, want unchanged already-set.exe", bin, ok)
	}
	if e.proj.Windows.ExeName != "AlreadySet.exe" {
		t.Errorf("Windows.ExeName = %q, want unchanged %q", e.proj.Windows.ExeName, "AlreadySet.exe")
	}
	if e.proj.MacOS.BundleExecutable != "AlreadySet" {
		t.Errorf("MacOS.BundleExecutable = %q, want unchanged %q", e.proj.MacOS.BundleExecutable, "AlreadySet")
	}
}
