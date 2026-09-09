package main

import "testing"

// TestScanSummaryDistinguishesAlreadySetFromNoMatch is a regression test:
// scanSummary used to report a slot as "no match" whenever nothing was
// newly written to it, which was indistinguishable from a real "no match"
// even when a file was actually found but the slot already had a value
// (e.g. New Project's own bin/ auto-scan already filled it — see
// projectio.go's applyScannedBinaries) — misleadingly implying the rescan
// itself had failed.
func TestScanSummaryDistinguishesAlreadySetFromNoMatch(t *testing.T) {
	fields := []binaryField{
		{os: "windows", arch: "amd64"},
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "linux", arch: "amd64"},
	}
	filled := map[string]string{
		binaryFieldLabel("windows", "amd64"): "bin/app.exe",
	}
	alreadySet := map[string]bool{
		binaryFieldLabel("darwin", "amd64"): true,
	}
	ambiguous := map[string]bool{
		binaryFieldLabel("darwin", "arm64"): true,
	}

	got := scanSummary(fields, filled, alreadySet, ambiguous)
	want := binaryFieldLabel("windows", "amd64") + ": bin/app.exe\n" +
		binaryFieldLabel("darwin", "amd64") + ": already set, kept\n" +
		binaryFieldLabel("darwin", "arm64") + ": multiple matches, skipped\n" +
		binaryFieldLabel("linux", "amd64") + ": no match"
	if got != want {
		t.Errorf("scanSummary() = %q, want %q", got, want)
	}
}
