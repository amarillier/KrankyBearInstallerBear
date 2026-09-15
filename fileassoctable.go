package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
)

const fileAssocColCount = 3 // Select | Extension | Description

// buildFileAssociationsTab shows Project.FileAssociations in an
// inline-editable table, the same cell-recycling pattern payloadtable.go
// already established (see its own doc comment on why OnChanged must be
// reset before repopulating a recycled cell). Row selection for Remove is
// its own dedicated Select checkbox column, not widget.Table's own
// OnSelected/row-selection - see buildPayloadTab's own doc comment for
// why that mechanism never fires at all once every cell is its own
// interactive widget (a real bug found via Allan's own hands-on testing,
// fixed here the same way).
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
		obj.(*widget.Label).SetText([]string{"Select", "Extension", "Description"}[id.Col])
	}
	e.fileAssocTable.SetColumnWidth(0, 60)
	e.fileAssocTable.SetColumnWidth(1, 120)
	e.fileAssocTable.SetColumnWidth(2, 360)

	addBtn := widget.NewButton("Add", func() {
		e.proj.FileAssociations = append(e.proj.FileAssociations, packproject.FileAssociation{Extension: ".ext"})
		e.fileAssocTable.Refresh()
	})
	removeBtn := widget.NewButton("Remove", func() { e.removeSelectedFileAssociations() })

	note := widget.NewLabel("Double-clicking a file with this extension opens it in this app. Supported on Windows and Linux; macOS can't support this without a hand-placed Info.plist (see README).")
	note.Wrapping = fyne.TextWrapWord

	toolbar := container.NewHBox(addBtn, removeBtn)
	return container.NewBorder(container.NewVBox(toolbar, note), nil, nil, nil, e.fileAssocTable)
}

// removeSelectedFileAssociations deletes every row currently checked in
// the Select column, clearing the selection afterward - a no-op if
// nothing is checked. Extracted as its own method (rather than an inline
// button closure) so it's directly unit-testable without needing to dig
// a *widget.Button out of buildFileAssociationsTab's returned widget tree.
func (e *editor) removeSelectedFileAssociations() {
	if len(e.fileAssocSelected) == 0 {
		return
	}
	kept := make([]packproject.FileAssociation, 0, len(e.proj.FileAssociations))
	for i, assoc := range e.proj.FileAssociations {
		if !e.fileAssocSelected[i] {
			kept = append(kept, assoc)
		}
	}
	e.proj.FileAssociations = kept
	e.fileAssocSelected = make(map[int]bool)
	e.fileAssocTable.Refresh()
}

// newFileAssocCell builds one recyclable table cell - an Entry for the two
// text columns (Extension/Description), or a Check for the Select column,
// the same stacked Entry+Check shape payloadtable.go's own newPayloadCell
// uses (see its own doc comment).
func newFileAssocCell() fyne.CanvasObject {
	return container.NewStack(widget.NewEntry(), widget.NewCheck("", nil))
}

// updateFileAssocCell binds one recycled cell to
// e.proj.FileAssociations[id.Row]'s field for id.Col (or, for the Select
// column, to e.fileAssocSelected[id.Row] - see mainwindow.go's own field
// doc comment for why that's separate from the project data itself).
// OnChanged is reset before SetText/SetChecked and reassigned after, same
// reason as payloadtable.go's updatePayloadCell: a recycled cell must
// never fire a stale callback bound to whatever row it used to represent.
func (e *editor) updateFileAssocCell(id widget.TableCellID, obj fyne.CanvasObject) {
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
		check.SetChecked(e.fileAssocSelected[row])
		check.OnChanged = func(v bool) {
			if v {
				e.fileAssocSelected[row] = true
			} else {
				delete(e.fileAssocSelected, row)
			}
		}
		check.Show()
	case 1:
		entry.SetText(e.proj.FileAssociations[row].Extension)
		entry.OnChanged = func(v string) { e.proj.FileAssociations[row].Extension = v }
		entry.Show()
	case 2:
		entry.SetText(e.proj.FileAssociations[row].Description)
		entry.OnChanged = func(v string) { e.proj.FileAssociations[row].Description = v }
		entry.Show()
	}
}

func (e *editor) refreshFileAssociationsTab() {
	e.fileAssocTable.Refresh()
}
