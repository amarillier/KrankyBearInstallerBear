package main

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"installerbear/internal/packager"
)

func TestEditor_CleanOldVersionsCheckBindsToProject(t *testing.T) {
	e := newTestEditor(t)

	test.Tap(e.cleanOldVersionsCheck)
	if !e.proj.Output.CleanOldVersions {
		t.Error("checking the box should set Output.CleanOldVersions")
	}

	e.proj.Output.CleanOldVersions = false
	e.refreshBuildTab()
	if e.cleanOldVersionsCheck.Checked {
		t.Error("refreshBuildTab should reflect Output.CleanOldVersions back into the checkbox")
	}
}

func TestEditor_CopyBuildLogPutsTextOnClipboard(t *testing.T) {
	e := newTestEditor(t)
	e.logEntry.SetText("some build output")

	e.copyBuildLog()

	if got := e.app.Clipboard().Content(); got != "some build output" {
		t.Errorf("clipboard = %q, want %q", got, "some build output")
	}
}

// TestEditor_OfferCleanupOldInstallers_NoOpWhenDisabled confirms the
// checkbox being off means StaleInstallers is never even consulted — no
// dialog, no chance of a surprise prompt for a project that never opted in.
func TestEditor_OfferCleanupOldInstallers_NoOpWhenDisabled(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Output.CleanOldVersions = false
	e.proj.Identity.Version = "0.3.0"

	// Should return immediately without touching the filesystem or trying
	// to show a dialog; if it panicked or blocked, the test would hang/fail.
	e.offerCleanupOldInstallers([]packager.BuildResult{
		{Target: packager.TargetDEB, OutputPath: "/nonexistent/app_0.3.0.deb"},
	})
}
