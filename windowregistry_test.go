package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// spyWindow embeds a real headless test window and only overrides Show/Hide
// to record whether each was called - fyne/test's own Show/Hide are no-ops
// that don't affect Content().Visible() (confirmed by reading Fyne v2.8.1's
// own test/window.go: Hide just clears focus, Show just requests it), so
// checking Content().Visible() the way this app's own production
// singleton-window reuse check does can't be used to observe registry
// behavior in a headless test - only a real glfw window propagates
// Hide()/Show() down to content visibility. Tracking calls directly here
// instead is simpler than a full fyne.Window mock.
type spyWindow struct {
	fyne.Window
	shown, hidden bool
}

func newSpyWindow() *spyWindow {
	return &spyWindow{Window: test.NewWindow(nil)}
}

func (s *spyWindow) Show() {
	s.shown = true
	s.Window.Show()
}

func (s *spyWindow) Hide() {
	s.hidden = true
	s.Window.Hide()
}

// TestHideShowAllWindows_RestoresExactlyTheOpenSet is a regression test for
// the real bug Allan found by hand: hide all, then show all, used to bring
// back every window Fyne's driver still held onto (including ones
// explicitly closed earlier), not just the ones that were actually open
// when hideAllWindows ran.
func TestHideShowAllWindows_RestoresExactlyTheOpenSet(t *testing.T) {
	managedWindows = nil // isolate from any other test in this package
	t.Cleanup(func() { managedWindows = nil })

	winA := newSpyWindow()
	t.Cleanup(winA.Close)
	mwA := registerManagedWindow(winA)
	mwA.open = true

	winB := newSpyWindow()
	t.Cleanup(winB.Close)
	mwB := registerManagedWindow(winB)
	mwB.open = true

	// Simulate the user explicitly closing window B (its own
	// SetCloseIntercept would set open = false before Hide(), exactly like
	// about.go/help.go/update.go/releasenotes.go do) *before* hideAllWindows
	// ever runs - the exact sequence Allan's own manual test followed.
	mwB.open = false
	winB.hidden = false // reset: the close-intercept's own Hide() isn't under test here

	hideAllWindows()
	if !winA.hidden {
		t.Error("expected Hide() to be called on window A (still open)")
	}
	if winB.hidden {
		t.Error("expected hideAllWindows to skip window B (already marked closed)")
	}

	winA.shown, winB.shown = false, false // reset before the real assertion
	showAllWindows()
	if !winA.shown {
		t.Error("expected Show() to be called on window A (still open)")
	}
	if winB.shown {
		t.Error("expected window B (explicitly closed before hideAllWindows ran) to stay hidden - showAllWindows must not call Show() on it")
	}
}

// TestShowAllWindows_NeverShowsAClosedWindow pins down the core invariant
// directly: showAllWindows only ever calls Show() on a managedWindow whose
// open flag is true, regardless of what hideAllWindows previously did.
func TestShowAllWindows_NeverShowsAClosedWindow(t *testing.T) {
	managedWindows = nil
	t.Cleanup(func() { managedWindows = nil })

	win := newSpyWindow()
	t.Cleanup(win.Close)
	mw := registerManagedWindow(win)
	mw.open = false

	showAllWindows()
	if win.shown {
		t.Error("expected showAllWindows to never call Show() on a window with open = false")
	}
}

// TestRegisterManagedWindow_AppendsToRegistry confirms each call adds a
// distinct entry rather than replacing a prior one - main.go and each
// secondary window's show* function each register once, independently.
func TestRegisterManagedWindow_AppendsToRegistry(t *testing.T) {
	managedWindows = nil
	t.Cleanup(func() { managedWindows = nil })

	win1 := newSpyWindow()
	t.Cleanup(win1.Close)
	win2 := newSpyWindow()
	t.Cleanup(win2.Close)

	registerManagedWindow(win1)
	registerManagedWindow(win2)

	if len(managedWindows) != 2 {
		t.Fatalf("len(managedWindows) = %d, want 2", len(managedWindows))
	}
}
