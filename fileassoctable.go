package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
)

const fileAssocColCount = 2 // Extension | Description

// buildFileAssociationsTab shows Project.FileAssociations in an
// inline-editable table, the same cell-recycling pattern payloadtable.go
// already established (see its own doc comment on why OnChanged must be
// reset before repopulating a recycled cell). Deliberately simpler than
// the Payload table - both columns are plain text, so there's no need
// for a separate Add/Edit dialog or a mixed Entry/Check cell type; Add
// just appends a blank row directly into the table for in-place editing.
func (e *editor) buildFileAssociationsTab() fyne.CanvasObject {
	e.fileAssocTable = widget.NewTable(
		func() (int, int) { return len(e.proj.FileAssociations), fileAssocColCount },
		newFileAssocCell,
		e.updateFileAssocCell,
	)
	e.fileAssocTable.ShowHeaderRow = true
	e.fileAssocTable.CreateHeader = func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.TextStyle = fyne.TextStyle{Bold: true}
		return l
	}
	e.fileAssocTable.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		obj.(*widget.Label).SetText([]string{"Extension", "Description"}[id.Col])
	}
	e.fileAssocTable.SetColumnWidth(0, 120)
	e.fileAssocTable.SetColumnWidth(1, 360)
	e.fileAssocTable.OnSelected = func(id widget.TableCellID) { e.lastSelectedFileAssocRow = id.Row }
	e.fileAssocTable.OnUnselected = func(widget.TableCellID) { e.lastSelectedFileAssocRow = -1 }

	addBtn := widget.NewButton("Add", func() {
		e.proj.FileAssociations = append(e.proj.FileAssociations, packproject.FileAssociation{Extension: ".ext"})
		e.fileAssocTable.Refresh()
	})
	removeBtn := widget.NewButton("Remove", func() {
		if row := e.lastSelectedFileAssocRow; row >= 0 && row < len(e.proj.FileAssociations) {
			e.proj.FileAssociations = append(e.proj.FileAssociations[:row], e.proj.FileAssociations[row+1:]...)
			e.lastSelectedFileAssocRow = -1
			e.fileAssocTable.UnselectAll()
			e.fileAssocTable.Refresh()
		}
	})

	note := widget.NewLabel("Double-clicking a file with this extension opens it in this app. Supported on Windows and Linux; macOS can't support this without a hand-placed Info.plist (see README).")
	note.Wrapping = fyne.TextWrapWord

	toolbar := container.NewHBox(addBtn, removeBtn)
	return container.NewBorder(container.NewVBox(toolbar, note), nil, nil, nil, e.fileAssocTable)
}

// newFileAssocCell builds one recyclable table cell - a plain Entry
// suffices for both columns (Extension/Description), unlike the Payload
// table's mixed Entry/Check cell, since neither field here is a boolean.
func newFileAssocCell() fyne.CanvasObject {
	return widget.NewEntry()
}

// updateFileAssocCell binds one recycled cell to
// e.proj.FileAssociations[id.Row]'s field for id.Col. OnChanged is reset
// before SetText and reassigned after, same reason as
// payloadtable.go's updatePayloadCell: a recycled cell must never fire a
// stale callback bound to whatever row it used to represent.
func (e *editor) updateFileAssocCell(id widget.TableCellID, obj fyne.CanvasObject) {
	entry := obj.(*widget.Entry)
	entry.OnChanged = nil

	row := id.Row
	switch id.Col {
	case 0:
		entry.SetText(e.proj.FileAssociations[row].Extension)
		entry.OnChanged = func(v string) { e.proj.FileAssociations[row].Extension = v }
	case 1:
		entry.SetText(e.proj.FileAssociations[row].Description)
		entry.OnChanged = func(v string) { e.proj.FileAssociations[row].Description = v }
	}
}

func (e *editor) refreshFileAssociationsTab() {
	e.fileAssocTable.Refresh()
}
