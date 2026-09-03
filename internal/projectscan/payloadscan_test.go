package projectscan

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

func TestScanPayloadCandidatesBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ReleaseNotes.txt"), "x")
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}

	proj := &packproject.Project{BaseDir: dir}
	got := ScanPayloadCandidates(dir, proj)

	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), got)
	}
	byDest := make(map[string]PayloadCandidate)
	for _, c := range got {
		byDest[c.Dest] = c
	}
	if c := byDest["ReleaseNotes.txt"]; c.Recursive || c.Source != "ReleaseNotes.txt" {
		t.Errorf("ReleaseNotes.txt candidate = %+v", c)
	}
	if c := byDest["assets"]; !c.Recursive || c.Source != "assets" {
		t.Errorf("assets candidate = %+v", c)
	}
}

func TestScanPayloadCandidatesSkipsCoveredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "LICENSE"), "x")
	writeFile(t, filepath.Join(dir, "extra.txt"), "x")

	proj := &packproject.Project{
		BaseDir: dir,
		Identity: packproject.Identity{
			LicenseFile: "LICENSE",
		},
	}
	got := ScanPayloadCandidates(dir, proj)

	if len(got) != 1 || got[0].Dest != "extra.txt" {
		t.Fatalf("got %+v, want only extra.txt", got)
	}
}

func TestScanPayloadCandidatesSkipsExistingPayload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "already-added.txt"), "x")
	writeFile(t, filepath.Join(dir, "new.txt"), "x")

	proj := &packproject.Project{
		BaseDir: dir,
		Payload: []packproject.PayloadEntry{
			{Source: "already-added.txt", Dest: "already-added.txt"},
		},
	}
	got := ScanPayloadCandidates(dir, proj)

	if len(got) != 1 || got[0].Dest != "new.txt" {
		t.Fatalf("got %+v, want only new.txt", got)
	}
}
