package packproject

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writePayloadTestFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sourcesOf(entries []PayloadEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Source
	}
	sort.Strings(out)
	return out
}

func TestExpandedPayload_LiteralEntryPassesThroughUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "ReleaseNotes.md")

	entries := []PayloadEntry{{Source: "ReleaseNotes.md", Dest: ""}}
	out, err := ExpandedPayload(entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Source != "ReleaseNotes.md" {
		t.Errorf("expected the literal entry unchanged, got %+v", out)
	}
}

func TestExpandedPayload_GlobExpandsToEachMatch(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "a.yaml", "b.yaml", "c.txt")

	entries := []PayloadEntry{{Source: "*.yaml", Dest: ""}}
	out, err := ExpandedPayload(entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := sourcesOf(out)
	want := []string{"a.yaml", "b.yaml"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, e := range out {
		if e.Dest != "" {
			t.Errorf("expected Dest to be inherited from the original entry, got %+v", e)
		}
	}
}

func TestExpandedPayload_ExcludesFiltersGlobMatches(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "installerbear.yaml", "sample-installerbear.yaml")

	entries := []PayloadEntry{{
		Source:   "*.yaml",
		Dest:     "",
		Excludes: []string{"sample-installerbear.yaml"},
	}}
	out, err := ExpandedPayload(entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := sourcesOf(out)
	if len(got) != 1 || got[0] != "installerbear.yaml" {
		t.Errorf("expected only installerbear.yaml to survive the exclude, got %v", got)
	}
}

func TestExpandedPayload_ZeroMatchesIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	entries := []PayloadEntry{{Source: "*.nonexistent", Dest: ""}}
	out, err := ExpandedPayload(entries, dir)
	if err != nil {
		t.Fatalf("expected no error for a pattern matching nothing, got %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected zero entries, got %+v", out)
	}
}

func TestExpandedPayload_RecursiveGlobExpandsEachMatchedDirectory(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "logs/2026-01/a.log", "logs/2026-02/b.log")

	entries := []PayloadEntry{{Source: "logs/*", Dest: "logs", Recursive: true}}
	out, err := ExpandedPayload(entries, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := sourcesOf(out)
	want := []string{"logs/2026-01", "logs/2026-02"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, e := range out {
		if !e.Recursive {
			t.Errorf("expected Recursive to be inherited, got %+v", e)
		}
	}
}

func TestExpandedPayload_DoesNotMutateOriginalSlice(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "a.yaml", "b.yaml")

	entries := []PayloadEntry{{Source: "*.yaml", Dest: ""}}
	if _, err := ExpandedPayload(entries, dir); err != nil {
		t.Fatal(err)
	}
	if entries[0].Source != "*.yaml" {
		t.Errorf("expected the original entry's Source to remain the pattern, got %q", entries[0].Source)
	}
}

func TestExpandedPayload_InvalidPatternReturnsError(t *testing.T) {
	dir := t.TempDir()
	entries := []PayloadEntry{{Source: "[", Dest: ""}}
	if _, err := ExpandedPayload(entries, dir); err == nil {
		t.Error("expected an error for a malformed glob pattern")
	}
}

func TestProject_ExpandedPayload_UsesOwnBaseDir(t *testing.T) {
	dir := t.TempDir()
	writePayloadTestFiles(t, dir, "a.yaml")

	proj := &Project{BaseDir: dir, Payload: []PayloadEntry{{Source: "*.yaml"}}}
	out, err := proj.ExpandedPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Source != "a.yaml" {
		t.Errorf("got %+v", out)
	}
}
