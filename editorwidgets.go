package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// newBrowseFileRow pairs entry with a "Browse" button that opens a file-open
// dialog filtered to exts (each like ".png", pass none to allow anything)
// and writes the picked path into entry — which fires entry's own OnChanged,
// so the caller never needs a separate callback for the browse case.
func newBrowseFileRow(win fyne.Window, entry *widget.Entry, exts ...string) fyne.CanvasObject {
	browse := widget.NewButton("Browse...", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			entry.SetText(reader.URI().Path())
		}, win)
		if len(exts) > 0 {
			fd.SetFilter(storage.NewExtensionFileFilter(exts))
		}
		fd.Show()
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

// newBrowseFolderRow is newBrowseFileRow's directory-picking counterpart,
// used for the output directory field.
func newBrowseFolderRow(win fyne.Window, entry *widget.Entry) fyne.CanvasObject {
	browse := widget.NewButton("Browse...", func() {
		dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			entry.SetText(u.Path())
		}, win).Show()
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

// bindEntry wires an Entry's OnChanged straight to a project-field setter —
// every identity/binaries field follows this "OnChanged writes straight into
// e.proj" pattern, so refresh (on New/Open) and edit (as the user types)
// share one code path with nothing to resynchronize by hand.
func bindEntry(entry *widget.Entry, set func(string)) *widget.Entry {
	entry.OnChanged = set
	return entry
}
