package main

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"installerbear/internal/packproject"
)

// sampleProjectBundledName is the filename this repo's own
// installerbear.yaml packages as a Payload entry (Dest: "") so it lands
// beside the installed binary on every backend, the same convention
// releasenotes.go's own lookup uses for ReleaseNotes.md/.txt. Kept as a
// distinct filename (not "installerbear.yaml") so it can never collide
// with, or be mistaken for, an unrelated project file someone happens to
// have in the same folder as the installed exe.
const sampleProjectBundledName = "sample-installerbear.yaml"

// newSampleProject lets the user start from a real, fully-featured example
// project instead of a blank one — Allan's own idea, prompted by the
// GUI's smart defaults still leaving every field blank for someone who's
// never seen an installerbear.yaml before. Prefers the bundled sample this
// app ships with itself (a real, working config — see
// sampleProjectBundledName's own comment); falls back to a generated
// in-code sample (generatedSampleProject) when that file isn't found,
// e.g. running via `go run .` before packaging, or a portable build
// without its Payload files attached.
func (e *editor) newSampleProject() {
	e.confirmDiscardIfDirty(func() {
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

			data, err := sampleProjectYAML()
			if err != nil {
				dialog.ShowError(err, e.win)
				return
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				dialog.ShowError(err, e.win)
				return
			}

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
		fd.SetFileName("installerbear.yaml")
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
		fd.Show()
	})
}

// sampleProjectYAML returns the bundled sample's bytes verbatim if found
// beside the running executable, otherwise marshals generatedSampleProject
// the same way Save/Save As would. Mirrors releasenotes.go's own
// find-beside-the-executable-or-fall-back pattern.
func sampleProjectYAML() ([]byte, error) {
	if exePath, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(exePath), sampleProjectBundledName)
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}
	return packproject.Marshal(generatedSampleProject())
}

// generatedSampleProject is the fallback used when this app's own bundled
// sample file can't be found. Deliberately built as a real
// packproject.Project value (not a hand-written YAML string): every field
// name is checked by the Go compiler against the current schema, so a
// future field rename can't silently leave this sample stale the way a
// separate hand-maintained template string could. A fresh UpgradeGUID is
// generated on every call (never a fixed placeholder) so nobody who saves
// several sample projects in a row ends up with colliding Windows
// UpgradeCodes.
func generatedSampleProject() *packproject.Project {
	return &packproject.Project{
		Identity: packproject.Identity{
			Name:        "My Sample App",
			ID:          "com.example.mysampleapp",
			Version:     "1.0.0",
			Publisher:   "Your Name or Company",
			Vendor:      "Your Name or Company",
			URL:         "https://github.com/you/my-sample-app",
			Description: "A short description of what this application does.",
			License:     "MIT",
			LicenseFile: "LICENSE",
			Icons: packproject.IconSet{
				ICO:  "assets/images/icon.ico",
				ICNS: "assets/images/icon.icns",
				PNG:  "assets/images/icon.png",
			},
		},
		Binaries: []packproject.BinaryEntry{
			{OS: "windows", Arch: "amd64", Path: "bin/MySampleApp.exe"},
			{OS: "darwin", Arch: "amd64", Path: "bin/my-sample-app-macos-amd64"},
			{OS: "darwin", Arch: "arm64", Path: "bin/my-sample-app-macos-arm64"},
			{OS: "linux", Arch: "amd64", Path: "bin/my-sample-app-linux-amd64"},
			{OS: "linux", Arch: "arm64", Path: "bin/my-sample-app-linux-arm64"},
		},
		Payload: []packproject.PayloadEntry{
			{Source: "ReleaseNotes.md", Dest: ""},
			{Source: "assets", Dest: "assets", Recursive: true},
		},
		Install: packproject.InstallLocations{
			Windows: `$PROGRAMFILES64\My Sample App`,
			MacOS:   "/Applications/My Sample App.app",
			Linux:   "/opt/My Sample App",
		},
		InstallExperience: packproject.InstallExperience{
			LaunchAfterInstall: true,
			DesktopShortcut:    true,
		},
		Windows: packproject.WindowsOptions{
			UpgradeGUID: "{" + uuid.NewString() + "}",
			ExeName:     "MySampleApp.exe",
		},
		MacOS: packproject.MacOSOptions{
			BundleExecutable: "MySampleApp",
			MinSystemVersion: "10.13",
			Category:         "public.app-category.utilities",
		},
		Linux: packproject.LinuxOptions{
			DesktopCategories: "Utility",
			DesktopComment:    "A short description of what this application does.",
		},
		Targets: packproject.AllTargets,
		Output: packproject.OutputOptions{
			Dir: "installers",
		},
	}
}
