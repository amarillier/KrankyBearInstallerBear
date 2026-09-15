package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// dialoglocations.go picks a sensible starting directory for the four
// project file/folder dialogs (New Project, New Sample Project..., Open
// Project..., Save Project As...) instead of leaving it to whatever the
// OS/Fyne's own default happens to be (often the user's home directory) -
// Allan's own idea, prompted by a fresh install otherwise dropping a
// first-time user in their home folder with no clue the bundled
// ReleaseNotes.md/sample-installerbear.yaml exist right next to the
// installed binary.
//
// prefLastProjectDir is shared across all four kinds of dialog - once any
// one of them has been used successfully, its own directory becomes the
// starting point for all of them from then on (the common "remember the
// last folder I was in" convention most file-dialog-heavy apps use),
// which takes priority over both first-time defaults below.
const prefLastProjectDir = "lastProjectDir"

// discoveryDialogStartDir is where New Project/Open Project/New Sample
// Project should start browsing before anything's ever been remembered:
// beside the running executable, not the user's home directory. A real
// install already ships ReleaseNotes.md and sample-installerbear.yaml
// right there — starting the browse there instead of in the user's empty
// home folder means a first-time user actually sees them, rather than
// needing to already know to go looking. New Sample Project counts as
// "discovery" here too, even though it's technically a Save dialog: its
// whole purpose is pointing a first-time user at a real example, so
// showing them the folder that already has one is the same idea as Open/
// New Project. Falls back to the home directory if the executable's own
// path can't be resolved (should be rare).
func discoveryDialogStartDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}
	return saveDialogStartDir()
}

// saveDialogStartDir is where Save Project As should start browsing
// before anything's ever been remembered: the user's own home directory.
// A personal project file belongs there, not inside the application's
// own install directory (which on Windows/macOS is often not even
// writable by a normal user without elevation).
func saveDialogStartDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "."
}

// dialogStartLocation resolves the fyne.ListableURI a file/folder dialog's
// own SetLocation should be pointed at: the remembered last-used project
// directory if one exists, else the appropriate first-time default for a
// "discovery" (New/Open/New Sample Project) vs "save" (Save Project As)
// dialog. Returns nil (Fyne's own dialog default takes over silently) if
// the resolved directory can't actually be listed - e.g. it's since been
// deleted, or storage.ListerForURI just doesn't like it for some reason;
// this is a convenience default, never worth failing a dialog over.
func dialogStartLocation(a fyne.App, forSave bool) fyne.ListableURI {
	dir := a.Preferences().String(prefLastProjectDir)
	if dir == "" {
		if forSave {
			dir = saveDialogStartDir()
		} else {
			dir = discoveryDialogStartDir()
		}
	}
	lister, err := storage.ListerForURI(storage.NewFileURI(dir))
	if err != nil {
		return nil
	}
	return lister
}

// rememberProjectDir updates the shared last-used-directory preference
// after a New/Open/Save/Save As dialog successfully resolves to a real
// path, so the next dialog of any of these four kinds starts there next
// time - called with the directory a just-picked file or folder lives in.
func rememberProjectDir(a fyne.App, dir string) {
	if dir == "" {
		return
	}
	a.Preferences().SetString(prefLastProjectDir, dir)
}
