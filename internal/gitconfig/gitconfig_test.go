package gitconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolateHome points os.UserHomeDir() at an empty temp dir for the duration
// of the test, so the real machine's ~/.gitconfig can never leak into a
// test's expectations about the global fallback.
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestReadLocalConfig(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), `
[core]
	repositoryformatversion = 0
[remote "origin"]
	url = git@github.com:amarillier/KrankyBearInstallerBear.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[user]
	name = Allan Marillier
	email = allan@example.com
`)

	info := Read(dir)
	if info.RemoteOriginURL != "git@github.com:amarillier/KrankyBearInstallerBear.git" {
		t.Errorf("RemoteOriginURL = %q", info.RemoteOriginURL)
	}
	if info.UserName != "Allan Marillier" {
		t.Errorf("UserName = %q", info.UserName)
	}
	if info.UserEmail != "allan@example.com" {
		t.Errorf("UserEmail = %q", info.UserEmail)
	}
}

func TestReadNotARepo(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	info := Read(dir)
	if info != (Info{}) {
		t.Errorf("expected zero Info for non-repo dir, got %+v", info)
	}
}

func TestReadNoRemote(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), `
[user]
	name = Someone
`)
	info := Read(dir)
	if info.RemoteOriginURL != "" {
		t.Errorf("expected no remote, got %q", info.RemoteOriginURL)
	}
	if info.UserName != "Someone" {
		t.Errorf("UserName = %q", info.UserName)
	}
}

func TestReadFallsBackToGlobalUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".gitconfig"), `
[user]
	name = Global Name
	email = global@example.com
`)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), `
[remote "origin"]
	url = https://github.com/owner/repo.git
`)

	info := Read(dir)
	if info.UserName != "Global Name" {
		t.Errorf("UserName = %q, want fallback from global config", info.UserName)
	}
	if info.UserEmail != "global@example.com" {
		t.Errorf("UserEmail = %q, want fallback from global config", info.UserEmail)
	}
	if info.RemoteOriginURL != "https://github.com/owner/repo.git" {
		t.Errorf("RemoteOriginURL = %q", info.RemoteOriginURL)
	}
}

func TestReadLocalUserWinsOverGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".gitconfig"), `
[user]
	name = Global Name
`)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), `
[user]
	name = Local Name
`)

	info := Read(dir)
	if info.UserName != "Local Name" {
		t.Errorf("UserName = %q, want repo-local value to win", info.UserName)
	}
}
