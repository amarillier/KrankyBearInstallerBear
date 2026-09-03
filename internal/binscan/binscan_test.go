package binscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		wantOS   string
		wantArch string
		wantOK   bool
	}{
		{"myapp.exe", "windows", "amd64", true},
		{"myapp-arm64.exe", "windows", "arm64", true},
		{"krankybear-installerbear-macos-amd64", "darwin", "amd64", true},
		{"krankybear-installerbear-macos-arm64", "darwin", "arm64", true},
		{"myapp-darwin-amd64", "darwin", "amd64", true},
		{"myapp_linux_arm64", "linux", "arm64", true},
		{"myapp_linux_aarch64", "linux", "arm64", true},
		{"myapp_linux_x86_64", "linux", "amd64", true},
		{"myapp-linux", "linux", "amd64", true},
		{"README.txt", "", "", false},
		{"myapp.dll", "", "", false},
		{"myapp", "", "", false},
	}

	for _, c := range cases {
		gotOS, gotArch, gotOK := classify(c.name)
		if gotOK != c.wantOK || gotOS != c.wantOS || gotArch != c.wantArch {
			t.Errorf("classify(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.name, gotOS, gotArch, gotOK, c.wantOS, c.wantArch, c.wantOK)
		}
	}
}

func TestScanDir(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"myapp.exe",
		"myapp-linux-amd64",
		"myapp-macos-arm64",
		"README.txt",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	guesses, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(guesses) != 3 {
		t.Fatalf("got %d guesses, want 3: %+v", len(guesses), guesses)
	}
}
