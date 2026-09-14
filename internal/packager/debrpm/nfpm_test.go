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

	"github.com/goreleaser/nfpm/v2/files"

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

// TestBuildInfo_OwnsInstallRootDirectory is a regression test for a real
// bug found via `rpm -e` on a real machine, one level up from the
// excludeFilteredTree directory-ownership fix: even with that fix in
// place, the install root itself (e.g. /opt/TestApp) was left behind,
// empty, after uninstall - it's never the explicit destination of
// anything, only ever the parent of the binary/License.txt/Payload
// entries, so on its own it only ever gets registered as an unowned
// implicit directory, which rpm's builder skips. buildInfo now adds an
// explicit TypeDir entry for it directly.
func TestBuildInfo_OwnsInstallRootDirectory(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)

	info, cleanup, err := buildInfo(proj, "amd64", proj.Binaries[0])
	if err != nil {
		t.Fatalf("buildInfo: %v", err)
	}
	defer cleanup()

	var found bool
	for _, c := range info.Overridables.Contents {
		if c.Destination == proj.Install.Linux {
			if c.Type != files.TypeDir {
				t.Errorf("expected %q to be a TypeDir entry, got type %q", proj.Install.Linux, c.Type)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("expected an explicit directory entry for the install root %q, got %+v", proj.Install.Linux, info.Overridables.Contents)
	}
}

// TestBuild_InstallExperienceNotApplicableNotesOnLinux confirms that
// setting InstallExperience.LaunchAfterInstall/DesktopShortcut - a
// Windows-only feature, see packproject.InstallExperience's own doc
// comment - doesn't silently do nothing on Linux: it surfaces a clear
// progress note explaining why, rather than leaving someone wondering why
// a setting they turned on had no visible effect.
func TestBuild_InstallExperienceNotApplicableNotesOnLinux(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.InstallExperience = packproject.InstallExperience{LaunchAfterInstall: true, DesktopShortcut: true, AutostartAtLogin: true}

	var lines []string
	_, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, func(_ packager.Target, line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{"launch_after_install has no effect", "desktop_shortcut has no effect", "autostart_at_login has no effect"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a progress note containing %q, got:\n%s", want, joined)
		}
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

// TestDebBuild_ContainsDesktopEntry confirms a real app-menu .desktop file
// lands in the built .deb's actual file payload (data.tar.gz, not just the
// maintainer scripts) at the standard freedesktop.org location, with the
// expected Name= - this is always installed regardless of
// InstallExperience.DesktopShortcut (see desktopEntry's own comment).
func TestDebBuild_ContainsDesktopEntry(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.Linux.DesktopComment = "A test application"
	proj.Linux.DesktopCategories = "Utility"

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debDataArchiveContains(outPath, "usr/share/applications/test-app.desktop", "Name=Test App")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf(".desktop entry missing or wrong content in %s", outPath)
	}

	foundCategories, err := debDataArchiveContains(outPath, "usr/share/applications/test-app.desktop", "Categories=Utility;")
	if err != nil {
		t.Fatal(err)
	}
	if !foundCategories {
		t.Errorf("expected Categories=Utility; (semicolon-terminated) in the .desktop entry")
	}
}

// TestDebBuild_ContainsIconWhenPNGSet confirms Identity.Icons.PNG lands in
// the hicolor icon theme directory the .desktop entry's Icon= line
// references by name.
func TestDebBuild_ContainsIconWhenPNGSet(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.Identity.Icons.PNG = filepath.Join(dir, "icon.png")
	if err := os.WriteFile(proj.Identity.Icons.PNG, []byte("fake-png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debDataArchiveContains(outPath, "usr/share/icons/hicolor/256x256/apps/test-app.png", "")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("expected the PNG icon installed into the hicolor icon theme in %s", outPath)
	}
}

// TestDebBuild_NoIconFileWhenPNGUnset confirms nothing gets installed into
// the icon theme directory when Identity.Icons.PNG is blank - there's
// nothing to copy, and the .desktop entry itself already omits Icon= in
// that case (see TestDesktopEntry_OmitsIconWhenNoPNGSet).
func TestDebBuild_NoIconFileWhenPNGUnset(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debDataArchiveContains(outPath, "usr/share/icons/hicolor/256x256/apps/test-app.png", "")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Errorf("expected no icon file in %s when Icons.PNG is unset", outPath)
	}
}

// TestDebBuild_PostInstallRefreshesDesktopCaches confirms the desktop-
// database/icon-cache refresh commands land in a real postinst maintainer
// script (nfpm's PostInstall slot, previously unused by any Hooks field).
func TestDebBuild_PostInstallRefreshesDesktopCaches(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.Identity.Icons.PNG = filepath.Join(dir, "icon.png")
	if err := os.WriteFile(proj.Identity.Icons.PNG, []byte("fake-png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, want := range []string{"update-desktop-database", "gtk-update-icon-cache"} {
		found, err := debControlArchiveContains(outPath, "postinst", want)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Errorf("expected postinst to contain %q", want)
		}
	}
}

// TestDebBuild_PostRemoveMergesUserHookWithDesktopRefresh confirms the
// project's own Hooks.PostUninstall text and the desktop/icon-cache
// refresh commands both land in the same postrm script - the refresh
// commands must be appended after the user's hook, not replace it (nfpm
// only has one PostRemove slot, and Hooks.PostUninstall already claims it).
func TestDebBuild_PostRemoveMergesUserHookWithDesktopRefresh(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir) // sampleProject already sets Hooks.PostUninstall
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, want := range []string{"rm -rf /opt/test-app", "update-desktop-database"} {
		found, err := debControlArchiveContains(outPath, "postrm", want)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Errorf("expected postrm to contain %q", want)
		}
	}
}

// TestDebBuild_FileAssociationInstallsMimePackageAndDesktopFields confirms
// a real end-to-end build: the shared-mime-info package lands in the
// actual .deb payload at the standard freedesktop.org location with the
// right content, and the .desktop entry itself gets MimeType=/%f - all
// from a real built archive, not just the desktopfile_test.go unit tests
// of the string-building logic in isolation.
func TestDebBuild_FileAssociationInstallsMimePackageAndDesktopFields(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.FileAssociations = []packproject.FileAssociation{
		{Extension: ".myp", Description: "Test App Project"},
	}

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debDataArchiveContains(outPath, "usr/share/mime/packages/test-app.xml", "application/x-test-app-myp")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("expected the shared-mime-info package installed at the standard location in %s", outPath)
	}

	foundDesktop, err := debDataArchiveContains(outPath, "usr/share/applications/test-app.desktop", "MimeType=application/x-test-app-myp;")
	if err != nil {
		t.Fatal(err)
	}
	if !foundDesktop {
		t.Errorf("expected the .desktop entry to list the association's MimeType")
	}
}

func TestDebBuild_PostInstallRefreshesMimeDatabaseWhenAssociationsSet(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp"}}

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debControlArchiveContains(outPath, "postinst", "update-mime-database")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Errorf("expected postinst to refresh the mime database when FileAssociations is set")
	}
}

func TestDebBuild_NoMimePackageWithoutFileAssociations(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")

	outPath, err := NewDeb().Build(context.Background(), proj, packager.BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	found, err := debDataArchiveContains(outPath, "usr/share/mime/packages/test-app.xml", "")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Errorf("expected no shared-mime-info package when FileAssociations is empty")
	}
}

// debControlArchiveContains extracts control.tar.gz from a .deb (an ar
// archive) and checks whether the named member's content contains want.
func debControlArchiveContains(debPath, member, want string) (bool, error) {
	return debArchiveMemberContains(debPath, "control.tar", member, want)
}

// debDataArchiveContains extracts data.tar.gz (the actual installed payload,
// as opposed to control.tar.gz's maintainer scripts) from a .deb and checks
// whether the named member's content contains want.
func debDataArchiveContains(debPath, member, want string) (bool, error) {
	return debArchiveMemberContains(debPath, "data.tar", member, want)
}

// debArchiveMemberContains extracts the ar member whose name has the given
// prefix (a .deb is itself an ar archive containing "control.tar.*" and
// "data.tar.*" members, each a gzipped tar) and checks whether the named
// file inside it contains want.
func debArchiveMemberContains(debPath, arMemberPrefix, member, want string) (bool, error) {
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
		if strings.HasPrefix(name, arMemberPrefix) {
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
