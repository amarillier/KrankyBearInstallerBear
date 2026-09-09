package main

import (
	"testing"

	"installerbear/internal/projectscan"
)

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
