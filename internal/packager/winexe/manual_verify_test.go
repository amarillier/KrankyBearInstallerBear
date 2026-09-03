//go:build manual_verify

// This file is excluded from normal `go test ./...` runs (build tag
// manual_verify) since it requires a real, working makensis on PATH — not
// something every dev machine or CI runner has. Run it explicitly on a host
// with makensis to verify the full Build() pipeline end-to-end, not just
// template rendering:
//
//	go test -tags manual_verify -run TestManualRealBuild ./internal/packager/winexe/... -v
package winexe

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

func TestManualRealBuild(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "testapp.exe")
	if err := os.WriteFile(binPath, []byte("fake-exe-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(licensePath, []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj := &packproject.Project{
		BaseDir: dir,
		Identity: packproject.Identity{
			Name:        "Test App",
			ID:          "com.example.testapp",
			Version:     "1.0.0",
			Publisher:   "Someone",
			LicenseFile: "LICENSE",
		},
		Binaries: []packproject.BinaryEntry{{OS: "windows", Arch: "amd64", Path: binPath}},
		Windows:  packproject.WindowsOptions{ExeName: "TestApp.exe"},
	}
	proj.Defaults()
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := New().Build(context.Background(), proj, packager.BuildOptions{}, func(_ packager.Target, line string) {
		t.Logf("makensis: %s", line)
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	t.Logf("built %s (%d bytes)", outPath, info.Size())
}
