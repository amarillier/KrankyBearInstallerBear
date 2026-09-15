package main

import "testing"

func TestLastColumnWidth_FillsRemainingSpace(t *testing.T) {
	got := lastColumnWidth(1000, []float32{260, 220, 80, 140}, 120)
	want := float32(1000 - 260 - 220 - 80 - 140) // 300
	if got != want {
		t.Errorf("lastColumnWidth() = %v, want %v", got, want)
	}
}

func TestLastColumnWidth_FloorsAtMinimumOnNarrowWindow(t *testing.T) {
	// Fixed columns alone (260+220+80+140=700) already exceed the total
	// width - the last column must never go to zero or negative.
	got := lastColumnWidth(500, []float32{260, 220, 80, 140}, 120)
	if got != 120 {
		t.Errorf("lastColumnWidth() = %v, want the 120 floor", got)
	}
}

func TestLastColumnWidth_ExactlyAtFloorBoundary(t *testing.T) {
	// totalWidth leaves exactly minLastCol remaining - should return that
	// exact value, not push below it.
	got := lastColumnWidth(700+120, []float32{260, 220, 80, 140}, 120)
	if got != 120 {
		t.Errorf("lastColumnWidth() = %v, want 120", got)
	}
}

func TestLastColumnWidth_NoFixedColumns(t *testing.T) {
	got := lastColumnWidth(500, nil, 120)
	if got != 500 {
		t.Errorf("lastColumnWidth() = %v, want the full width (500) when there are no fixed columns", got)
	}
}
