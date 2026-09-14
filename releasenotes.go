package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var releaseNotesWindow fyne.Window
var releaseNotesManagedWindow *managedWindow // hide-all/show-all tracking, see windowregistry.go

// releaseNotesFileNames are tried in order, both as siblings of the
// running executable. .md variants first: rendered through Fyne's own
// built-in widget.NewRichTextFromMarkdown (no new dependency - real
// headers/bold/lists/links if the file has real Markdown syntax, and
// still reads fine as plain paragraphs if it doesn't, so this is safe for
// a project's existing plain ReleaseNotes.txt too, not just a future
// ReleaseNotes.md). Both PascalCase (this project's own convention) and
// all-lowercase are checked - Linux's filesystem is case-sensitive, and a
// future switch to real Markdown could plausibly land as either
// releasenotes.md or ReleaseNotes.md. Every project config this repo
// ships (installerbear.yaml, installerbearIAM-manualConfig.yaml) packages
// ReleaseNotes.txt as a Payload entry with Dest: "" (the install root) -
// winexe/winmsi/debrpm all land that directly alongside the binary, and
// macpkg's own Payload handling lands it in Contents/MacOS/ (where the
// binary itself lives too) the same way - so "next to the running
// executable" is the right, portable place to look on every OS this app
// ships for.
var releaseNotesFileNames = []string{
	"ReleaseNotes.md", "releasenotes.md",
	"ReleaseNotes.txt", "releasenotes.txt",
}

// showReleaseNotes displays the installed release notes file in its own
// window, matching showHelp's pattern (singleton, scrollable, Hide not
// Close). Deliberately reads from disk rather than embedding the text
// into the binary at compile time: release notes only grow over a
// project's life, an embed would be baked in and go stale until the next
// rebuild, and this app's own convention already installs the file as a
// real Payload entry precisely so it can be read like this. The tradeoff
// that comes with reading from disk - a standalone/portable binary
// copied on its own, without the file that normally travels alongside it
// - is handled explicitly below rather than silently failing or
// shelling out to the OS's default text viewer (what most installers do,
// and reads as an afterthought rather than a real part of the app).
//
// The file read and widget.NewRichTextFromMarkdown parse/build both run
// in a background goroutine, not inline here - found via real testing
// against a project with a release notes file close to 400KB: doing both
// synchronously before the window ever appears froze the whole app (macOS
// spinning-wait-cursor) for several seconds, since this runs on the main
// goroutine (a Help-menu click) or one already wrapped in fyne.Do (the
// tray item). The window now shows immediately with a loading indicator,
// and fyne.Do below swaps the real content in once it's ready - mandatory
// per CLAUDE.md's "fyne.Do is mandatory" (the goroutine itself must never
// touch releaseNotesWindow's canvas objects directly). Most projects'
// release notes stay small enough that this is imperceptible either way;
// this only matters once one grows large.
func showReleaseNotes(a fyne.App) {
	if releaseNotesWindow != nil && releaseNotesWindow.Content().Visible() {
		releaseNotesWindow.Show()
		releaseNotesWindow.RequestFocus()
		releaseNotesManagedWindow.open = true
		return
	}

	releaseNotesWindow = a.NewWindow(appName + " - Release Notes")
	releaseNotesWindow.SetIcon(resourceKrankyBearInstallerBearPng)
	releaseNotesManagedWindow = registerManagedWindow(releaseNotesWindow)

	loadingBar := widget.NewProgressBarInfinite()
	loading := container.NewVBox(
		widget.NewLabel("Loading release notes..."),
		loadingBar,
	)

	scroll := container.NewScroll(loading)
	scroll.SetMinSize(fyne.NewSize(700, 550))

	header := container.NewVBox(
		widget.NewLabelWithStyle(appName+" - Release Notes", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)

	releaseNotesWindow.SetContent(container.NewPadded(container.NewBorder(header, nil, nil, nil, scroll)))
	releaseNotesWindow.Resize(fyne.NewSize(760, 620))
	releaseNotesWindow.SetCloseIntercept(func() {
		releaseNotesManagedWindow.open = false
		releaseNotesWindow.Hide()
	})
	releaseNotesManagedWindow.open = true
	releaseNotesWindow.Show()

	go func() {
		body := releaseNotesBody()
		richText := widget.NewRichTextFromMarkdown(body)
		richText.Wrapping = fyne.TextWrapWord

		fyne.Do(func() {
			loadingBar.Stop() // halts its animation goroutine before it's dropped
			scroll.Content = richText
			scroll.Refresh()
			// scroll.Refresh() alone only recomputes the widget's own layout
			// (confirmed by reading internal/widget/scroller.go's renderer -
			// it re-associates Objects()[0] and calls Layout(), neither of
			// which touches the canvas's own dirty flag). The actual repaint
			// trigger is the canvas's own Refresh(obj), which queues the
			// object and calls SetDirty() - the exact thing the 60Hz paint
			// loop's CheckDirtyAndClear() gates on. Without this, the swap
			// happens correctly in memory but never gets painted until
			// something else incidentally marks the canvas dirty (closing
			// and reopening the window, or another Show()/RequestFocus() -
			// both confirmed via real testing to "fix" it, which is what
			// gave this away) - a real bug, not a hypothetical one.
			releaseNotesWindow.Canvas().Refresh(scroll)
		})
	}()
}

// releaseNotesBody reads the first of releaseNotesFileNames found beside
// the running executable, or explains clearly why none could be when
// that fails - a standalone binary copied without its accompanying files
// is a real, expected scenario (not a bug to hide), so this reports it
// plainly rather than silently showing an empty window or crashing.
// Returned as Markdown either way (a real link in the not-found case, and
// - see releaseNotesFileNames' own comment - safe for a plain-text file
// too), since the caller always renders through RichTextFromMarkdown.
func releaseNotesBody() string {
	exePath, err := os.Executable()
	if err != nil {
		return "**Release Notes could not be found.**\n\n" +
			"(Could not determine this program's own location: " + err.Error() + ")"
	}
	dir := filepath.Dir(exePath)
	for _, name := range releaseNotesFileNames {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return string(data)
		}
	}
	return "**Release Notes could not be found.**\n\n" +
		"This usually means you're running a standalone copy of " + appName + " without " +
		"the files that normally install alongside it. Check the project's release history here:\n\n" +
		"[" + appName + " on GitHub](https://github.com/amarillier/KrankyBearInstallerBear)"
}

// "There is a crack in everything, that's how the light gets in." Leonard Cohen
