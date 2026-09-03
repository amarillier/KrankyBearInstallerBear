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
