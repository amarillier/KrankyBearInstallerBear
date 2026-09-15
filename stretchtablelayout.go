package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// stretchLastColumnLayout wraps a *widget.Table so its own last column
// stretches to fill whatever width is left over once every other column's
// fixed width is subtracted, instead of leaving unused space to the right
// (and forcing a horizontal scrollbar even when the window is plenty wide
// enough to show everything) - Allan's own report, on the Payload tab's
// new Excludes column specifically, but written generically since
// widget.Table itself has no "stretch"/"star-sized" column concept at
// all (SetColumnWidth only ever sets a fixed width; confirmed by reading
// Fyne v2.8.1's own table.go).
//
// Only the last column grows; every other column keeps the width it was
// given, matching the common "last column fills the rest" convention
// (e.g. a native file-manager's list view) rather than distributing extra
// space across every column, which would make the fixed columns'
// on-screen width unpredictable too.
type stretchLastColumnLayout struct {
	table          *widget.Table
	fixedColWidths []float32 // every column except the last, in order
	minLastCol     float32   // floor so the last column never collapses to nothing on a narrow window
}

// newStretchLastColumnLayout returns a layout for table whose columns
// 0..len(fixedColWidths)-1 stay at fixedColWidths and whose last column
// (index len(fixedColWidths)) stretches to fill the rest.
func newStretchLastColumnLayout(table *widget.Table, fixedColWidths ...float32) *stretchLastColumnLayout {
	return &stretchLastColumnLayout{table: table, fixedColWidths: fixedColWidths, minLastCol: 120}
}

func (l *stretchLastColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}

	l.table.SetColumnWidth(len(l.fixedColWidths), lastColumnWidth(size.Width, l.fixedColWidths, l.minLastCol))

	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
}

// lastColumnWidth computes how wide the stretching last column should be:
// whatever's left of totalWidth once every fixedColWidths entry is
// subtracted, floored at minLastCol so it never collapses to nothing (or
// goes negative) on a narrower window than the fixed columns alone need.
// A free function, not a method, so this arithmetic is directly
// unit-testable without needing a real *widget.Table to inspect -
// widget.Table has no public getter for a column's current width at all.
func lastColumnWidth(totalWidth float32, fixedColWidths []float32, minLastCol float32) float32 {
	var fixedTotal float32
	for _, w := range fixedColWidths {
		fixedTotal += w
	}
	remaining := totalWidth - fixedTotal
	if remaining < minLastCol {
		return minLastCol
	}
	return remaining
}

func (l *stretchLastColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}
