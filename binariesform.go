package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/binscan"
	"installerbear/internal/packproject"
)

// binaryField is one (OS, arch) slot's Entry, bound to that exact pair —
// unlike an earlier design with one Entry + an arch dropdown per OS, each
// field here maps 1:1 to a specific BinaryEntry, so there's nothing to
// collapse/overwrite when another arch's field changes or when a refresh
// repopulates the form. Registering both amd64 and arm64 for an OS is what
// tells packager.Run to build both — see its own comment on the multi-arch
// loop.
type binaryField struct {
	os, arch string
	entry    *widget.Entry
}

// buildBinariesTab lays out one row per (OS, arch) pair this tool knows how
// to build for for — two rows per OS, since every backend here only ever
// targets amd64/arm64.
func (e *editor) buildBinariesTab() fyne.CanvasObject {
	e.binaryFields = []binaryField{
		{os: "windows", arch: "amd64", entry: widget.NewEntry()},
		{os: "windows", arch: "arm64", entry: widget.NewEntry()},
		{os: "darwin", arch: "amd64", entry: widget.NewEntry()},
		{os: "darwin", arch: "arm64", entry: widget.NewEntry()},
		{os: "linux", arch: "amd64", entry: widget.NewEntry()},
		{os: "linux", arch: "arm64", entry: widget.NewEntry()},
	}

	items := make([]*widget.FormItem, 0, len(e.binaryFields))
	for i := range e.binaryFields {
		f := &e.binaryFields[i]
		f.entry.OnChanged = func(path string) { e.setBinary(f.os, f.arch, path) }
		items = append(items, widget.NewFormItem(binaryFieldLabel(f.os, f.arch), newBrowseFileRow(e.win, f.entry)))
	}
	form := widget.NewForm(items...)

	scanBtn := widget.NewButton("Scan folder...", func() { e.scanBinariesFolder() })

	hint := widget.NewLabel("Leave an arch blank if you don't build for it (e.g. Windows ARM64). Building a target produces one output per arch you've filled in here — no separate arch picker.")
	hint.Wrapping = fyne.TextWrapWord

	return container.NewVScroll(container.NewVBox(sectionHeader("Binaries"), form, scanBtn, hint))
}

// scanBinariesFolder lets the user point at a directory (e.g. a "bin" build
// output folder) and best-effort-fills any still-empty (OS, arch) slots by
// filename convention via binscan.ScanDir. Never overwrites a slot that
// already has a manually-set path. Shows a summary of what was matched so a
// silent 6-field auto-fill doesn't surprise the user.
func (e *editor) scanBinariesFolder() {
	dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		guesses, err := binscan.ScanDir(u.Path())
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}

		filled := make(map[string]string)
		for _, g := range guesses {
			if bin, ok := e.proj.BinaryFor(g.OS, g.Arch); ok && bin.Path != "" {
				continue
			}
			key := binaryFieldLabel(g.OS, g.Arch)
			if _, already := filled[key]; already {
				continue // ambiguous: more than one file matched this slot, skip it
			}
			filled[key] = g.Path
		}
		for _, f := range e.binaryFields {
			if path, ok := filled[binaryFieldLabel(f.os, f.arch)]; ok {
				e.setBinary(f.os, f.arch, path)
			}
		}
		e.refreshBinariesTab()

		dialog.ShowInformation("Scan folder", scanSummary(e.binaryFields, filled), e.win)
	}, e.win).Show()
}

// scanSummary reports which of the 6 slots the scan filled and which it
// left alone, in a stable (matching the form's) order.
func scanSummary(fields []binaryField, filled map[string]string) string {
	var lines []string
	for _, f := range fields {
		label := binaryFieldLabel(f.os, f.arch)
		if path, ok := filled[label]; ok {
			lines = append(lines, fmt.Sprintf("%s: %s", label, path))
		} else {
			lines = append(lines, fmt.Sprintf("%s: no match", label))
		}
	}
	return strings.Join(lines, "\n")
}

func binaryFieldLabel(os, arch string) string {
	names := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}
	return names[os] + " (" + arch + ")"
}

// setBinary upserts or removes the single BinaryEntry for exactly this
// (os, arch) pair — never touches any other entry, so it's safe to call
// both from a real edit and from refreshBinariesTab's SetText calls.
func (e *editor) setBinary(os, arch, path string) {
	for i, b := range e.proj.Binaries {
		if b.OS == os && b.Arch == arch {
			if path == "" {
				e.proj.Binaries = append(e.proj.Binaries[:i], e.proj.Binaries[i+1:]...)
			} else {
				e.proj.Binaries[i].Path = path
			}
			return
		}
	}
	if path != "" {
		e.proj.Binaries = append(e.proj.Binaries, packproject.BinaryEntry{OS: os, Arch: arch, Path: path})
	}
}

func (e *editor) refreshBinariesTab() {
	for _, f := range e.binaryFields {
		bin, _ := e.proj.BinaryFor(f.os, f.arch)
		f.entry.SetText(bin.Path)
	}
}
