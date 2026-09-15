package main

import (
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

const payloadColCount = 6 // Select | Source | Dest | Recursive | OS filter | Excludes

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
//
// Row selection for Remove/Edit... is its own dedicated Select checkbox
// column, NOT widget.Table's own OnSelected/row-selection mechanism - a
// real bug found via Allan's own hands-on testing (Remove silently did
// nothing) traced to a genuine Fyne limitation: every cell here is filled
// by its own interactive Entry/Check widget, and Fyne's own hit-testing
// (internal/driver.FindObjectAtPositionMatching) always dispatches a
// click to the deepest matching object under the pointer - which is
// always that cell's own widget, never bubbling up to the Table's own
// Tapped/Select. So OnSelected/OnUnselected never fired for this table at
// all once every cell became independently interactive (since 0.3.0's
// inline-editing change) - not something a tweak to that mechanism can
// fix, since the same dispatch rule applies to any cell containing its
// own Tappable/Focusable widget.
func (e *editor) buildPayloadTab() fyne.CanvasObject {
	e.payloadTable = widget.NewTable(
		func() (int, int) { return len(e.proj.Payload), payloadColCount },
		newPayloadCell,
		e.updatePayloadCell,
	)
	e.payloadTable.ShowHeaderRow = true
	// Headers are Buttons, not Labels, so Source/Dest/OS can be clicked to
	// sort - see togglePayloadSort/sortPayloadRows below. Select/Recursive/
	// Excludes get the same Button (widget.Table recycles one header object
	// per column position, so every header must be the same widget type)
	// but with OnTapped left nil, so clicking them does nothing.
	e.payloadTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewButton("", nil)
	}
	e.payloadTable.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		btn := obj.(*widget.Button)
		col := id.Col
		title := []string{"Select", "Source", "Dest", "Recursive", "OS", "Excludes"}[col]
		if col == e.payloadSortCol {
			if e.payloadSortAsc {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}
		btn.SetText(title)
		if col == 1 || col == 2 || col == 4 {
			btn.OnTapped = func() { e.togglePayloadSort(col) }
		} else {
			btn.OnTapped = nil
		}
	}
	e.payloadTable.SetColumnWidth(0, 60)
	e.payloadTable.SetColumnWidth(1, 260)
	e.payloadTable.SetColumnWidth(2, 220)
	e.payloadTable.SetColumnWidth(3, 80)
	e.payloadTable.SetColumnWidth(4, 140)
	// Column 5 (Excludes) is set by stretchLastColumnLayout below, not a
	// fixed width here - it stretches to fill whatever's left of the
	// window instead of leaving dead space and forcing a horizontal
	// scrollbar on a wide window.

	addBtn := widget.NewButton("Add File...", func() { e.showPayloadDialog(-1, false) })
	addDirBtn := widget.NewButton("Add Folder...", func() { e.showPayloadDialog(-1, true) })
	editBtn := widget.NewButton("Edit...", func() { e.editSelectedPayload() })
	removeBtn := widget.NewButton("Remove", func() { e.removeSelectedPayload() })
	scanBtn := widget.NewButton("Scan folder...", func() { e.scanPayloadFolder() })

	toolbar := container.NewHBox(addBtn, addDirBtn, editBtn, removeBtn, scanBtn)
	tableArea := container.New(newStretchLastColumnLayout(e.payloadTable, 60, 260, 220, 80, 140), e.payloadTable)
	return container.NewBorder(toolbar, nil, nil, nil, tableArea)
}

// onlySelectedRow returns the one row index set true in selected, or -1 if
// zero or more than one are - for an action like Edit... that only makes
// sense against exactly one row, unlike Remove which acts on any number.
func onlySelectedRow(selected map[int]bool) int {
	row := -1
	for r, checked := range selected {
		if !checked {
			continue
		}
		if row != -1 {
			return -1 // more than one selected
		}
		row = r
	}
	return row
}

// editSelectedPayload opens the Add/Edit dialog against the one row
// currently checked in the Select column, or shows an information dialog
// if zero or more than one are checked - editing more than one entry at
// once through a single form doesn't make sense, unlike Remove below.
func (e *editor) editSelectedPayload() {
	row := onlySelectedRow(e.payloadSelected)
	if row < 0 {
		dialog.ShowInformation("Edit Payload Entry", "Check exactly one row's Select box first.", e.win)
		return
	}
	e.showPayloadDialog(row, e.proj.Payload[row].Recursive)
}

// removeSelectedPayload deletes every row currently checked in the Select
// column, clearing the selection afterward - a no-op if nothing is
// checked. Extracted as its own method (rather than an inline button
// closure) so it's directly unit-testable without needing to dig a
// *widget.Button out of buildPayloadTab's returned widget tree.
func (e *editor) removeSelectedPayload() {
	if len(e.payloadSelected) == 0 {
		return
	}
	kept := make([]packproject.PayloadEntry, 0, len(e.proj.Payload))
	for i, entry := range e.proj.Payload {
		if !e.payloadSelected[i] {
			kept = append(kept, entry)
		}
	}
	e.proj.Payload = kept
	e.payloadSelected = make(map[int]bool)
	e.payloadTable.Refresh()
}

// togglePayloadSort is wired to a header button's OnTapped for the three
// sortable Payload columns (Source=1, Dest=2, OS=4) - see buildPayloadTab.
// Clicking the currently-sorted column reverses direction; clicking a
// different column jumps straight to ascending on the new one.
func (e *editor) togglePayloadSort(col int) {
	if e.payloadSortCol == col {
		e.payloadSortAsc = !e.payloadSortAsc
	} else {
		e.payloadSortCol = col
		e.payloadSortAsc = true
	}
	e.sortPayloadRows()
}

// sortPayloadRows reorders e.proj.Payload itself by the current
// payloadSortCol/payloadSortAsc - a real, persisted reorder rather than a
// display-only overlay: comment-preservation (internal/packproject/
// comments.go) matches entries by content, not position, so a hand-typed #
// comment on a Payload line stays attached to its own entry across a sort.
// Any current Select-column checks are cleared, since remembering "row 3
// was checked" across a reorder would silently start referring to a
// different entry.
func (e *editor) sortPayloadRows() {
	key := func(entry packproject.PayloadEntry) string {
		switch e.payloadSortCol {
		case 1:
			return entry.Source
		case 2:
			return entry.Dest
		case 4:
			return strings.Join(entry.OS, ",")
		default:
			return ""
		}
	}
	sort.SliceStable(e.proj.Payload, func(i, j int) bool {
		a, b := key(e.proj.Payload[i]), key(e.proj.Payload[j])
		if e.payloadSortAsc {
			return a < b
		}
		return a > b
	})
	e.payloadSelected = make(map[int]bool)
	e.payloadTable.Refresh()
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

	// PlaceHolder is a static property of the recycled Entry, not tied to
	// whichever row it's currently bound to - reset per column on every
	// bind (not just once in newPayloadCell) since the very same Entry
	// object gets reused across Source/Dest/OS columns as the user
	// scrolls, and a Dest hint left over from a prior bind would
	// otherwise show up on a Source/OS cell too.
	entry.SetPlaceHolder("")

	row := id.Row
	switch id.Col {
	case 0:
		check.SetChecked(e.payloadSelected[row])
		check.OnChanged = func(v bool) {
			if v {
				e.payloadSelected[row] = true
			} else {
				delete(e.payloadSelected, row)
			}
		}
		check.Show()
	case 1:
		entry.SetText(e.proj.Payload[row].Source)
		entry.OnChanged = func(v string) { e.proj.Payload[row].Source = v }
		entry.Show()
	case 2:
		entry.SetPlaceHolder("(install root)")
		entry.SetText(e.proj.Payload[row].Dest)
		entry.OnChanged = func(v string) { e.proj.Payload[row].Dest = v }
		entry.Show()
	case 3:
		check.SetChecked(e.proj.Payload[row].Recursive)
		check.OnChanged = func(v bool) { e.proj.Payload[row].Recursive = v }
		check.Show()
	case 4:
		entry.SetPlaceHolder("all OSes (e.g. linux/amd64, mac/arm64)")
		entry.SetText(strings.Join(e.proj.Payload[row].OS, ","))
		entry.OnChanged = func(v string) { e.proj.Payload[row].OS = parseOSFilter(v) }
		entry.Show()
	case 5:
		entry.SetPlaceHolder("none (or e.g. *.tmp,mesa-win/*)")
		entry.SetText(strings.Join(e.proj.Payload[row].Excludes, ","))
		entry.OnChanged = func(v string) { e.proj.Payload[row].Excludes = parseCommaList(v) }
		entry.Show()
	}
}

// parseOSFilter splits the OS-filter column/field's comma-separated text
// into Payload.OS, trimming whitespace and dropping empty parts — shared by
// the inline table cell above and showPayloadDialog below so the two never
// drift into parsing this differently. A blank filter means "all OSes",
// i.e. nil, not a slice containing "".
func parseOSFilter(text string) []string {
	return parseCommaList(text)
}

// parseCommaList splits text on "," into a slice, trimming whitespace and
// dropping empty parts - the same comma-separated shape both the OS-filter
// and Excludes columns/fields use, so parseOSFilter is just this under a
// name that matches its own column. A blank string means "none", i.e.
// nil, not a slice containing "".
func parseCommaList(text string) []string {
	var out []string
	for _, part := range strings.Split(text, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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
	destEntry.SetPlaceHolder("(blank = install root)")

	recursiveCheck := widget.NewCheck("Copy recursively (source is a folder)", nil)
	recursiveCheck.SetChecked(existing.Recursive || (row < 0 && isDir))

	// Auto-fill Dest as long as the user hasn't typed one of their own —
	// this is what "Scan folder..."/imports already do (see
	// projectscan.ScanPayloadCandidates), but this Add File/Add Folder
	// dialog previously left Dest blank unless someone filled it in by
	// hand, which used to risk several blank-Dest entries silently
	// colliding as duplicate destinations (surfacing only later as a
	// confusing error at build/validate time, not here). The proposal
	// itself differs by kind, matching Dest's own documented meaning
	// ("always a destination directory" - see PayloadEntry's own doc
	// comment): a folder's own basename for a Recursive entry (its
	// contents land under a same-named subdirectory, e.g. "images"), but
	// blank - the install root - for a plain file, since the installed
	// filename already comes from Source's own basename automatically.
	// Defaulting a file's Dest to its own basename too (this dialog's
	// real behavior until this fix) silently nested it one level deeper
	// than intended - e.g. a Source of "ReleaseNotes.md" installing at
	// ".../ReleaseNotes.md/ReleaseNotes.md" instead of just
	// ".../ReleaseNotes.md" - found while looking into a related GUI
	// hint request, not something anyone had reported hitting directly.
	userEditedDest := existing.Dest != ""
	lastAutoDest := existing.Dest
	autoFillDest := func() {
		if userEditedDest {
			return
		}
		lastAutoDest = defaultPayloadDest(sourceEntry.Text, recursiveCheck.Checked)
		destEntry.SetText(lastAutoDest)
	}
	destEntry.OnChanged = func(v string) {
		if v != lastAutoDest {
			userEditedDest = true
		}
	}
	sourceEntry.OnChanged = func(string) { autoFillDest() }
	recursiveCheck.OnChanged = func(bool) { autoFillDest() }

	osPicker, readOSList := newPayloadOSPicker(existing.OS)

	excludesEntry := widget.NewEntry()
	excludesEntry.SetPlaceHolder("blank = none; or *.tmp,mesa-win/*")
	excludesEntry.SetText(strings.Join(existing.Excludes, ","))

	items := []*widget.FormItem{
		widget.NewFormItem("Source", sourceRow),
		widget.NewFormItem("Dest (relative to install root)", destEntry),
		widget.NewFormItem("", recursiveCheck),
		widget.NewFormItem("OS filter", osPicker),
		widget.NewFormItem("Excludes", excludesEntry),
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
		if dest == "" && recursiveCheck.Checked {
			// Belt-and-suspenders, folders only: autoFillDest keeps this
			// from happening in the normal course of using the dialog, but
			// a still-blank Dest for a *recursive* entry must never reach
			// e.proj.Payload uncorrected — see the comment on
			// userEditedDest above for what this guards against. A blank
			// Dest for a plain file is the correct, intentional value (the
			// install root), not something to "fix" here.
			dest = defaultPayloadDest(sourceEntry.Text, recursiveCheck.Checked)
		}
		entry := packproject.PayloadEntry{
			Source:    sourceEntry.Text,
			Dest:      dest,
			Recursive: recursiveCheck.Checked,
			OS:        readOSList(),
			Excludes:  parseCommaList(excludesEntry.Text),
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
// path and whether it's a Recursive (folder) entry - matching
// projectscan.ScanPayloadCandidates' own proposal for a top-level
// file/folder. Pure and independent of any dialog widget so it's directly
// unit-testable; see showPayloadDialog's own comment for the full
// reasoning: a folder's own basename for isDir (its contents land under a
// same-named subdirectory), but "" - the install root - for a plain file,
// since a non-recursive entry's installed filename already comes from
// Source's own basename automatically; defaulting a file's Dest to its
// own basename too would nest it one level deeper than intended.
func defaultPayloadDest(source string, isDir bool) string {
	if !isDir {
		return ""
	}
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
			// "/" would be ambiguous here now that an OS entry can itself
			// contain one (an "os/arch" pair, e.g. "windows/arm64") -
			// joining several such entries with "/" too would read as one
			// long, confusing path-like string instead of a list.
			kind += ", " + strings.Join(c.OS, ", ") + " only"
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
