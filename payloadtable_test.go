package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

// cellWidgets pulls the Entry and Check out of a cell built by
// newPayloadCell, matching updatePayloadCell's own assumption about that
// container's shape.
func cellWidgets(t *testing.T, obj fyne.CanvasObject) (*widget.Entry, *widget.Check) {
	t.Helper()
	stack, ok := obj.(*fyne.Container)
	if !ok || len(stack.Objects) != 2 {
		t.Fatalf("expected a 2-child container from newPayloadCell, got %#v", obj)
	}
	entry, ok1 := stack.Objects[0].(*widget.Entry)
	check, ok2 := stack.Objects[1].(*widget.Check)
	if !ok1 || !ok2 {
		t.Fatalf("expected [Entry, Check] children, got %#v", stack.Objects)
	}
	return entry, check
}

func TestPayloadCell_EditsSourceAndDestInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "a.txt", Dest: "docs"}}

	sourceCell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 0}, sourceCell)
	entry, _ := cellWidgets(t, sourceCell)
	entry.SetText("b.txt")
	if e.proj.Payload[0].Source != "b.txt" {
		t.Errorf("Source = %q, want %q", e.proj.Payload[0].Source, "b.txt")
	}

	destCell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 1}, destCell)
	entry, _ = cellWidgets(t, destCell)
	entry.SetText("assets")
	if e.proj.Payload[0].Dest != "assets" {
		t.Errorf("Dest = %q, want %q", e.proj.Payload[0].Dest, "assets")
	}
}

func TestPayloadCell_TogglesRecursiveInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "assets", Dest: "assets", Recursive: false}}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 2}, cell)
	_, check := cellWidgets(t, cell)
	if check.Checked {
		t.Fatal("expected the check to start unchecked, matching Recursive: false")
	}

	check.SetChecked(true)
	if !e.proj.Payload[0].Recursive {
		t.Error("expected Recursive to become true after checking the box")
	}
}

func TestPayloadCell_EditsOSFilterInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "a", Dest: "a", OS: []string{"windows"}}}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 3}, cell)
	entry, _ := cellWidgets(t, cell)
	if entry.Text != "windows" {
		t.Fatalf("expected the cell to start populated with %q, got %q", "windows", entry.Text)
	}

	entry.SetText("darwin,linux")
	if got := e.proj.Payload[0].OS; len(got) != 2 || got[0] != "darwin" || got[1] != "linux" {
		t.Errorf("OS = %v, want [darwin linux]", got)
	}
}

// TestPayloadCell_RebindingDoesNotLeakIntoThePreviousRow is a regression
// test for the exact hazard updatePayloadCell's own doc comment calls out:
// widget.Table recycles one cell object across every row as the user
// scrolls, so rebinding that same object to a new row must fully replace
// its OnChanged (via container.NewStack(Entry, Check), reset each call) —
// otherwise a keystroke after scrolling could silently edit whatever row
// the cell used to represent instead of the row it's now bound to.
func TestPayloadCell_RebindingDoesNotLeakIntoThePreviousRow(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{
		{Source: "row0.txt", Dest: "a"},
		{Source: "row1.txt", Dest: "b"},
	}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 0}, cell)
	e.updatePayloadCell(widget.TableCellID{Row: 1, Col: 0}, cell) // recycled for a different row

	entry, _ := cellWidgets(t, cell)
	entry.SetText("edited.txt")

	if e.proj.Payload[0].Source != "row0.txt" {
		t.Errorf("row 0 should be untouched, got Source = %q", e.proj.Payload[0].Source)
	}
	if e.proj.Payload[1].Source != "edited.txt" {
		t.Errorf("row 1 should have received the edit, got Source = %q", e.proj.Payload[1].Source)
	}
}

func TestParseOSFilter(t *testing.T) {
	cases := map[string][]string{
		"":                  nil,
		"windows":           {"windows"},
		" windows, darwin ": {"windows", "darwin"},
		",,":                nil,
	}
	for in, want := range cases {
		got := parseOSFilter(in)
		if len(got) != len(want) {
			t.Errorf("parseOSFilter(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parseOSFilter(%q) = %v, want %v", in, got, want)
				break
			}
		}
	}
}

func TestBuildPayloadEntriesFiltersAndEditsDest(t *testing.T) {
	candidates := []projectscan.PayloadCandidate{
		{Source: "assets", Dest: "assets", Recursive: true},
		{Source: "ReleaseNotes.txt", Dest: "ReleaseNotes.txt", Recursive: false},
		{Source: "LICENSE", Dest: "LICENSE", Recursive: false},
	}
	included := []bool{true, false, true}
	dests := []string{"assets", "ReleaseNotes.txt", "License.txt"} // edited dest for the 3rd entry

	got := buildPayloadEntries(candidates, included, dests)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2 (excluded entry skipped): %+v", len(got), got)
	}
	if got[0].Source != "assets" || !got[0].Recursive {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Source != "LICENSE" || got[1].Dest != "License.txt" {
		t.Errorf("got[1] = %+v, want edited Dest %q", got[1], "License.txt")
	}
}

func TestBuildPayloadEntriesCarriesOSAndExcludes(t *testing.T) {
	candidates := []projectscan.PayloadCandidate{
		{Source: "assets", Dest: "assets", Recursive: true, OS: []string{"windows"}, Excludes: []string{"mesa-win/*"}},
	}
	got := buildPayloadEntries(candidates, []bool{true}, []string{"assets"})
	if len(got) != 1 {
		t.Fatalf("got %+v, want 1 entry", got)
	}
	if len(got[0].OS) != 1 || got[0].OS[0] != "windows" {
		t.Errorf("OS = %v, want [windows]", got[0].OS)
	}
	if len(got[0].Excludes) != 1 || got[0].Excludes[0] != "mesa-win/*" {
		t.Errorf("Excludes = %v, want [mesa-win/*]", got[0].Excludes)
	}
}

func TestBuildPayloadEntriesNoneIncluded(t *testing.T) {
	candidates := []projectscan.PayloadCandidate{
		{Source: "a", Dest: "a"},
		{Source: "b", Dest: "b"},
	}
	got := buildPayloadEntries(candidates, []bool{false, false}, []string{"a", "b"})
	if len(got) != 0 {
		t.Fatalf("got %+v, want none", got)
	}
}

// TestDefaultPayloadDest is a regression test: showPayloadDialog's Add
// File/Add Folder flow used to leave Dest blank unless the user typed one
// by hand, so several Add-ed entries in a row would all get Dest == "" and
// only fail later, at validate/build time, as a confusing "payload dest
// used by both X and Y" duplicate-destination error.
func TestDefaultPayloadDest(t *testing.T) {
	cases := map[string]string{
		"/Users/allan/proj/assets/images":    "images",
		"/Users/allan/proj/ReleaseNotes.txt": "ReleaseNotes.txt",
		"/Users/allan/proj/assets/mesa-win/": "mesa-win",
		"relative/path/LICENSE":              "LICENSE",
	}
	for source, want := range cases {
		if got := defaultPayloadDest(source); got != want {
			t.Errorf("defaultPayloadDest(%q) = %q, want %q", source, got, want)
		}
	}
}
