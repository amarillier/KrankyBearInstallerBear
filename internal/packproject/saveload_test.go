package packproject

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestSaveLoad_PayloadExcludesRoundTrip confirms a PayloadEntry.Excludes
// value survives a real Save/Load cycle through YAML, not just an
// in-memory struct — the schema field is only useful if it actually
// persists to installerbear.yaml and back.
func TestSaveLoad_PayloadExcludesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "installerbear.yaml")

	proj := &Project{
		Identity: Identity{Name: "Test App", ID: "com.example.testapp", Version: "1.0.0"},
		Payload: []PayloadEntry{
			{Source: "assets/images", Dest: "assets/images", Recursive: true, Excludes: []string{"mesa-win/*", "*.tmp"}},
		},
	}

	if err := Save(proj, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := LoadLenient(path)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}

	if len(got.Payload) != 1 {
		t.Fatalf("got %d payload entries, want 1: %+v", len(got.Payload), got.Payload)
	}
	want := []string{"mesa-win/*", "*.tmp"}
	if !reflect.DeepEqual(got.Payload[0].Excludes, want) {
		t.Errorf("Excludes = %v, want %v", got.Payload[0].Excludes, want)
	}
}

// TestSaveLoad_IdentityLicenseRoundTrip is the same guarantee for
// Identity.License (a short identifier like "GPL v3", distinct from
// Identity.LicenseFile's path).
func TestSaveLoad_IdentityLicenseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "installerbear.yaml")

	proj := &Project{
		Identity: Identity{Name: "Test App", ID: "com.example.testapp", Version: "1.0.0", License: "GPL v3"},
	}
	if err := Save(proj, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := LoadLenient(path)
	if err != nil {
		t.Fatalf("LoadLenient: %v", err)
	}
	if got.Identity.License != "GPL v3" {
		t.Errorf("Identity.License = %q, want GPL v3", got.Identity.License)
	}
}
