package debrpm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

func sampleProject(t *testing.T, dir string) *packproject.Project {
	t.Helper()

	binPath := filepath.Join(dir, "testapp-linux-amd64")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(licensePath, []byte("MIT"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &packproject.Project{
		BaseDir: dir,
		Identity: packproject.Identity{
			Name:        "Test App",
			ID:          "com.example.testapp",
			Version:     "1.2.3",
			Publisher:   "Someone",
			LicenseFile: "LICENSE",
		},
		Binaries: []packproject.BinaryEntry{{OS: "linux", Arch: "amd64", Path: binPath}},
		Hooks:    packproject.Hooks{PostUninstall: "rm -rf /opt/test-app || true"},
	}
	p.Defaults()
	return p
}

func TestDebBuild_ProducesReadableArchive(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	outDir := filepath.Join(dir, "out")
	proj.Output.Dir = outDir

	deb := NewDeb()
	if err := deb.HostSupported(); err != nil {
		t.Fatalf("HostSupported: %v", err)
	}
	if err := deb.Preflight(); err != nil {
		t.Fatalf("Preflight: %v", err)
	}

	outPath, err := deb.Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.HasSuffix(outPath, ".deb") {
		t.Fatalf("expected a .deb file, got %s", outPath)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output not written: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// A .deb is an ar archive; ar magic + the fixed "debian-binary" member
	// name confirm nfpm wrote a real deb, not garbage.
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatalf("output does not look like an ar archive (deb)")
	}
	if !bytes.Contains(data, []byte("debian-binary")) {
		t.Fatalf("expected a debian-binary member in the .deb")
	}
}

func TestRPMBuild_ProducesReadableArchive(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")

	rpm := NewRPM()
	outPath, err := rpm.Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.HasSuffix(outPath, ".rpm") {
		t.Fatalf("expected a .rpm file, got %s", outPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// RPM lead magic: 0xED 0xAB 0xEE 0xDB.
	want := []byte{0xed, 0xab, 0xee, 0xdb}
	if len(data) < 4 || !bytes.Equal(data[:4], want) {
		t.Fatalf("output does not start with the RPM lead magic bytes")
	}
}

// TestBuild_RelativeBinaryPathResolvesAgainstBaseDir guards against a real
// bug caught only by a manual end-to-end smoke test: BinaryEntry.Path (like
// every other config path) must resolve against proj.BaseDir, not the
// process's current working directory. A project loaded via
// packproject.Load and then run from a different cwd must still find its
// binary.
func TestBuild_RelativeBinaryPathResolvesAgainstBaseDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "testapp"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	proj := &packproject.Project{
		BaseDir:  dir, // relative Binaries[].Path must resolve against this, not os.Getwd()
		Identity: packproject.Identity{Name: "TestApp", ID: "com.example.testapp", Version: "1.0.0"},
		Binaries: []packproject.BinaryEntry{{OS: "linux", Arch: "amd64", Path: "bin/testapp"}},
	}
	proj.Defaults()
	proj.Output.Dir = filepath.Join(dir, "out")

	if _, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil); err != nil {
		t.Fatalf("Build with a relative binary path failed: %v", err)
	}
}

// TestBuildInfo_License confirms Identity.License (a short identifier like
// "GPL v3", distinct from Identity.LicenseFile) reaches nfpm's own License
// field, which is package metadata nfpm already supports but this backend
// wasn't setting at all before.
func TestBuildInfo_License(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Identity.License = "GPL v3"

	info, cleanup, err := buildInfo(proj, "amd64", proj.Binaries[0])
	if err != nil {
		t.Fatalf("buildInfo: %v", err)
	}
	defer cleanup()

	if info.License != "GPL v3" {
		t.Errorf("License = %q, want GPL v3", info.License)
	}
}

func TestBuild_MissingBinaryForArchFailsWithClearError(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Binaries = nil // no linux/amd64 binary registered

	_, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "no linux/amd64 binary registered") {
		t.Fatalf("expected a clear missing-binary error, got %v", err)
	}
}

// TestDebBuild_ContainsPostRemoveHook confirms the inline Hooks.PostUninstall
// text actually lands in the package as a real maintainer script, not just
// gets silently dropped — this is the one place nfpm's file-path-based
// Scripts API and our inline-text schema could quietly diverge.
func TestDebBuild_ContainsPostRemoveHook(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debControlArchiveContains(outPath, "postrm", "rm -rf /opt/test-app")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("postrm script in %s does not contain the configured post_uninstall hook", outPath)
	}
}

// debControlArchiveContains extracts control.tar.gz from a .deb (an ar
// archive) and checks whether the named member's content contains want.
func debControlArchiveContains(debPath, member, want string) (bool, error) {
	data, err := os.ReadFile(debPath)
	if err != nil {
		return false, err
	}

	// Minimal ar reader: skip the "!<arch>\n" magic, then walk fixed 60-byte
	// headers (name[16] mtime[12] uid[6] gid[6] mode[8] size[10] magic[2]).
	buf := data[8:]
	for len(buf) >= 60 {
		name := strings.TrimRight(string(buf[0:16]), " ")
		sizeStr := strings.TrimSpace(string(buf[48:58]))
		var size int
		for _, c := range sizeStr {
			size = size*10 + int(c-'0')
		}
		body := buf[60 : 60+size]
		if strings.HasPrefix(name, "control.tar") {
			return tarGzContains(body, member, want)
		}
		if size%2 == 1 {
			size++
		}
		buf = buf[60+size:]
	}
	return false, nil
}

func tarGzContains(gzData []byte, member, want string) (bool, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return false, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return false, nil //nolint:nilerr // end of archive without a match is "not found", not an error
		}
		if strings.TrimPrefix(hdr.Name, "./") == member {
			content := new(bytes.Buffer)
			if _, err := content.ReadFrom(tr); err != nil {
				return false, err
			}
			return strings.Contains(content.String(), want), nil
		}
	}
}
