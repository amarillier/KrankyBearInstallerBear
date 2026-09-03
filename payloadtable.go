package main

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

const payloadColCount = 4 // Source | Dest | Recursive | OS filter

// buildPayloadTab shows Project.Payload read-only in a table — editing goes
// through the add/edit dialog below rather than inline-editable cells,
// which would need a lot more Fyne plumbing for not much benefit at this
// tool's "basics" scope. This is the GUI analogue of today's Inno [Files]
// section / fpm src=dest arguments.
func (e *editor) buildPayloadTab() fyne.CanvasObject {
	e.payloadTable = widget.NewTable(
		func() (int, int) { return len(e.proj.Payload) + 1, payloadColCount }, // +1 header row
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText([]string{"Source", "Dest", "Recursive", "OS"}[id.Col])
				return
			}
			label.TextStyle = fyne.TextStyle{}
			entry := e.proj.Payload[id.Row-1]
			switch id.Col {
			case 0:
				label.SetText(entry.Source)
			case 1:
				label.SetText(entry.Dest)
			case 2:
				label.SetText(strconv.FormatBool(entry.Recursive))
			case 3:
				if len(entry.OS) == 0 {
					label.SetText("all")
				} else {
					label.SetText(strings.Join(entry.OS, ", "))
				}
			}
		},
	)
	e.payloadTable.SetColumnWidth(0, 260)
	e.payloadTable.SetColumnWidth(1, 220)
	e.payloadTable.SetColumnWidth(2, 80)
	e.payloadTable.SetColumnWidth(3, 140)
	e.payloadTable.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 { // header row isn't a real entry
			e.payloadTable.UnselectAll()
			e.lastSelectedPayloadRow = -1
			return
		}
		e.lastSelectedPayloadRow = id.Row - 1
	}
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
		entry := packproject.PayloadEntry{
			Source:    sourceEntry.Text,
			Dest:      destEntry.Text,
			Recursive: recursiveCheck.Checked,
		}
		if osFilter := strings.TrimSpace(osEntry.Text); osFilter != "" {
			for _, part := range strings.Split(osFilter, ",") {
				if part = strings.TrimSpace(part); part != "" {
					entry.OS = append(entry.OS, part)
				}
			}
		}
		if row >= 0 {
			e.proj.Payload[row] = entry
		} else {
			e.proj.Payload = append(e.proj.Payload, entry)
		}
		e.payloadTable.Refresh()
	}, e.win)
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
		})
	}
	return entries
}
