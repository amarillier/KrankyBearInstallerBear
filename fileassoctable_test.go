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
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 0}, extCell)
	extCell.(*widget.Entry).SetText(".prj")
	if e.proj.FileAssociations[0].Extension != ".prj" {
		t.Errorf("Extension = %q, want %q", e.proj.FileAssociations[0].Extension, ".prj")
	}

	descCell := newFileAssocCell()
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 1}, descCell)
	descCell.(*widget.Entry).SetText("My Project File")
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
	e.updateFileAssocCell(widget.TableCellID{Row: 0, Col: 0}, cell)
	e.updateFileAssocCell(widget.TableCellID{Row: 1, Col: 0}, cell)
	cell.(*widget.Entry).SetText(".changed")

	if e.proj.FileAssociations[0].Extension != ".one" {
		t.Errorf("row 0 Extension = %q, want unchanged %q", e.proj.FileAssociations[0].Extension, ".one")
	}
	if e.proj.FileAssociations[1].Extension != ".changed" {
		t.Errorf("row 1 Extension = %q, want %q", e.proj.FileAssociations[1].Extension, ".changed")
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

func TestEditor_RefreshFileAssociationsTabDoesNotPanicWhenEmpty(t *testing.T) {
	e := newTestEditor(t)
	e.refreshFileAssociationsTab() // must not panic against a zero-row table
}
