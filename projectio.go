package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

// newProject discards the current in-memory project (no unsaved-changes
// prompt in this v1 — see the CLAUDE.md-style note in the plan about
// scope) and starts a new one rooted at a directory the user picks, so its
// Identity can be best-effort-prefilled from that directory's git
// remote/identity, LICENSE file, and icons (see applyScannedDefaults).
// Cancelling the folder picker leaves whatever project was already open
// untouched.
func (e *editor) newProject() {
	dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		e.proj = &packproject.Project{BaseDir: u.Path()}
		e.path = ""
		e.applyScannedDefaults(u.Path())
		e.refreshAll()
	}, e.win).Show()
}

// applyScannedDefaults best-effort-populates Identity fields from
// projectscan.Scan. Every field is only ever filled if still blank — on a
// brand new project every field already is, but the guard keeps this safe
// to reuse later against an in-progress project.
func (e *editor) applyScannedDefaults(dir string) {
	d := projectscan.Scan(dir)
	if e.proj.Identity.URL == "" {
		e.proj.Identity.URL = d.URL
	}
	if e.proj.Identity.Publisher == "" {
		e.proj.Identity.Publisher = d.Publisher
	}
	if e.proj.Identity.LicenseFile == "" {
		e.proj.Identity.LicenseFile = d.LicenseFile
	}
	if e.proj.Identity.Icons.ICO == "" {
		e.proj.Identity.Icons.ICO = d.IconICO
	}
	if e.proj.Identity.Icons.ICNS == "" {
		e.proj.Identity.Icons.ICNS = d.IconICNS
	}
	if e.proj.Identity.Icons.PNG == "" {
		e.proj.Identity.Icons.PNG = d.IconPNG
	}
}

// openProject reads a project file leniently — unlike the CLI's
// packproject.Load, this does NOT call Validate, so a work-in-progress
// project someone saved mid-edit (missing a version, say) can still be
// reopened and finished, rather than refusing to load at all. Validation
// still happens for real right before an actual build (buildpanel.go's
// startBuild).
func (e *editor) openProject() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		if reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()

		proj, err := packproject.LoadLenient(path)
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		e.proj = proj
		e.path = path
		e.refreshAll()
	}, e.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
	fd.Show()
}

// saveProject writes to the already-known path, or falls back to
// saveProjectAs for a never-saved project.
func (e *editor) saveProject() {
	if e.path == "" {
		e.saveProjectAs()
		return
	}
	if err := packproject.Save(e.proj, e.path); err != nil {
		dialog.ShowError(err, e.win)
		return
	}
	e.updateWindowTitle()
}

func (e *editor) saveProjectAs() {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		if writer == nil {
			return
		}
		path := writer.URI().Path()
		writer.Close()

		if err := packproject.Save(e.proj, path); err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		e.path = path
		e.proj.BaseDir = filepath.Dir(path)
		e.updateWindowTitle()
	}, e.win)
	fd.SetFileName("packman.yaml")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
	fd.Show()
}
