package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"installerbear/internal/packproject"
)

func TestSuggestBundleID(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		publisher string
		vendor    string
		appName   string
		want      string
	}{
		{
			"github URL takes precedence, matching this project's own real convention",
			"https://github.com/amarillier/KrankyBearInstallerBear", "someone@example.com", "",
			"KrankyBear InstallerBear", "com.github.amarillier.krankybear-installerbear",
		},
		{"non-github URL falls through to publisher", "https://example.org/amarillier", "someone@example.com", "", "Test App", "com.example.test-app"},
		{"email publisher reverses domain", "", "someone@example.com", "", "Test App", "com.example.test-app"},
		{"email vendor used when publisher blank", "", "", "team@my-company.co.uk", "Test App", "uk.co.my-company.test-app"},
		{"plain publisher name slugged under com.", "", "Allan Marillier", "", "Test App", "com.allan-marillier.test-app"},
		{"blank publisher and vendor falls back to com.example", "", "", "", "Test App", "com.example.test-app"},
		{"blank app name falls back to app", "", "someone@example.com", "", "", "com.example.app"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			proj := &packproject.Project{Identity: packproject.Identity{
				Name:      c.appName,
				URL:       c.url,
				Publisher: c.publisher,
				Vendor:    c.vendor,
			}}
			if got := suggestBundleID(proj); got != c.want {
				t.Errorf("suggestBundleID() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestEditor_PNGIconThumbnailShowsForValidPathHidesOtherwise covers
// refreshPNGIconThumbnail: a real, existing file shows the preview; a blank
// or nonexistent path hides it again rather than showing Fyne's own
// broken-image placeholder. Only tests the file-existence gate here —
// the thumbnail's own image decoding is Fyne/canvas.Image's job, not
// something this project's code does.
func TestEditor_PNGIconThumbnailShowsForValidPathHidesOtherwise(t *testing.T) {
	e := newTestEditor(t)
	dir := t.TempDir()
	e.proj.BaseDir = dir

	pngPath := filepath.Join(dir, "icon.png")
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	e.pngEntry.SetText("icon.png") // relative, resolved against proj.BaseDir like every other project path
	if e.pngIconThumbnail.Hidden {
		t.Error("expected the thumbnail to show for a real, existing file")
	}
	if e.pngIconThumbnail.File != pngPath {
		t.Errorf("thumbnail File = %q, want the BaseDir-resolved path %q", e.pngIconThumbnail.File, pngPath)
	}

	e.pngEntry.SetText("does-not-exist.png")
	if !e.pngIconThumbnail.Hidden {
		t.Error("expected the thumbnail to hide for a nonexistent path")
	}

	e.pngEntry.SetText("")
	if !e.pngIconThumbnail.Hidden {
		t.Error("expected the thumbnail to hide for a blank path")
	}
}

func TestEditor_InstallExperienceChecksWriteIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.launchAfterInstallCheck.SetChecked(true)
	e.desktopShortcutCheck.SetChecked(true)
	if !e.proj.InstallExperience.LaunchAfterInstall {
		t.Error("expected checking Launch after install to set InstallExperience.LaunchAfterInstall")
	}
	if !e.proj.InstallExperience.DesktopShortcut {
		t.Error("expected checking Desktop shortcut to set InstallExperience.DesktopShortcut")
	}

	e.launchAfterInstallCheck.SetChecked(false)
	e.desktopShortcutCheck.SetChecked(false)
	if e.proj.InstallExperience.LaunchAfterInstall || e.proj.InstallExperience.DesktopShortcut {
		t.Error("expected unchecking both to clear InstallExperience back to false")
	}
}

func TestEditor_AutostartAtLoginCheckWritesIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.autostartAtLoginCheck.SetChecked(true)
	if !e.proj.InstallExperience.AutostartAtLogin {
		t.Error("expected checking Run at startup to set InstallExperience.AutostartAtLogin")
	}

	e.autostartAtLoginCheck.SetChecked(false)
	if e.proj.InstallExperience.AutostartAtLogin {
		t.Error("expected unchecking Run at startup to clear InstallExperience.AutostartAtLogin")
	}
}

// TestEditor_RefreshIdentityTabRestoresInstallExperienceChecks is a
// regression-shaped test for the same class of bug New/Open Project has
// hit before elsewhere on this tab (see the PNG icon thumbnail's own
// tests): refreshIdentityTab must push a loaded project's
// InstallExperience booleans into the checkboxes, not just leave them at
// whatever an earlier project left behind.
func TestEditor_RefreshIdentityTabRestoresInstallExperienceChecks(t *testing.T) {
	e := newTestEditor(t)
	e.proj.InstallExperience = packproject.InstallExperience{LaunchAfterInstall: true, DesktopShortcut: true}

	e.refreshIdentityTab()

	if !e.launchAfterInstallCheck.Checked {
		t.Error("expected refreshIdentityTab to check Launch after install from the loaded project")
	}
	if !e.desktopShortcutCheck.Checked {
		t.Error("expected refreshIdentityTab to check Desktop shortcut from the loaded project")
	}
}

func TestEditor_RefreshIdentityTabRestoresAutostartAtLoginCheck(t *testing.T) {
	e := newTestEditor(t)
	e.proj.InstallExperience = packproject.InstallExperience{AutostartAtLogin: true}

	e.refreshIdentityTab()

	if !e.autostartAtLoginCheck.Checked {
		t.Error("expected refreshIdentityTab to check Run at startup from the loaded project")
	}
}

func TestEditor_InstallScopeSelectWritesIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.installScopeSelect.SetSelected(installScopeLabelCurrentUser)
	if e.proj.Windows.InstallScope != packproject.InstallScopeCurrentUser {
		t.Errorf("Windows.InstallScope = %q, want %q", e.proj.Windows.InstallScope, packproject.InstallScopeCurrentUser)
	}

	e.installScopeSelect.SetSelected(installScopeLabelAllUsers)
	if e.proj.Windows.InstallScope != packproject.InstallScopeAllUsers {
		t.Errorf("Windows.InstallScope = %q, want %q", e.proj.Windows.InstallScope, packproject.InstallScopeAllUsers)
	}
}

// TestEditor_RefreshIdentityTabRestoresInstallScope is a regression-shaped
// test for the same class of bug New/Open Project has hit before on this
// tab (see the PNG icon thumbnail's and InstallExperience checks' own
// tests): refreshIdentityTab must push a loaded project's InstallScope
// into the Select, not just leave it at whatever an earlier project left
// behind.
func TestEditor_RefreshIdentityTabRestoresInstallScope(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Windows.InstallScope = packproject.InstallScopeCurrentUser

	e.refreshIdentityTab()

	if e.installScopeSelect.Selected != installScopeLabelCurrentUser {
		t.Errorf("installScopeSelect.Selected = %q, want %q", e.installScopeSelect.Selected, installScopeLabelCurrentUser)
	}
}

// TestEditor_RefreshIdentityTabDefaultsInstallScopeToAllUsersWhenBlank
// covers a project loaded before this field existed (install_scope ""),
// or one that never went through packproject.Defaults() (e.g. a
// brand-new in-memory Project in a test) - it must read as "All users",
// the pre-existing behavior, not some third blank Select state.
func TestEditor_RefreshIdentityTabDefaultsInstallScopeToAllUsersWhenBlank(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Windows.InstallScope = ""

	e.refreshIdentityTab()

	if e.installScopeSelect.Selected != installScopeLabelAllUsers {
		t.Errorf("installScopeSelect.Selected = %q, want %q", e.installScopeSelect.Selected, installScopeLabelAllUsers)
	}
}

func TestEditor_HooksEntriesWriteIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.preInstallHookEntry.SetText("echo pre-install")
	if e.proj.Hooks.PreInstall != "echo pre-install" {
		t.Errorf("Hooks.PreInstall = %q, want %q", e.proj.Hooks.PreInstall, "echo pre-install")
	}

	e.postUninstallHookEntry.SetText("echo post-uninstall")
	if e.proj.Hooks.PostUninstall != "echo post-uninstall" {
		t.Errorf("Hooks.PostUninstall = %q, want %q", e.proj.Hooks.PostUninstall, "echo post-uninstall")
	}
}

func TestEditor_PostBuildHookEntryWritesIntoProject(t *testing.T) {
	e := newTestEditor(t)

	e.postBuildHookEntry.SetText("echo post-build")
	if e.proj.Output.PostBuildHook != "echo post-build" {
		t.Errorf("Output.PostBuildHook = %q, want %q", e.proj.Output.PostBuildHook, "echo post-build")
	}
}

// TestEditor_RefreshIdentityTabRestoresHooksAndPostBuildHook is a
// regression-shaped test for the same class of bug New/Open Project has
// hit before on this tab (see the Install Scope/InstallExperience checks'
// own tests): refreshIdentityTab must push a loaded project's Hooks and
// Output.PostBuildHook text into their entries, not just leave them at
// whatever an earlier project left behind.
func TestEditor_RefreshIdentityTabRestoresHooksAndPostBuildHook(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Hooks = packproject.Hooks{PreInstall: "echo pre", PostUninstall: "echo post"}
	e.proj.Output.PostBuildHook = "echo build"

	e.refreshIdentityTab()

	if e.preInstallHookEntry.Text != "echo pre" {
		t.Errorf("preInstallHookEntry.Text = %q, want %q", e.preInstallHookEntry.Text, "echo pre")
	}
	if e.postUninstallHookEntry.Text != "echo post" {
		t.Errorf("postUninstallHookEntry.Text = %q, want %q", e.postUninstallHookEntry.Text, "echo post")
	}
	if e.postBuildHookEntry.Text != "echo build" {
		t.Errorf("postBuildHookEntry.Text = %q, want %q", e.postBuildHookEntry.Text, "echo build")
	}
}

func TestEditor_GenerateIDButtonFillsIDField(t *testing.T) {
	e := newTestEditor(t)
	e.nameEntry.SetText("Test App")
	e.publisherEntry.SetText("someone@example.com")

	// The button's own OnTapped just calls suggestBundleID and SetTexts the
	// result — exercised directly here since simulating a Tap would need
	// locating the button among the form's children, more machinery for no
	// extra confidence over calling the same code path the button calls.
	e.idEntry.SetText(suggestBundleID(e.proj))

	if e.proj.Identity.ID != "com.example.test-app" {
		t.Errorf("Identity.ID = %q, want com.example.test-app", e.proj.Identity.ID)
	}
}
