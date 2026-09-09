package winexe

import (
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

func TestExcludeFilteredFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icon.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "nested.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "excluded.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := packproject.PayloadEntry{Excludes: []string{"*.tmp"}}
	got, err := excludeFilteredFiles(dir, `assets\images`, entry)
	if err != nil {
		t.Fatalf("excludeFilteredFiles: %v", err)
	}

	byName := make(map[string]nsiFileEntry)
	for _, f := range got {
		byName[filepath.Base(f.Source)] = f
	}

	if _, ok := byName["excluded.tmp"]; ok {
		t.Error("excluded.tmp should have been skipped by Excludes")
	}
	top, ok := byName["icon.png"]
	if !ok {
		t.Fatal("expected icon.png in the result")
	}
	if top.DestDir != `assets\images` {
		t.Errorf("icon.png DestDir = %q, want %q", top.DestDir, `assets\images`)
	}
	nested, ok := byName["nested.png"]
	if !ok {
		t.Fatal("expected nested.png in the result")
	}
	if nested.DestDir != `assets\images\sub` {
		t.Errorf("nested.png DestDir = %q, want %q", nested.DestDir, `assets\images\sub`)
	}
}
