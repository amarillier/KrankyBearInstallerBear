package macpkg

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// TestBuild_InstallExperienceNotApplicableNotesOnMac confirms that setting
// InstallExperience.LaunchAfterInstall/DesktopShortcut - a Windows-only
// feature, see packproject.InstallExperience's own doc comment - doesn't
// silently do nothing on macOS: it surfaces a clear progress note
// explaining why, rather than leaving someone wondering why a setting
// they turned on had no visible effect.
func TestBuild_InstallExperienceNotApplicableNotesOnMac(t *testing.T) {
	dir := t.TempDir()
	proj := sampleProject(t, dir)
	proj.Output.Dir = filepath.Join(dir, "out")
	proj.InstallExperience = packproject.InstallExperience{LaunchAfterInstall: true, DesktopShortcut: true, AutostartAtLogin: true}
	proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp"}}

	var lines []string
	_, err := macpkgPackager{}.Build(context.Background(), proj, packager.BuildOptions{}, func(_ packager.Target, line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{"launch_after_install has no effect", "desktop_shortcut has no effect", "autostart_at_login has no effect", "file_associations has no effect"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a progress note containing %q, got:\n%s", want, joined)
		}
	}
}
