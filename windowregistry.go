package main

// windowregistry.go implements CLAUDE.md's "Hide all / show all windows"
// convention properly: Show All Windows must restore exactly the set of
// windows that was open, not every window object Fyne's driver still
// happens to hold. Confirmed as a real, reproducible bug via Allan's own
// hands-on testing (not just a theoretical edge case): open Release
// Notes/Help/About/Update, explicitly close two of them, then Hide All
// Windows followed by Show All Windows brought all four back, including
// the two just closed. Root cause: Fyne has no reliable
// Window.IsVisible(), and every one of this app's secondary windows'
// SetCloseIntercept only ever calls Hide() (see about.go/help.go/
// update.go/releasenotes.go — none of them destroy the window on close,
// so it stays), so the previous "just Show()/Hide() every window
// a.Driver().AllWindows() returns" approach couldn't distinguish "open"
// from "exists but was dismissed." Ported from
// ../KrankyBearClipboardSentinel's own windowregistry.go, the reference
// implementation for this exact convention across the KrankyBear project
// family.
import "fyne.io/fyne/v2"

// managedWindow pairs a window with an explicitly tracked "open" flag.
// hideAllWindows/showAllWindows only call Hide()/Show() (which does not
// trigger a window's own close-intercept), so open reflects the user's
// logical "I still want this window around" state, not momentary
// visibility — only each window's own close-intercept sets it false.
type managedWindow struct {
	win  fyne.Window
	open bool
}

var managedWindows []*managedWindow

// registerManagedWindow adds win to the hide-all/show-all set and returns a
// handle the caller flips .open around its own Show()/close-intercept.
//
// Call once per window: for the main window, once at startup; for a
// singleton secondary window (about/help/update/release notes), once the
// first time it's lazily created — not on every reuse of the same window.
func registerManagedWindow(win fyne.Window) *managedWindow {
	mw := &managedWindow{win: win}
	managedWindows = append(managedWindows, mw)
	return mw
}

// hideAllWindows hides every currently-open managed window without
// changing their open flags, so showAllWindows can restore exactly this set.
func hideAllWindows() {
	for _, mw := range managedWindows {
		if mw.open {
			mw.win.Hide()
		}
	}
}

// showAllWindows re-shows every window left open by the last hideAllWindows
// (or that was never hidden at all). Callers focus the main window
// afterward themselves (see main.go's own View/tray menu wiring) — this
// function only restores visibility, matching this same split in
// ../KrankyBearClipboardSentinel's own windowregistry.go.
func showAllWindows() {
	for _, mw := range managedWindows {
		if mw.open {
			mw.win.Show()
		}
	}
}
