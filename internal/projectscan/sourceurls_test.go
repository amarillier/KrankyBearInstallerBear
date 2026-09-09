package projectscan

import (
	"path/filepath"
	"testing"
)

func TestFindSourceURLGitHubNormalizesToRepoRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "about.go"), `package main
const licenseURL = "https://github.com/amarillier/MyApp/blob/main/LICENSE"
`)
	writeFile(t, filepath.Join(dir, "update.go"), `package main
const updateRepoDL = "https://github.com/amarillier/MyApp/releases/latest"
`)

	got := findSourceURL(dir)
	want := "https://github.com/amarillier/MyApp"
	if got != want {
		t.Errorf("findSourceURL() = %q, want %q", got, want)
	}
}

func TestFindSourceURLNonGitHubPrefersShortestPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "help.go"), `package main
const (
	homepage = "https://gitlab.example.com/team/myapp"
	docsLink = "https://gitlab.example.com/team/myapp/-/blob/main/README.md"
)
`)

	got := findSourceURL(dir)
	want := "https://gitlab.example.com/team/myapp"
	if got != want {
		t.Errorf("findSourceURL() = %q, want %q (the shorter, plainer link)", got, want)
	}
}

func TestFindSourceURLNoFiles(t *testing.T) {
	dir := t.TempDir()
	if got := findSourceURL(dir); got != "" {
		t.Errorf("findSourceURL() = %q, want empty for a dir with none of the source files", got)
	}
}

func TestScanPrefersGitRemoteOverSourceURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), `
[remote "origin"]
	url = https://github.com/realowner/RealRepo.git
`)
	writeFile(t, filepath.Join(dir, "help.go"), `package main
const url = "https://github.com/stale-template-owner/OldTemplateName"
`)

	got := Scan(dir)
	want := "https://github.com/realowner/RealRepo"
	if got.URL != want {
		t.Errorf("Scan().URL = %q, want %q (git remote should win over a source-scanned URL)", got.URL, want)
	}
}

func TestScanFallsBackToSourceURLWithNoGitRemote(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "about.go"), `package main
const url = "https://github.com/amarillier/MyApp"
`)

	got := Scan(dir)
	want := "https://github.com/amarillier/MyApp"
	if got.URL != want {
		t.Errorf("Scan().URL = %q, want %q", got.URL, want)
	}
}
