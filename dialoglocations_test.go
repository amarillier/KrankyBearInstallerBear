package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestDialogStartLocation_UsesRememberedDirWhenSet(t *testing.T) {
	a := test.NewApp()
	dir := t.TempDir()
	rememberProjectDir(a, dir)

	for _, forSave := range []bool{true, false} {
		loc := dialogStartLocation(a, forSave)
		if loc == nil {
			t.Fatalf("forSave=%v: expected a non-nil location", forSave)
		}
		got := loc.Path()
		want, _ := filepath.EvalSymlinks(dir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		if gotResolved != want {
			t.Errorf("forSave=%v: location = %q, want %q", forSave, got, want)
		}
	}
}

func TestDialogStartLocation_FirstTimeDiscoveryDefaultsBesideExecutable(t *testing.T) {
	a := test.NewApp() // fresh app, nothing remembered yet
	loc := dialogStartLocation(a, false)
	if loc == nil {
		t.Fatal("expected a non-nil location")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wantDir, _ := filepath.EvalSymlinks(filepath.Dir(exe))
	gotDir, _ := filepath.EvalSymlinks(loc.Path())
	if gotDir != wantDir {
		t.Errorf("discovery location = %q, want the executable's own directory %q", gotDir, wantDir)
	}
}

func TestDialogStartLocation_FirstTimeSaveDefaultsToHome(t *testing.T) {
	a := test.NewApp() // fresh app, nothing remembered yet
	loc := dialogStartLocation(a, true)
	if loc == nil {
		t.Fatal("expected a non-nil location")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	wantDir, _ := filepath.EvalSymlinks(home)
	gotDir, _ := filepath.EvalSymlinks(loc.Path())
	if gotDir != wantDir {
		t.Errorf("save location = %q, want the home directory %q", gotDir, wantDir)
	}
}

func TestDialogStartLocation_RememberedDirOverridesBothFirstTimeDefaults(t *testing.T) {
	a := test.NewApp()
	dir := t.TempDir()
	rememberProjectDir(a, dir)

	discoveryLoc := dialogStartLocation(a, false)
	saveLoc := dialogStartLocation(a, true)
	wantDir, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(discoveryLoc.Path()); got != wantDir {
		t.Errorf("discovery location = %q, want remembered dir %q", got, wantDir)
	}
	if got, _ := filepath.EvalSymlinks(saveLoc.Path()); got != wantDir {
		t.Errorf("save location = %q, want remembered dir %q", got, wantDir)
	}
}

func TestRememberProjectDir_IgnoresEmptyDir(t *testing.T) {
	a := test.NewApp()
	rememberProjectDir(a, "")
	if got := a.Preferences().String(prefLastProjectDir); got != "" {
		t.Errorf("expected the preference to stay unset, got %q", got)
	}
}
