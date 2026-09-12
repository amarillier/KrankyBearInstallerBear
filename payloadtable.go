package main

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

const payloadColCount = 4 // Source | Dest | Recursive | OS filter

// buildPayloadTab shows Project.Payload in a table with every cell directly
// editable in place — the Add/Edit dialog below still exists for Browse-
// assisted Source/Dest picking on a new or existing entry, but a quick Dest
// or OS-filter tweak no longer needs it. This is the GUI analogue of
// today's Inno [Files] section / fpm src=dest arguments.
//
// Uses Fyne's native ShowHeaderRow/CreateHeader/UpdateHeader (rather than
// faking a bold row 0 in the data grid, this table's original approach) so
// dragging a header column boundary resizes that column for free — that's
// a built-in Table behavior gated entirely on ShowHeaderRow being set, see
// widget.Table's own Dragged/DragEnd.
func (e *editor) buildPayloadTab() fyne.CanvasObject {
	e.payloadTable = widget.NewTable(
		func() (int, int) { return len(e.proj.Payload), payloadColCount },
		newPayloadCell,
		e.updatePayloadCell,
	)
	e.payloadTable.ShowHeaderRow = true
	e.payloadTable.CreateHeader = func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.TextStyle = fyne.TextStyle{Bold: true}
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	e.payloadTable.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		obj.(*widget.Label).SetText([]string{"Source", "Dest", "Recursive", "OS"}[id.Col])
	}
	e.payloadTable.SetColumnWidth(0, 260)
	e.payloadTable.SetColumnWidth(1, 220)
	e.payloadTable.SetColumnWidth(2, 80)
	e.payloadTable.SetColumnWidth(3, 140)
	e.payloadTable.OnSelected = func(id widget.TableCellID) { e.lastSelectedPayloadRow = id.Row }
	e.payloadTable.OnUnselected = func(widget.TableCellID) { e.lastSelectedPayloadRow = -1 }

	addBtn := widget.NewButton("Add File...", func() { e.showPayloadDialog(-1, false) })
	addDirBtn := widget.NewButton("Add Folder...", func() { e.showPayloadDialog(-1, true) })
	editBtn := widget.NewButton("Edit...", func() {
		if row := e.selectedPayloadRow(); row >= 0 {
			e.showPayloadDialog(row, e.proj.Payload[row].Recursive)
		}
	})
	removeBtn := widget.NewButton("Remove", func() {
		if row := e.selectedPayloadRow(); row >= 0 {
			e.proj.Payload = append(e.proj.Payload[:row], e.proj.Payload[row+1:]...)
			e.payloadTable.UnselectAll()
			e.payloadTable.Refresh()
		}
	})
	scanBtn := widget.NewButton("Scan folder...", func() { e.scanPayloadFolder() })

	toolbar := container.NewHBox(addBtn, addDirBtn, editBtn, removeBtn, scanBtn)
	return container.NewBorder(toolbar, nil, nil, nil, e.payloadTable)
}

// selectedPayloadRow returns the selected data row index (0-based into
// e.proj.Payload), or -1 if nothing is selected. widget.Table has no direct
// "currently selected" getter, so buildPayloadTab's OnSelected/OnUnselected
// track it into e.lastSelectedPayloadRow as selection changes.
func (e *editor) selectedPayloadRow() int {
	return e.lastSelectedPayloadRow
}

// newPayloadCell builds one recyclable table cell. widget.Table reuses a
// fixed pool of CanvasObjects across every row and column as the user
// scrolls (see widget.Table's own CreateCell/UpdateCell docs), so a single
// cell object must be able to represent any of this table's columns — an
// Entry for the three free-text columns (Source/Dest/OS) or a Check for
// the boolean Recursive column. Both live in the same stacked container;
// updatePayloadCell shows whichever one the current column needs and hides
// the other.
func newPayloadCell() fyne.CanvasObject {
	return container.NewStack(widget.NewEntry(), widget.NewCheck("", nil))
}

// updatePayloadCell binds one recycled cell (see newPayloadCell) to
// e.proj.Payload[id.Row]'s field for id.Col, editable in place: typing in
// an Entry or toggling the Check writes straight back into e.proj.Payload,
// no Save/dialog step needed. OnChanged is cleared before SetText/
// SetChecked and reassigned after so re-populating a recycled cell for a
// (possibly different) row during scrolling never fires a stale callback
// bound to whatever row the object last represented.
func (e *editor) updatePayloadCell(id widget.TableCellID, obj fyne.CanvasObject) {
	stack := obj.(*fyne.Container)
	entry := stack.Objects[0].(*widget.Entry)
	check := stack.Objects[1].(*widget.Check)
	entry.OnChanged = nil
	check.OnChanged = nil
	entry.Hide()
	check.Hide()

	row := id.Row
	switch id.Col {
	case 0:
		entry.SetText(e.proj.Payload[row].Source)
		entry.OnChanged = func(v string) { e.proj.Payload[row].Source = v }
		entry.Show()
	case 1:
		entry.SetText(e.proj.Payload[row].Dest)
		entry.OnChanged = func(v string) { e.proj.Payload[row].Dest = v }
		entry.Show()
	case 2:
		check.SetChecked(e.proj.Payload[row].Recursive)
		check.OnChanged = func(v bool) { e.proj.Payload[row].Recursive = v }
		check.Show()
	case 3:
		entry.SetText(strings.Join(e.proj.Payload[row].OS, ","))
		entry.OnChanged = func(v string) { e.proj.Payload[row].OS = parseOSFilter(v) }
		entry.Show()
	}
}

// parseOSFilter splits the OS-filter column/field's comma-separated text
// into Payload.OS, trimming whitespace and dropping empty parts — shared by
// the inline table cell above and showPayloadDialog below so the two never
// drift into parsing this differently. A blank filter means "all OSes",
// i.e. nil, not a slice containing "".
func parseOSFilter(text string) []string {
	var os []string
	for _, part := range strings.Split(text, ",") {
		if part = strings.TrimSpace(part); part != "" {
			os = append(os, part)
		}
	}
	return os
}

// showPayloadDialog opens a small add/edit form for one PayloadEntry. row
// is the index to edit, or -1 to append a new entry; isDir pre-fills
// Recursive (and steers the Browse button to a folder picker) since a
// recursive tree copy only ever makes sense for a directory source.
func (e *editor) showPayloadDialog(row int, isDir bool) {
	var existing packproject.PayloadEntry
	if row >= 0 {
		existing = e.proj.Payload[row]
	}

	sourceEntry := widget.NewEntry()
	sourceEntry.SetText(existing.Source)
	var sourceRow fyne.CanvasObject
	if isDir {
		browse := widget.NewButton("Browse...", func() {
			dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
				if err == nil && u != nil {
					sourceEntry.SetText(u.Path())
				}
			}, e.win).Show()
		})
		sourceRow = container.NewBorder(nil, nil, nil, browse, sourceEntry)
	} else {
		sourceRow = newBrowseFileRow(e.win, sourceEntry)
	}

	destEntry := widget.NewEntry()
	destEntry.SetText(existing.Dest)

	// Auto-fill Dest from Source's own basename as long as the user hasn't
	// typed a Dest of their own — this is what "Scan folder..."/imports
	// already do (see projectscan.ScanPayloadCandidates), but this
	// Add File/Add Folder dialog previously left Dest blank unless someone
	// filled it in by hand. A blank Dest is worse than a merely-imperfect
	// guess: several blank-Dest entries collide as duplicate destinations,
	// which only surfaces later as a confusing "payload dest "" used by
	// both X and Y" error at build/validate time instead of here.
	userEditedDest := existing.Dest != ""
	lastAutoDest := existing.Dest
	autoFillDest := func() {
		if userEditedDest {
			return
		}
		lastAutoDest = defaultPayloadDest(sourceEntry.Text)
		destEntry.SetText(lastAutoDest)
	}
	destEntry.OnChanged = func(v string) {
		if v != lastAutoDest {
			userEditedDest = true
		}
	}
	sourceEntry.OnChanged = func(string) { autoFillDest() }

	recursiveCheck := widget.NewCheck("Copy recursively (source is a folder)", nil)
	recursiveCheck.SetChecked(existing.Recursive || (row < 0 && isDir))
	osEntry := widget.NewEntry()
	osEntry.SetPlaceHolder("blank = all OSes; or windows,darwin,linux")
	osEntry.SetText(strings.Join(existing.OS, ","))

	items := []*widget.FormItem{
		widget.NewFormItem("Source", sourceRow),
		widget.NewFormItem("Dest (relative to install root)", destEntry),
		widget.NewFormItem("", recursiveCheck),
		widget.NewFormItem("OS filter", osEntry),
	}

	title := "Add Payload Entry"
	if row >= 0 {
		title = "Edit Payload Entry"
	}

	dialog.ShowForm(title, "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		dest := destEntry.Text
		if dest == "" {
			// Belt-and-suspenders: autoFillDest keeps this from happening in
			// the normal course of using the dialog, but a still-blank Dest
			// must never actually reach e.proj.Payload — see the comment on
			// userEditedDest above for what this is guarding against.
			dest = defaultPayloadDest(sourceEntry.Text)
		}
		entry := packproject.PayloadEntry{
			Source:    sourceEntry.Text,
			Dest:      dest,
			Recursive: recursiveCheck.Checked,
			OS:        parseOSFilter(osEntry.Text),
		}
		if row >= 0 {
			e.proj.Payload[row] = entry
		} else {
			e.proj.Payload = append(e.proj.Payload, entry)
		}
		e.payloadTable.Refresh()
	}, e.win)
}

// defaultPayloadDest proposes a Dest for a Payload entry from its Source
// path alone — just the base name, same as projectscan.ScanPayloadCandidates
// proposes for a top-level file/folder. Pure and independent of any dialog
// widget so it's directly unit-testable; see showPayloadDialog's own
// comment on why leaving this to chance was a real bug (blank Dest values
// collide as duplicate destinations at validate/build time).
func defaultPayloadDest(source string) string {
	return filepath.Base(strings.TrimRight(source, "/"))
}

func (e *editor) refreshPayloadTab() {
	e.payloadTable.Refresh()
}

// scanPayloadFolder lets the user point at a directory (their project root,
// an assets folder, ...) and proposes one PayloadEntry per top-level
// file/folder found there, skipping anything already covered by
// Identity.LicenseFile/Icons.* or an existing Payload entry. Nothing is
// added to e.proj.Payload until the user confirms via the review dialog —
// unlike Features 1/2's direct auto-fill, payload destinations matter too
// much to add without a look.
func (e *editor) scanPayloadFolder() {
	dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		candidates := projectscan.ScanPayloadCandidates(u.Path(), e.proj)
		if len(candidates) == 0 {
			dialog.ShowInformation("Scan folder", "No new payload entries found in that folder.", e.win)
			return
		}
		e.showPayloadScanReviewDialog(candidates)
	}, e.win).Show()
}

// showPayloadScanReviewDialog lists each scanned candidate with a checkbox
// (include/exclude, checked by default) and an editable Dest field; only
// entries left checked on confirm are appended to e.proj.Payload.
func (e *editor) showPayloadScanReviewDialog(candidates []projectscan.PayloadCandidate) {
	type row struct {
		candidate projectscan.PayloadCandidate
		include   *widget.Check
		dest      *widget.Entry
	}

	rows := make([]row, len(candidates))
	form := container.NewVBox()
	for i, c := range candidates {
		destEntry := widget.NewEntry()
		destEntry.SetText(c.Dest)

		kind := "file"
		if c.Recursive {
			kind = "folder"
		}
		if len(c.OS) > 0 {
			kind += ", " + strings.Join(c.OS, "/") + " only"
		}
		if len(c.Excludes) > 0 {
			kind += ", excludes " + strings.Join(c.Excludes, "; ")
		}
		include := widget.NewCheck(c.Source+" ("+kind+")", nil)
		include.SetChecked(true)

		rows[i] = row{candidate: c, include: include, dest: destEntry}
		form.Add(container.NewBorder(nil, nil, include, nil, destEntry))
	}

	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(500, 320))

	dialog.ShowCustomConfirm("Review Payload Entries", "Add Selected", "Cancel", scroll, func(ok bool) {
		if !ok {
			return
		}
		included := make([]bool, len(rows))
		dests := make([]string, len(rows))
		for i, r := range rows {
			included[i] = r.include.Checked
			dests[i] = r.dest.Text
		}
		e.proj.Payload = append(e.proj.Payload, buildPayloadEntries(candidates, included, dests)...)
		e.refreshPayloadTab()
	}, e.win)
}

// buildPayloadEntries converts scanned candidates into real PayloadEntry
// values, keeping only the ones marked included and using each one's
// (possibly user-edited) dest. Pure logic, independent of any dialog
// widget, so it's directly unit-testable without rendering the review
// dialog itself.
func buildPayloadEntries(candidates []projectscan.PayloadCandidate, included []bool, dests []string) []packproject.PayloadEntry {
	var entries []packproject.PayloadEntry
	for i, c := range candidates {
		if i >= len(included) || !included[i] {
			continue
		}
		dest := c.Dest
		if i < len(dests) {
			dest = dests[i]
		}
		entries = append(entries, packproject.PayloadEntry{
			Source:    c.Source,
			Dest:      dest,
			Recursive: c.Recursive,
			OS:        c.OS,
			Excludes:  c.Excludes,
		})
	}
	return entries
}
