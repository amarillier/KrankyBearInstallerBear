package packager

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestStaleInstallers_FindsOlderVersionsAcrossRealNamingConventions mirrors
// the actual filenames this project's own backends produce (deb, rpm,
// macpkg, winexe) for two different versions in the same output dir — the
// exact "upgraded from 0.2.0 to 0.3.0, leftover 0.2.0 installers" scenario
// this feature exists for.
func TestStaleInstallers_FindsOlderVersionsAcrossRealNamingConventions(t *testing.T) {
	dir := t.TempDir()

	current := map[string]string{
		"deb":    "krankybear-installerbear_0.3.0_amd64.deb",
		"rpm":    "krankybear-installerbear-0.3.0-1.x86_64.rpm",
		"macpkg": "krankybear-installerbear_0.3.0_amd64.pkg",
		"winexe": "KrankyBearInstallerBearSetup_0.3.0_amd64.exe",
	}
	older := map[string]string{
		"deb":    "krankybear-installerbear_0.2.0_amd64.deb",
		"rpm":    "krankybear-installerbear-0.2.0-1.x86_64.rpm",
		"macpkg": "krankybear-installerbear_0.2.0_amd64.pkg",
		"winexe": "KrankyBearInstallerBearSetup_0.2.0_amd64.exe",
	}
	// An unrelated file that happens to share the directory shouldn't ever
	// be touched.
	touch(t, filepath.Join(dir, "ReadMe.txt"))

	var results []BuildResult
	for key, name := range current {
		full := filepath.Join(dir, name)
		touch(t, full)
		touch(t, filepath.Join(dir, older[key]))
		results = append(results, BuildResult{Target: Target(key), OutputPath: full})
	}

	got := StaleInstallers(results, "0.3.0")

	if len(got) != len(older) {
		t.Fatalf("got %d stale files, want %d: %v", len(got), len(older), got)
	}
	gotSet := make(map[string]bool, len(got))
	for _, p := range got {
		gotSet[p] = true
	}
	for _, name := range older {
		want := filepath.Join(dir, name)
		if !gotSet[want] {
			t.Errorf("expected %s to be reported stale, got %v", want, got)
		}
	}
	for _, name := range current {
		if gotSet[filepath.Join(dir, name)] {
			t.Errorf("the just-built %s must never be reported as stale", name)
		}
	}
	if gotSet[filepath.Join(dir, "ReadMe.txt")] {
		t.Error("an unrelated file must never be reported as stale")
	}
}

func TestStaleInstallers_IgnoresFailedAndSkippedResults(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "app_0.2.0.deb"))

	results := []BuildResult{
		{Target: TargetDEB, OutputPath: filepath.Join(dir, "app_0.3.0.deb"), Err: errors.New("build failed")},
		{Target: TargetRPM, OutputPath: filepath.Join(dir, "app_0.3.0.rpm"), Skipped: true},
	}
	if got := StaleInstallers(results, "0.3.0"); len(got) != 0 {
		t.Errorf("expected no stale files from failed/skipped results, got %v", got)
	}
}

func TestStaleInstallers_NoVersionInFilenameIsSkipped(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "app.deb"))
	results := []BuildResult{{Target: TargetDEB, OutputPath: filepath.Join(dir, "app.deb")}}

	if got := StaleInstallers(results, "0.3.0"); len(got) != 0 {
		t.Errorf("expected no stale files when the output filename doesn't contain the version, got %v", got)
	}
}
