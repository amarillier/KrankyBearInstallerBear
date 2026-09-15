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
	bySource := make(map[string]PayloadCandidate)
	for _, c := range got {
		bySource[c.Source] = c
	}
	// A plain file's Dest defaults to "" (the install root) - not its own
	// basename, which would otherwise nest it one level deeper than
	// intended (see defaultPayloadDest's own doc comment in
	// payloadtable.go for the real bug this guards against).
	if c := bySource["ReleaseNotes.txt"]; c.Recursive || c.Dest != "" {
		t.Errorf("ReleaseNotes.txt candidate = %+v, want Recursive=false Dest=\"\"", c)
	}
	// A folder's Dest still defaults to its own basename - its contents
	// land under a same-named subdirectory.
	if c := bySource["assets"]; !c.Recursive || c.Dest != "assets" {
		t.Errorf("assets candidate = %+v, want Recursive=true Dest=\"assets\"", c)
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

	if len(got) != 1 || got[0].Source != "extra.txt" {
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

	if len(got) != 1 || got[0].Source != "new.txt" {
		t.Fatalf("got %+v, want only new.txt", got)
	}
}
