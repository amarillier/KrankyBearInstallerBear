package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
)

// TestMain suppresses checkAllPreflight's background goroutine for every
// test in this package — see targetselect.go's disablePreflightAutoCheck
// comment on why (avoids racing test window teardown; the goroutine itself
// is exercised by the real app, not by these tests).
func TestMain(m *testing.M) {
	disablePreflightAutoCheck = true
	os.Exit(m.Run())
}

// newTestEditor builds a real editor against Fyne's headless test driver —
// no window server needed, but every widget/dialog behaves exactly as it
// would in the real app.
func newTestEditor(t *testing.T) *editor {
	t.Helper()
	a := test.NewApp()
	win := test.NewWindow(nil)
	t.Cleanup(win.Close)
	e := newEditor(a, win)
	win.SetContent(e.content())
	return e
}

func TestEditor_IdentityBindingWritesIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.nameEntry.SetText("Test App")
	e.versionEntry.SetText("1.2.3")
	e.idEntry.SetText("com.example.testapp")

	if e.proj.Identity.Name != "Test App" {
		t.Errorf("Identity.Name = %q, want %q", e.proj.Identity.Name, "Test App")
	}
	if e.proj.Identity.Version != "1.2.3" {
		t.Errorf("Identity.Version = %q, want %q", e.proj.Identity.Version, "1.2.3")
	}
	if e.proj.Identity.ID != "com.example.testapp" {
		t.Errorf("Identity.ID = %q, want %q", e.proj.Identity.ID, "com.example.testapp")
	}
}

func TestEditor_TargetToggleAddsAndRemoves(t *testing.T) {
	e := newTestEditor(t)

	test.Tap(e.targetChecks[packproject.TargetDEB])
	if !containsStr(e.proj.Targets, packproject.TargetDEB) {
		t.Fatalf("Targets = %v, want it to contain %q after checking the box", e.proj.Targets, packproject.TargetDEB)
	}

	test.Tap(e.targetChecks[packproject.TargetDEB])
	if containsStr(e.proj.Targets, packproject.TargetDEB) {
		t.Fatalf("Targets = %v, want %q removed after unchecking the box", e.proj.Targets, packproject.TargetDEB)
	}
}

// binaryFieldEntry finds the Entry bound to one (os, arch) pair, for tests.
func (e *editor) binaryFieldEntry(os, arch string) *widget.Entry {
	for _, f := range e.binaryFields {
		if f.os == os && f.arch == arch {
			return f.entry
		}
	}
	return nil
}

// TestEditor_RefreshBinariesPreservesMultiArch is a regression test for a
// real bug in an earlier design caught before it ever ran for real: a
// single Entry + arch dropdown per OS meant refreshBinariesTab's SetText
// fired setBinary in a way that replaced *every* BinaryEntry for that OS —
// silently dropping a second arch entry merely by opening (not editing) a
// hand-authored multi-arch project. Each (OS, arch) now has its own
// dedicated field, which should make that structurally impossible.
func TestEditor_RefreshBinariesPreservesMultiArch(t *testing.T) {
	e := newTestEditor(t)

	e.proj.Binaries = []packproject.BinaryEntry{
		{OS: "windows", Arch: "amd64", Path: "bin/app-amd64.exe"},
		{OS: "windows", Arch: "arm64", Path: "bin/app-arm64.exe"},
	}

	e.refreshBinariesTab()

	if len(e.proj.Binaries) != 2 {
		t.Fatalf("Binaries = %v, want both windows entries preserved across a refresh (len 2)", e.proj.Binaries)
	}
	if got := e.binaryFieldEntry("windows", "amd64").Text; got != "bin/app-amd64.exe" {
		t.Errorf("windows/amd64 field = %q, want bin/app-amd64.exe", got)
	}
	if got := e.binaryFieldEntry("windows", "arm64").Text; got != "bin/app-arm64.exe" {
		t.Errorf("windows/arm64 field = %q, want bin/app-arm64.exe", got)
	}
}

// TestEditor_BinaryFieldsAreIndependent confirms editing one (OS, arch)
// field never touches another — the property that replaced the old
// "e.loading guard" design entirely, rather than working around a shared
// per-OS field's destructive replace-on-change behavior.
func TestEditor_BinaryFieldsAreIndependent(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Binaries = []packproject.BinaryEntry{
		{OS: "windows", Arch: "amd64", Path: "bin/app-amd64.exe"},
		{OS: "windows", Arch: "arm64", Path: "bin/app-arm64.exe"},
	}
	e.refreshAll()

	e.binaryFieldEntry("windows", "amd64").SetText("bin/app-amd64-v2.exe")

	if got, ok := e.proj.BinaryFor("windows", "amd64"); !ok || got.Path != "bin/app-amd64-v2.exe" {
		t.Errorf("BinaryFor(windows, amd64) = %+v, ok=%v; want the edited path", got, ok)
	}
	if got, ok := e.proj.BinaryFor("windows", "arm64"); !ok || got.Path != "bin/app-arm64.exe" {
		t.Errorf("BinaryFor(windows, arm64) = %+v, ok=%v; editing amd64 must not touch arm64", got, ok)
	}
}

func TestEditor_StartBuildRejectsEmptyTargets(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Identity = packproject.Identity{Name: "Test App", ID: "com.example.testapp", Version: "1.0.0"}
	e.proj.Targets = nil // nothing selected

	e.startBuild()

	if e.build.IsRunning() {
		t.Error("startBuild should not start a run when no targets are selected")
	}
}

func TestEditor_StartBuildRejectsInvalidProject(t *testing.T) {
	e := newTestEditor(t)
	// No Identity fields set at all — Validate should reject this before
	// any build attempt starts.
	e.proj.Targets = []string{packproject.TargetDEB}

	e.startBuild()

	if e.build.IsRunning() {
		t.Error("startBuild should not start a run for a project missing required Identity fields")
	}
}

// TestEditor_StartBuildProducesRealOutput drives the actual Start Build
// button path (startBuild -> runBuild's goroutine -> packager.Run) against
// the deb/rpm targets, which need no external tool, to prove the GUI
// really builds something — not just that its validation guards work.
func TestEditor_StartBuildProducesRealOutput(t *testing.T) {
	e := newTestEditor(t)

	dir := t.TempDir()
	binPath := filepath.Join(dir, "testapp")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	e.proj.BaseDir = dir
	e.proj.Identity = packproject.Identity{Name: "Test App", ID: "com.example.testapp", Version: "1.0.0"}
	e.proj.Binaries = []packproject.BinaryEntry{{OS: "linux", Arch: "amd64", Path: binPath}}
	e.proj.Targets = []string{packproject.TargetDEB}
	e.proj.Output.Dir = filepath.Join(dir, "out")

	e.startBuild()
	if !e.build.IsRunning() {
		t.Fatal("startBuild should have started a run for a valid deb-only project")
	}

	deadline := time.Now().Add(5 * time.Second)
	for e.build.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if e.build.IsRunning() {
		t.Fatal("build did not finish within 5s")
	}

	matches, err := filepath.Glob(filepath.Join(dir, "out", "*.deb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one .deb in the output dir, got %v", matches)
	}
}

// TestEditor_StartBuildButtonColorTransitions drives the same real build
// path as TestEditor_StartBuildProducesRealOutput, but asserts the Start
// button's traffic-light Importance at each stage: green before anything
// has run, orange the moment a build starts, and back to green once a
// build with no failures completes.
func TestEditor_StartBuildButtonColorTransitions(t *testing.T) {
	e := newTestEditor(t)

	if e.startBtn.Importance != widget.SuccessImportance {
		t.Errorf("Start button Importance = %v before any build, want SuccessImportance (green, ready)", e.startBtn.Importance)
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "testapp")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	e.proj.BaseDir = dir
	e.proj.Identity = packproject.Identity{Name: "Test App", ID: "com.example.testapp", Version: "1.0.0"}
	e.proj.Binaries = []packproject.BinaryEntry{{OS: "linux", Arch: "amd64", Path: binPath}}
	e.proj.Targets = []string{packproject.TargetDEB}
	e.proj.Output.Dir = filepath.Join(dir, "out")

	e.startBuild()
	if e.startBtn.Importance != widget.WarningImportance {
		t.Errorf("Start button Importance = %v while building, want WarningImportance (orange, running)", e.startBtn.Importance)
	}

	deadline := time.Now().Add(5 * time.Second)
	for e.build.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if e.build.IsRunning() {
		t.Fatal("build did not finish within 5s")
	}

	if e.startBtn.Importance != widget.SuccessImportance {
		t.Errorf("Start button Importance = %v after a successful build, want SuccessImportance (green)", e.startBtn.Importance)
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
