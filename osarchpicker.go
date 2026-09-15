package main

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packproject"
)

// osarchpicker.go backs the Add/Edit Payload dialog's OS/arch checkbox grid
// (see showPayloadDialog in payloadtable.go), which replaced free-text OS
// typing there per Allan's own request - the OS/arch combination space is
// small and fully known (see knownPayloadOSes/knownPayloadArches below), so
// checkboxes remove the typo/ambiguity risk of typing e.g. "windows/arm64,
// mac" by hand. The inline Payload table's own OS column is unchanged (still
// a free-text Entry) - only the dialog got the picker.

// knownPayloadOSes are the rows the picker offers - the same finite set
// osmatch.go's AppliesToOS understands, spelled with Go's own GOOS names
// since that's what gets written back to installerbear.yaml ("mac"/"macos"
// are accepted when parsing an existing entry, never written back out).
var knownPayloadOSes = []string{"windows", "darwin", "linux"}

// knownPayloadArches are the only two architectures this project's own
// Binaries tab/build backends ever produce, so those are the only ones the
// picker offers a dedicated checkbox for.
var knownPayloadArches = []string{"amd64", "arm64"}

// payloadOSPickerState is the checkbox grid's state: which OS/arch boxes
// are checked, plus any existing OS values that don't fit the grid (an
// exotic arch, a plain typo) - preserved verbatim rather than silently
// dropped when the dialog saves.
type payloadOSPickerState struct {
	anyArch map[string]bool            // OS -> "any arch of this OS" checked
	arch    map[string]map[string]bool // OS -> arch -> checked
	extra   []string                   // existing entries that don't fit the grid, kept as-is
}

// parsePayloadOSPicker classifies an existing Payload entry's OS list into
// checkbox state. "mac"/"macos" are recognized as the darwin alias (same as
// osmatch.go's own matching, via the exported packproject.CanonicalOS) so a
// hand-typed or imported entry using them still lands on the right
// checkbox instead of falling into extra.
func parsePayloadOSPicker(list []string) payloadOSPickerState {
	st := payloadOSPickerState{
		anyArch: make(map[string]bool),
		arch:    make(map[string]map[string]bool),
	}
	for _, os := range knownPayloadOSes {
		st.arch[os] = make(map[string]bool)
	}
	for _, raw := range list {
		osPart, archPart, hasArch := strings.Cut(raw, "/")
		canon := packproject.CanonicalOS(osPart)
		if !slices.Contains(knownPayloadOSes, canon) {
			st.extra = append(st.extra, raw)
			continue
		}
		if !hasArch {
			st.anyArch[canon] = true
			continue
		}
		archCanon := strings.ToLower(strings.TrimSpace(archPart))
		if !slices.Contains(knownPayloadArches, archCanon) {
			st.extra = append(st.extra, raw)
			continue
		}
		st.arch[canon][archCanon] = true
	}
	return st
}

// buildPayloadOSList is parsePayloadOSPicker's inverse: turns checkbox
// state back into a Payload OS list, canonical Go GOOS spelling only (never
// mac/macos - that alias is accepted on read, never written). A checked
// "any arch" box for an OS wins over that OS's individual arch boxes (the
// dialog itself keeps them mutually exclusive - see showPayloadDialog - but
// this stays correct even if that invariant were ever violated). Any
// preserved extra values from the original list are appended unchanged. A
// completely empty result (nothing checked, nothing extra) means "all
// OSes" - the same meaning a blank/empty OS filter already has.
func buildPayloadOSList(st payloadOSPickerState) []string {
	var out []string
	for _, os := range knownPayloadOSes {
		if st.anyArch[os] {
			out = append(out, os)
			continue
		}
		for _, arch := range knownPayloadArches {
			if st.arch[os][arch] {
				out = append(out, os+"/"+arch)
			}
		}
	}
	out = append(out, st.extra...)
	return out
}

// osDisplayName is knownPayloadOSes' own GOOS spelling shown as a friendlier
// label in the picker grid - purely cosmetic, never written to the project.
var osDisplayName = map[string]string{
	"windows": "Windows",
	"darwin":  "macOS",
	"linux":   "Linux",
}

// newPayloadOSPicker builds the OS/arch checkbox grid used in
// showPayloadDialog's "OS filter" form item, seeded from an existing
// Payload entry's OS list (parsePayloadOSPicker), and returns the assembled
// widget plus a closure that reads the checkboxes' live state back into a
// Payload OS list (buildPayloadOSList) at Save time. Each OS row's "Any
// arch" box is kept mutually exclusive with its own amd64/arm64 boxes -
// checking one unchecks the other(s) - since a Payload entry can't
// meaningfully mean both "every arch of windows" and "just windows/amd64"
// at once.
func newPayloadOSPicker(existingOS []string) (fyne.CanvasObject, func() []string) {
	initial := parsePayloadOSPicker(existingOS)

	anyChecks := make(map[string]*widget.Check, len(knownPayloadOSes))
	archChecks := make(map[string]map[string]*widget.Check, len(knownPayloadOSes))

	header := []fyne.CanvasObject{
		widget.NewLabel(""),
		widget.NewLabelWithStyle("Any arch", fyne.TextAlignCenter, fyne.TextStyle{}),
	}
	for _, arch := range knownPayloadArches {
		header = append(header, widget.NewLabelWithStyle(arch, fyne.TextAlignCenter, fyne.TextStyle{}))
	}
	gridObjects := append([]fyne.CanvasObject{}, header...)

	for _, os := range knownPayloadOSes {
		anyCheck := widget.NewCheck("", nil)
		anyCheck.SetChecked(initial.anyArch[os])
		anyChecks[os] = anyCheck

		rowArchChecks := make(map[string]*widget.Check, len(knownPayloadArches))
		archChecks[os] = rowArchChecks
		var archObjs []fyne.CanvasObject
		for _, arch := range knownPayloadArches {
			c := widget.NewCheck("", nil)
			c.SetChecked(initial.arch[os][arch])
			rowArchChecks[arch] = c
			archObjs = append(archObjs, container.NewCenter(c))
		}

		// Mutual exclusivity: checking "Any arch" clears this OS's per-arch
		// boxes, and checking any per-arch box clears "Any arch" - see this
		// func's own doc comment for why.
		anyCheck.OnChanged = func(v bool) {
			if !v {
				return
			}
			for _, c := range rowArchChecks {
				c.SetChecked(false)
			}
		}
		for _, c := range rowArchChecks {
			c.OnChanged = func(v bool) {
				if v {
					anyCheck.SetChecked(false)
				}
			}
		}

		gridObjects = append(gridObjects, widget.NewLabel(osDisplayName[os]), container.NewCenter(anyCheck))
		gridObjects = append(gridObjects, archObjs...)
	}

	grid := container.NewGridWithColumns(2+len(knownPayloadArches), gridObjects...)

	children := []fyne.CanvasObject{grid}
	hint := widget.NewLabel("Nothing checked = applies to all OSes/architectures.")
	hint.TextStyle = fyne.TextStyle{Italic: true}
	children = append(children, hint)
	if len(initial.extra) > 0 {
		extraLabel := widget.NewLabel("Also keeping (not editable here): " + strings.Join(initial.extra, ", "))
		extraLabel.TextStyle = fyne.TextStyle{Italic: true}
		extraLabel.Wrapping = fyne.TextWrapWord
		children = append(children, extraLabel)
	}

	readOSList := func() []string {
		st := payloadOSPickerState{
			anyArch: make(map[string]bool, len(knownPayloadOSes)),
			arch:    make(map[string]map[string]bool, len(knownPayloadOSes)),
			extra:   initial.extra,
		}
		for _, os := range knownPayloadOSes {
			st.anyArch[os] = anyChecks[os].Checked
			st.arch[os] = make(map[string]bool, len(knownPayloadArches))
			for _, arch := range knownPayloadArches {
				st.arch[os][arch] = archChecks[os][arch].Checked
			}
		}
		return buildPayloadOSList(st)
	}

	return container.NewVBox(children...), readOSList
}
