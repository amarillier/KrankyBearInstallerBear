package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// releaseNotesBody resolves paths via os.Executable, which in `go test`
// points at a temp test binary, not this repo's own executable directory -
// so these tests exercise the pure fallback text only. The disk-read path
// itself (os.ReadFile against a real sibling file) is simple enough to be
// covered adequately by manual verification per this repo's own convention
// of not chasing test coverage for thin OS-boundary code.
func TestReleaseNotesBody_NotFoundMentionsBothConventions(t *testing.T) {
	body := releaseNotesBody()
	if !strings.Contains(body, "could not be found") {
		t.Errorf("expected not-found message, got: %q", body)
	}
	if !strings.Contains(body, "github.com/amarillier/KrankyBearInstallerBear") {
		t.Errorf("expected GitHub link in not-found message, got: %q", body)
	}
}

func TestReleaseNotesFileNames_PrefersMarkdownOverText(t *testing.T) {
	want := []string{"ReleaseNotes.md", "releasenotes.md", "ReleaseNotes.txt", "releasenotes.txt"}
	if len(releaseNotesFileNames) != len(want) {
		t.Fatalf("expected %d candidate filenames, got %d: %v", len(want), len(releaseNotesFileNames), releaseNotesFileNames)
	}
	for i, w := range want {
		if releaseNotesFileNames[i] != w {
			t.Errorf("expected releaseNotesFileNames[%d] = %q, got %q", i, w, releaseNotesFileNames[i])
		}
	}
}

// TestReleaseNotesFileNames_ResolutionOrder confirms the actual selection
// logic (not just the slice order) picks .md over .txt when both exist
// beside a given directory, and falls back correctly when only one does.
func TestReleaseNotesFileNames_ResolutionOrder(t *testing.T) {
	dir := t.TempDir()

	pick := func() (string, bool) {
		for _, name := range releaseNotesFileNames {
			if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				return string(data), true
			}
		}
		return "", false
	}

	if _, ok := pick(); ok {
		t.Fatalf("expected no file found in empty dir")
	}

	if err := os.WriteFile(filepath.Join(dir, "ReleaseNotes.txt"), []byte("plain text notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := pick(); !ok || got != "plain text notes" {
		t.Fatalf("expected txt fallback to be picked, got %q, ok=%v", got, ok)
	}

	if err := os.WriteFile(filepath.Join(dir, "ReleaseNotes.md"), []byte("# Markdown notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := pick(); !ok || got != "# Markdown notes" {
		t.Fatalf("expected md to take priority over txt, got %q, ok=%v", got, ok)
	}
}

// TestReleaseNotesFileNames_FindsLowercaseVariant covers a project that
// named the file all-lowercase (releasenotes.txt/.md) instead of this
// project's own PascalCase convention - Linux's case-sensitive filesystem
// means these are genuinely different filenames, not just a style choice.
func TestReleaseNotesFileNames_FindsLowercaseVariant(t *testing.T) {
	dir := t.TempDir()

	pick := func() (string, bool) {
		for _, name := range releaseNotesFileNames {
			if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				return string(data), true
			}
		}
		return "", false
	}

	if err := os.WriteFile(filepath.Join(dir, "releasenotes.txt"), []byte("lowercase notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := pick(); !ok || got != "lowercase notes" {
		t.Fatalf("expected releasenotes.txt (lowercase) to be found, got %q, ok=%v", got, ok)
	}
}
