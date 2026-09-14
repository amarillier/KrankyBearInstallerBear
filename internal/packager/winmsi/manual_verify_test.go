//go:build manual_verify

// Excluded from normal `go test ./...` (build tag manual_verify) since it
// requires a real wixl on PATH — not guaranteed on every dev machine or CI
// runner. Run explicitly to verify the full Build() pipeline end-to-end:
//
//	go test -tags manual_verify -run TestManualRealBuild ./internal/packager/winmsi/... -v
package winmsi

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
	assetsDir := filepath.Join(dir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "icon.png"), []byte("fake-png"), 0o644); err != nil {
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
		Payload:  []packproject.PayloadEntry{{Source: "assets/images", Dest: "assets/images", Recursive: true}},
		Windows: packproject.WindowsOptions{
			ExeName:     "TestApp.exe",
			UpgradeGUID: "{4578B785-DB27-44FF-B3F9-2713B327BB90}",
		},
		FileAssociations: []packproject.FileAssociation{
			{Extension: ".myp", Description: "Test App Project"},
		},
	}
	proj.Defaults()
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := New().Build(context.Background(), proj, packager.BuildOptions{}, func(_ packager.Target, line string) {
		t.Logf("wixl: %s", line)
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
