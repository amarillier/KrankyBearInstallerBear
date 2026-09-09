package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"installerbear/internal/binscan"
	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

// isDirty reports whether e.proj has changed since it was last loaded or
// saved — see mainwindow.go's lastSavedSnapshot for why this compares a
// fresh encoding rather than tracking a live "dirty" flag. A marshal
// failure is treated as "not dirty" (best-effort; New/Open should never be
// blocked by this check misbehaving).
func (e *editor) isDirty() bool {
	current, err := packproject.Marshal(e.proj)
	if err != nil {
		return false
	}
	return !bytes.Equal(current, e.lastSavedSnapshot)
}

// markSaved resets the unsaved-changes baseline to e.proj's current state —
// call after New/Open replaces e.proj wholesale, and after a successful
// Save/Save As.
func (e *editor) markSaved() {
	snapshot, err := packproject.Marshal(e.proj)
	if err != nil {
		e.lastSavedSnapshot = nil
		return
	}
	e.lastSavedSnapshot = snapshot
}

// confirmDiscardIfDirty runs proceed immediately if there's nothing unsaved
// to lose; otherwise it asks first, running proceed only if the user
// confirms discarding the current project's changes. Shared by New Project
// and Open Project — both are about to replace e.proj wholesale.
func (e *editor) confirmDiscardIfDirty(proceed func()) {
	if !e.isDirty() {
		proceed()
		return
	}
	dialog.ShowConfirm(
		"Discard Unsaved Changes?",
		"This project has unsaved changes. Continue and discard them?",
		func(ok bool) {
			if ok {
				proceed()
			}
		}, e.win)
}

// newProject starts a new project rooted at a directory the user picks, so
// its Identity can be best-effort-prefilled from that directory's git
// remote/identity, LICENSE file, and icons (see applyScannedDefaults).
// Cancelling the folder picker leaves whatever project was already open
// untouched. Asks first if the current project has unsaved changes.
func (e *editor) newProject() {
	e.confirmDiscardIfDirty(func() {
		dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			e.proj = &packproject.Project{BaseDir: u.Path()}
			e.path = ""
			e.applyScannedDefaults(u.Path())
			e.refreshAll()
			e.markSaved()
		}, e.win).Show()
	})
}

// applyScannedDefaults best-effort-populates Identity fields from
// projectscan.Scan, a conventional bin/ folder (binariesform.go's own
// "Scan folder..." logic, applied automatically here to the one folder name
// every compile-*.sh script already outputs to), and the binaries that
// scan turns up. Every field is only ever filled if still blank — on a
// brand new project every field already is, but the guard keeps this safe
// to reuse later against an in-progress project.
func (e *editor) applyScannedDefaults(dir string) {
	d := projectscan.Scan(dir)
	if e.proj.Identity.Name == "" {
		e.proj.Identity.Name = d.Name
	}
	if e.proj.Identity.Version == "" {
		e.proj.Identity.Version = d.Version
	}
	if e.proj.Identity.Description == "" {
		e.proj.Identity.Description = d.Description
	}
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

	e.applyScannedBinaries(dir)
	e.applyExeNameDefaults()
}

// applyScannedBinaries best-effort-fills any still-empty Binaries slot from
// dir/bin, the folder every compile-*.sh script in this template already
// builds into — same filename-convention matching and same "never overwrite
// an already-set slot" rule as the Binaries tab's own "Scan folder..."
// action (scanBinariesFolder), just run automatically against the
// conventional folder name instead of one the user picks. A project with no
// bin/ folder (or an empty one) is left alone.
func (e *editor) applyScannedBinaries(dir string) {
	binDir := filepath.Join(dir, "bin")
	if info, err := os.Stat(binDir); err != nil || !info.IsDir() {
		return
	}
	guesses, err := binscan.ScanDir(binDir)
	if err != nil {
		return
	}

	filled := make(map[string]bool)
	for _, g := range guesses {
		if bin, ok := e.proj.BinaryFor(g.OS, g.Arch); ok && bin.Path != "" {
			continue
		}
		key := g.OS + "/" + g.Arch
		if filled[key] {
			continue // ambiguous: more than one file matched this slot, skip it
		}
		filled[key] = true
		e.proj.Binaries = append(e.proj.Binaries, packproject.BinaryEntry{OS: g.OS, Arch: g.Arch, Path: g.Path})
	}
}

// applyExeNameDefaults best-effort-fills Windows.ExeName and
// MacOS.BundleExecutable once Binaries/Identity.Name are known (whether
// from this scan or already set beforehand).
//
// Windows.ExeName is the literal installed filename (see
// internal/packager/winmsi/tree.go's addFile call), so it's taken verbatim
// from whichever Windows binary is registered — that file was already
// deliberately named (e.g. "KrankyBearInstallerBear.exe").
//
// MacOS.BundleExecutable is also a literal filename
// (internal/packager/macpkg/bundle.go copies the darwin binary to
// Contents/MacOS/<BundleExecutable>), but the registered darwin binaries
// carry an OS/arch suffix by this template's own convention
// (krankybear-installerbear-macos-amd64) that doesn't belong inside the
// bundle, so it's derived from Identity.Name instead, with whitespace
// stripped — matching this project's own installerbearNEW.yaml, where the
// Windows exe basename ("KrankyBearInstallerBear.exe") is exactly
// Identity.Name ("KrankyBear InstallerBear") with the space removed.
func (e *editor) applyExeNameDefaults() {
	if e.proj.Windows.ExeName == "" {
		for _, arch := range []string{"amd64", "arm64"} {
			if bin, ok := e.proj.BinaryFor("windows", arch); ok && bin.Path != "" {
				e.proj.Windows.ExeName = filepath.Base(bin.Path)
				break
			}
		}
	}
	if e.proj.MacOS.BundleExecutable == "" && e.proj.Identity.Name != "" {
		e.proj.MacOS.BundleExecutable = strings.Join(strings.Fields(e.proj.Identity.Name), "")
	}
}

// openProject reads a project file leniently — unlike the CLI's
// packproject.Load, this does NOT call Validate, so a work-in-progress
// project someone saved mid-edit (missing a version, say) can still be
// reopened and finished, rather than refusing to load at all. Validation
// still happens for real right before an actual build (buildpanel.go's
// startBuild). Asks first if the current project has unsaved changes.
func (e *editor) openProject() {
	e.confirmDiscardIfDirty(func() {
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
			e.markSaved()
		}, e.win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
		fd.Show()
	})
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
	e.markSaved()
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
		e.markSaved()
		e.updateWindowTitle()
	}, e.win)
	fd.SetFileName("installerbear.yaml")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
	fd.Show()
}
