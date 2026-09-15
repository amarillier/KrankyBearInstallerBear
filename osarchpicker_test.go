package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestParsePayloadOSPicker(t *testing.T) {
	st := parsePayloadOSPicker([]string{"windows", "darwin/amd64", "mac/arm64", "linux/amd64"})

	if !st.anyArch["windows"] {
		t.Errorf("expected bare \"windows\" to set anyArch[windows]")
	}
	if !st.arch["darwin"]["amd64"] {
		t.Errorf("expected darwin/amd64 checked")
	}
	if !st.arch["darwin"]["arm64"] {
		t.Errorf("expected mac/arm64 to resolve to darwin/arm64 via the mac alias")
	}
	if !st.arch["linux"]["amd64"] {
		t.Errorf("expected linux/amd64 checked")
	}
	if st.arch["linux"]["arm64"] {
		t.Errorf("did not expect linux/arm64 to be set")
	}
	if len(st.extra) != 0 {
		t.Errorf("expected no leftover extras, got %v", st.extra)
	}
}

// TestParsePayloadOSPicker_PreservesUnrecognizedEntries confirms a value
// that doesn't fit the picker's known OS/arch grid (an exotic arch, a
// typo, a future GOOS this app doesn't know about) is kept verbatim rather
// than silently dropped when the dialog re-saves the entry.
func TestParsePayloadOSPicker_PreservesUnrecognizedEntries(t *testing.T) {
	st := parsePayloadOSPicker([]string{"linux/riscv64", "freebsd", "windows/amd64"})

	want := []string{"linux/riscv64", "freebsd"}
	sort.Strings(st.extra)
	sort.Strings(want)
	if !reflect.DeepEqual(st.extra, want) {
		t.Errorf("extra = %v, want %v", st.extra, want)
	}
	if !st.arch["windows"]["amd64"] {
		t.Errorf("expected windows/amd64 still recognized alongside the extras")
	}
}

func TestBuildPayloadOSList_RoundTripsThroughParse(t *testing.T) {
	original := []string{"windows", "darwin/amd64"}
	st := parsePayloadOSPicker(original)
	got := buildPayloadOSList(st)

	sort.Strings(got)
	want := []string{"darwin/amd64", "windows"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBuildPayloadOSList_AnyArchWinsOverIndividualArches documents
// buildPayloadOSList's own tie-break (the picker's UI keeps these mutually
// exclusive, but the pure function stays correct even if that invariant
// were ever violated).
func TestBuildPayloadOSList_AnyArchWinsOverIndividualArches(t *testing.T) {
	st := payloadOSPickerState{
		anyArch: map[string]bool{"linux": true},
		arch:    map[string]map[string]bool{"linux": {"amd64": true, "arm64": true}},
	}
	got := buildPayloadOSList(st)
	want := []string{"linux"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBuildPayloadOSList_EmptyMeansAllOSes confirms nothing checked and no
// extras produces a nil/empty OS list - the same "applies to every OS"
// meaning a blank OS filter always had.
func TestBuildPayloadOSList_EmptyMeansAllOSes(t *testing.T) {
	got := buildPayloadOSList(parsePayloadOSPicker(nil))
	if len(got) != 0 {
		t.Errorf("expected an empty list, got %v", got)
	}
}

func TestBuildPayloadOSList_PreservesExtrasAlongsideCheckedBoxes(t *testing.T) {
	st := parsePayloadOSPicker([]string{"linux/riscv64", "windows/amd64"})
	got := buildPayloadOSList(st)

	sort.Strings(got)
	want := []string{"linux/riscv64", "windows/amd64"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
