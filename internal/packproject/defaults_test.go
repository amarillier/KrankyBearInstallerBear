package packproject

import "testing"

func TestDefaults_InstallScopeDefaultsToAllUsers(t *testing.T) {
	p := &Project{Identity: Identity{Name: "TestApp"}}
	p.Defaults()

	if p.Windows.InstallScope != InstallScopeAllUsers {
		t.Errorf("Windows.InstallScope = %q, want %q", p.Windows.InstallScope, InstallScopeAllUsers)
	}
}

func TestDefaults_InstallWindowsUsesProgramFilesForAllUsers(t *testing.T) {
	p := &Project{Identity: Identity{Name: "TestApp"}}
	p.Defaults()

	if want := `$PROGRAMFILES64\TestApp`; p.Install.Windows != want {
		t.Errorf("Install.Windows = %q, want %q", p.Install.Windows, want)
	}
}

func TestDefaults_InstallWindowsUsesLocalAppDataForCurrentUser(t *testing.T) {
	p := &Project{
		Identity: Identity{Name: "TestApp"},
		Windows:  WindowsOptions{InstallScope: InstallScopeCurrentUser},
	}
	p.Defaults()

	if want := `$LOCALAPPDATA\TestApp`; p.Install.Windows != want {
		t.Errorf("Install.Windows = %q, want %q", p.Install.Windows, want)
	}
}

// TestDefaults_NeverOverwritesAnAlreadySetInstallWindows guards the same
// "only fill blanks" rule every other Defaults() field already follows -
// an author who already set Install.Windows explicitly (e.g. via "Import
// Existing Config") must not have it silently replaced just because
// InstallScope also happens to be set.
func TestDefaults_NeverOverwritesAnAlreadySetInstallWindows(t *testing.T) {
	p := &Project{
		Identity: Identity{Name: "TestApp"},
		Windows:  WindowsOptions{InstallScope: InstallScopeCurrentUser},
		Install:  InstallLocations{Windows: `C:\Custom\Path`},
	}
	p.Defaults()

	if p.Install.Windows != `C:\Custom\Path` {
		t.Errorf("Install.Windows = %q, want the already-set value preserved", p.Install.Windows)
	}
}
