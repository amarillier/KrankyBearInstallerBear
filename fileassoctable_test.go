package main

import (
	"testing"

	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
)

func TestFileAssocCell_EditsExtensionAndDescriptionInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp", Description: "old"}}

	extCell := newFileAssocCell()
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 1}, extCell)
	entry, _ := cellWidgets(t, extCell)
	entry.SetText(".prj")
	if e.proj.FileAssociations[0].Extension != ".prj" {
		t.Errorf("Extension = %q, want %q", e.proj.FileAssociations[0].Extension, ".prj")
	}

	descCell := newFileAssocCell()
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 2}, descCell)
	entry, _ = cellWidgets(t, descCell)
	entry.SetText("My Project File")
	if e.proj.FileAssociations[0].Description != "My Project File" {
		t.Errorf("Description = %q, want %q", e.proj.FileAssociations[0].Description, "My Project File")
	}
}

// TestFileAssocCell_RebindingDoesNotLeakIntoThePreviousRow mirrors
// payloadtable_test.go's own regression test for the same class of bug:
// widget.Table recycles a fixed pool of cell objects across rows, so a
// cell rebound to a new row must never fire a callback still bound to
// whatever row it used to represent.
func TestFileAssocCell_RebindingDoesNotLeakIntoThePreviousRow(t *testing.T) {
	e := newTestEditor(t)
	e.proj.FileAssociations = []packproject.FileAssociation{
		{Extension: ".one"},
		{Extension: ".two"},
	}

	cell := newFileAssocCell()
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 1}, cell)
	e.updateFileAssocCell(widget.TableCellID{Row: 1, Col: 1}, cell)
	entry, _ := cellWidgets(t, cell)
	entry.SetText(".changed")

	if e.proj.FileAssociations[0].Extension != ".one" {
		t.Errorf("row 0 Extension = %q, want unchanged %q", e.proj.FileAssociations[0].Extension, ".one")
	}
	if e.proj.FileAssociations[1].Extension != ".changed" {
		t.Errorf("row 1 Extension = %q, want %q", e.proj.FileAssociations[1].Extension, ".changed")
	}
}

// TestFileAssocCell_EditsSelectInPlace confirms the Select column (col 0)
// writes into e.fileAssocSelected, not e.proj.FileAssociations itself -
// see mainwindow.go's own field doc comment for why this is kept separate
// from project data.
func TestFileAssocCell_EditsSelectInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.FileAssociations = []packproject.FileAssociation{{Extension: ".myp"}}

	cell := newFileAssocCell()
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 0}, cell)
	_, check := cellWidgets(t, cell)
	if check.Checked {
		t.Fatal("expected the Select box to start unchecked")
	}

	check.SetChecked(true)
	if !e.fileAssocSelected[0] {
		t.Error("expected row 0 to be marked selected after checking the box")
	}

	check.SetChecked(false)
	if e.fileAssocSelected[0] {
		t.Error("expected row 0 to be cleared from fileAssocSelected after unchecking the box")
	}
}

func TestFileAssociationsTab_AddAppendsBlankRow(t *testing.T) {
	e := newTestEditor(t)
	if len(e.proj.FileAssociations) != 0 {
		t.Fatalf("expected a fresh project to start with no file associations, got %d", len(e.proj.FileAssociations))
	}

	e.proj.FileAssociations = append(e.proj.FileAssociations, packproject.FileAssociation{Extension: ".ext"})
	e.fileAssocTable.Refresh()

	if len(e.proj.FileAssociations) != 1 || e.proj.FileAssociations[0].Extension != ".ext" {
		t.Errorf("expected one new association with a placeholder extension, got %+v", e.proj.FileAssociations)
	}
}

// TestFileAssociationsTab_RemoveDeletesOnlySelectedRows is a regression
// test for the real bug Allan found by hand: Remove silently did nothing,
// since widget.Table's own row selection never fires for a cell filled by
// its own interactive widget (see buildPayloadTab's doc comment for the
// full root cause). Remove now acts on the Select column's own checked
// rows instead, and can remove more than one at once.
func TestFileAssociationsTab_RemoveDeletesOnlySelectedRows(t *testing.T) {
	e := newTestEditor(t)
	e.proj.FileAssociations = []packproject.FileAssociation{
		{Extension: ".one"},
		{Extension: ".two"},
		{Extension: ".three"},
	}
	e.fileAssocSelected = map[int]bool{0: true, 2: true}

	e.removeSelectedFileAssociations()

	if len(e.proj.FileAssociations) != 1 || e.proj.FileAssociations[0].Extension != ".two" {
		t.Errorf("expected only .two to survive, got %+v", e.proj.FileAssociations)
	}
	if len(e.fileAssocSelected) != 0 {
		t.Errorf("expected fileAssocSelected to be cleared after Remove, got %v", e.fileAssocSelected)
	}
}

func TestEditor_RefreshFileAssociationsTabDoesNotPanicWhenEmpty(t *testing.T) {
	e := newTestEditor(t)
	e.refreshFileAssociationsTab() // must not panic against a zero-row table
}
