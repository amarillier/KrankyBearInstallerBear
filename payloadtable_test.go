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

// TestPayloadCell_EditsSelectInPlace confirms the Select column (col 0)
// writes into e.payloadSelected, not e.proj.Payload itself - see
// mainwindow.go's own field doc comment for why this is kept separate
// from project data.
func TestPayloadCell_EditsSelectInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "a.txt"}}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 0}, cell)
	_, check := cellWidgets(t, cell)
	if check.Checked {
		t.Fatal("expected the Select box to start unchecked")
	}

	check.SetChecked(true)
	if !e.payloadSelected[0] {
		t.Error("expected row 0 to be marked selected after checking the box")
	}

	check.SetChecked(false)
	if e.payloadSelected[0] {
		t.Error("expected row 0 to be cleared from payloadSelected after unchecking the box")
	}
}

func TestPayloadCell_EditsSourceAndDestInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "a.txt", Dest: "docs"}}

	sourceCell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 1}, sourceCell)
	entry, _ := cellWidgets(t, sourceCell)
	entry.SetText("b.txt")
	if e.proj.Payload[0].Source != "b.txt" {
		t.Errorf("Source = %q, want %q", e.proj.Payload[0].Source, "b.txt")
	}

	destCell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 2}, destCell)
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
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 3}, cell)
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
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 4}, cell)
	entry, _ := cellWidgets(t, cell)
	if entry.Text != "windows" {
		t.Fatalf("expected the cell to start populated with %q, got %q", "windows", entry.Text)
	}

	entry.SetText("darwin,linux")
	if got := e.proj.Payload[0].OS; len(got) != 2 || got[0] != "darwin" || got[1] != "linux" {
		t.Errorf("OS = %v, want [darwin linux]", got)
	}
}

func TestPayloadCell_EditsExcludesInPlace(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "assets", Dest: "assets", Excludes: []string{"mesa-win/*"}}}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 5}, cell)
	entry, _ := cellWidgets(t, cell)
	if entry.Text != "mesa-win/*" {
		t.Fatalf("expected the cell to start populated with %q, got %q", "mesa-win/*", entry.Text)
	}

	entry.SetText("*.tmp,*.bak")
	if got := e.proj.Payload[0].Excludes; len(got) != 2 || got[0] != "*.tmp" || got[1] != "*.bak" {
		t.Errorf("Excludes = %v, want [*.tmp *.bak]", got)
	}
}

// TestPayloadCell_DestPlaceholderDoesNotLeakIntoOtherColumns is a
// regression test for the same recycled-cell hazard as
// TestPayloadCell_RebindingDoesNotLeakIntoThePreviousRow, but for
// PlaceHolder specifically: it's a static property of the recycled Entry,
// not tied to any one row/column, so binding a cell to the Dest column
// (which sets a "(install root)" placeholder) and then rebinding the same
// object to Source must not leave that placeholder behind.
func TestPayloadCell_DestPlaceholderDoesNotLeakIntoOtherColumns(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "a.txt", Dest: ""}}

	cell := newPayloadCell()
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 2}, cell) // Dest - sets a placeholder
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 1}, cell) // recycled for Source

	entry, _ := cellWidgets(t, cell)
	if entry.PlaceHolder != "" {
		t.Errorf("expected no placeholder leaking onto the Source column, got %q", entry.PlaceHolder)
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
	e.updatePayloadCell(widget.TableCellID{Row: 0, Col: 1}, cell)
	e.updatePayloadCell(widget.TableCellID{Row: 1, Col: 1}, cell) // recycled for a different row

	entry, _ := cellWidgets(t, cell)
	entry.SetText("edited.txt")

	if e.proj.Payload[0].Source != "row0.txt" {
		t.Errorf("row 0 should be untouched, got Source = %q", e.proj.Payload[0].Source)
	}
	if e.proj.Payload[1].Source != "edited.txt" {
		t.Errorf("row 1 should have received the edit, got Source = %q", e.proj.Payload[1].Source)
	}
}

func TestOnlySelectedRow(t *testing.T) {
	if got := onlySelectedRow(map[int]bool{}); got != -1 {
		t.Errorf("empty map: got %d, want -1", got)
	}
	if got := onlySelectedRow(map[int]bool{2: true}); got != 2 {
		t.Errorf("single selected: got %d, want 2", got)
	}
	if got := onlySelectedRow(map[int]bool{1: true, 3: true}); got != -1 {
		t.Errorf("two selected: got %d, want -1", got)
	}
	// A row present in the map but false (unchecked-then-rechecked-false,
	// or a stale entry) must not count as "selected".
	if got := onlySelectedRow(map[int]bool{0: false, 2: true}); got != 2 {
		t.Errorf("one true + one false: got %d, want 2", got)
	}
}

// TestRemoveSelectedPayload_DeletesOnlyCheckedRows is a regression test
// for the real bug Allan found by hand: Remove silently did nothing,
// since widget.Table's own row selection never fires for a cell filled by
// its own interactive widget (see buildPayloadTab's own doc comment for
// the full root cause). Remove now acts on the Select column's own
// checked rows instead, and can remove more than one at once.
func TestRemoveSelectedPayload_DeletesOnlyCheckedRows(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{
		{Source: "one.txt"},
		{Source: "two.txt"},
		{Source: "three.txt"},
	}
	e.payloadSelected = map[int]bool{0: true, 2: true}

	e.removeSelectedPayload()

	if len(e.proj.Payload) != 1 || e.proj.Payload[0].Source != "two.txt" {
		t.Errorf("expected only two.txt to survive, got %+v", e.proj.Payload)
	}
	if len(e.payloadSelected) != 0 {
		t.Errorf("expected payloadSelected to be cleared after Remove, got %v", e.payloadSelected)
	}
}

func TestRemoveSelectedPayload_NoopWhenNothingSelected(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{{Source: "one.txt"}}

	e.removeSelectedPayload()

	if len(e.proj.Payload) != 1 {
		t.Errorf("expected Payload untouched when nothing is selected, got %+v", e.proj.Payload)
	}
}

// TestTogglePayloadSort_SortsBySourceThenReverses covers the header-click
// cycle: clicking a sortable column's header (Source=1) the first time
// sorts ascending, clicking it again reverses to descending, matching the
// two-state cycle documented on togglePayloadSort.
func TestTogglePayloadSort_SortsBySourceThenReverses(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{
		{Source: "charlie.txt"},
		{Source: "alpha.txt"},
		{Source: "bravo.txt"},
	}

	e.togglePayloadSort(1)
	got := []string{e.proj.Payload[0].Source, e.proj.Payload[1].Source, e.proj.Payload[2].Source}
	want := []string{"alpha.txt", "bravo.txt", "charlie.txt"}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("ascending sort: got %v, want %v", got, want)
	}

	e.togglePayloadSort(1)
	got = []string{e.proj.Payload[0].Source, e.proj.Payload[1].Source, e.proj.Payload[2].Source}
	want = []string{"charlie.txt", "bravo.txt", "alpha.txt"}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("descending sort: got %v, want %v", got, want)
	}
}

// TestTogglePayloadSort_SwitchingColumnResetsToAscending confirms clicking
// a *different* sortable column always starts fresh at ascending, rather
// than carrying over the previous column's direction.
func TestTogglePayloadSort_SwitchingColumnResetsToAscending(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{
		{Source: "a.txt", Dest: "z"},
		{Source: "b.txt", Dest: "y"},
	}

	e.togglePayloadSort(1) // Source ascending
	e.togglePayloadSort(1) // Source descending
	e.togglePayloadSort(2) // switch to Dest - must be ascending, not carry descending

	if !e.payloadSortAsc {
		t.Errorf("expected switching to a new column to reset to ascending, got descending")
	}
	if e.proj.Payload[0].Dest != "y" || e.proj.Payload[1].Dest != "z" {
		t.Errorf("expected Dest ascending order [y,z], got %+v", e.proj.Payload)
	}
}

// TestTogglePayloadSort_ClearsSelection guards against a reorder silently
// making an existing Select checkbox refer to a different row than the
// user actually checked.
func TestTogglePayloadSort_ClearsSelection(t *testing.T) {
	e := newTestEditor(t)
	e.proj.Payload = []packproject.PayloadEntry{
		{Source: "b.txt"},
		{Source: "a.txt"},
	}
	e.payloadSelected = map[int]bool{0: true}

	e.togglePayloadSort(1)

	if len(e.payloadSelected) != 0 {
		t.Errorf("expected selection cleared after sort, got %v", e.payloadSelected)
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

// TestDefaultPayloadDest_Folder is a regression test: showPayloadDialog's
// Add Folder flow used to leave Dest blank unless the user typed one by
// hand, so several Add-ed entries in a row would all get Dest == "" and
// only fail later, at validate/build time, as a confusing "payload dest
// used by both X and Y" duplicate-destination error. A folder's own
// basename is still the right default - its contents land under a
// same-named subdirectory.
func TestDefaultPayloadDest_Folder(t *testing.T) {
	cases := map[string]string{
		"/Users/allan/proj/assets/images":    "images",
		"/Users/allan/proj/assets/mesa-win/": "mesa-win",
	}
	for source, want := range cases {
		if got := defaultPayloadDest(source, true); got != want {
			t.Errorf("defaultPayloadDest(%q, true) = %q, want %q", source, got, want)
		}
	}
}

// TestDefaultPayloadDest_FileDefaultsToInstallRoot is a regression test
// for a real bug found via Allan's own screenshot: Add File defaulted
// Dest to the file's own basename (e.g. "branding.go" -> Dest:
// "branding.go"), which silently nests the file one level deeper than
// intended (".../branding.go/branding.go" instead of just
// ".../branding.go") since a non-recursive entry's installed filename
// already comes from Source's own basename automatically. The correct
// default for a plain file is "" (the install root).
func TestDefaultPayloadDest_FileDefaultsToInstallRoot(t *testing.T) {
	cases := []string{
		"/Users/allan/proj/ReleaseNotes.txt",
		"relative/path/LICENSE",
	}
	for _, source := range cases {
		if got := defaultPayloadDest(source, false); got != "" {
			t.Errorf("defaultPayloadDest(%q, false) = %q, want \"\"", source, got)
		}
	}
}
