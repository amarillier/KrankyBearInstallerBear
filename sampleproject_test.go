package main

import (
	"testing"

	"gopkg.in/yaml.v3"

	"installerbear/internal/packproject"
)

// newSampleProject itself is dialog-driven (a real dialog.NewFileSave) and,
// matching this project's own convention (openProject/saveProjectAs aren't
// unit tested either - see projectio_test.go), isn't exercised directly
// here. These tests cover the pure logic behind it instead: the generated
// fallback project and the bundled-file-or-fallback selection.

func TestGeneratedSampleProject_HasKeyFieldsPopulated(t *testing.T) {
	proj := generatedSampleProject()

	if proj.Identity.Name == "" || proj.Identity.ID == "" || proj.Identity.Version == "" {
		t.Errorf("expected Identity.Name/ID/Version all set, got %+v", proj.Identity)
	}
	if len(proj.Binaries) == 0 {
		t.Error("expected at least one sample Binaries entry")
	}
	if len(proj.Payload) == 0 {
		t.Error("expected at least one sample Payload entry")
	}
	if len(proj.Targets) == 0 {
		t.Error("expected at least one sample Target")
	}
	if proj.Windows.UpgradeGUID == "" {
		t.Error("expected a generated Windows.UpgradeGUID")
	}
}

// TestGeneratedSampleProject_FreshGUIDPerCall guards against a fixed
// placeholder GUID sneaking back in - every call must mint its own, so two
// sample projects saved back to back never collide on Windows UpgradeCode.
func TestGeneratedSampleProject_FreshGUIDPerCall(t *testing.T) {
	a := generatedSampleProject()
	b := generatedSampleProject()

	if a.Windows.UpgradeGUID == b.Windows.UpgradeGUID {
		t.Errorf("expected two calls to mint different UpgradeGUIDs, both got %q", a.Windows.UpgradeGUID)
	}
}

// TestGeneratedSampleProject_RoundTripsThroughYAML confirms the generated
// fallback actually marshals and unmarshals cleanly - the same path
// newSampleProject takes when writing it to disk and reopening it via
// packproject.LoadLenient.
func TestGeneratedSampleProject_RoundTripsThroughYAML(t *testing.T) {
	data, err := packproject.Marshal(generatedSampleProject())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundTripped packproject.Project
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if roundTripped.Identity.Name != "My Sample App" {
		t.Errorf("round-tripped Identity.Name = %q, want %q", roundTripped.Identity.Name, "My Sample App")
	}
}

// TestSampleProjectYAML_FallsBackWhenNoBundledFileExists exercises
// sampleProjectYAML in the environment every `go test` run actually has:
// no sample-installerbear.yaml sits beside the test binary, so this always
// takes the generated-fallback branch - the bundled-file branch is a
// simple os.ReadFile beside the running executable, the same shape as
// releasenotes.go's own lookup, and not re-tested here for the same reason
// that one isn't (see releasenotes_test.go's own comment on os.Executable
// pointing at a temp test binary in `go test`).
func TestSampleProjectYAML_FallsBackWhenNoBundledFileExists(t *testing.T) {
	data, err := sampleProjectYAML()
	if err != nil {
		t.Fatalf("sampleProjectYAML: %v", err)
	}

	var proj packproject.Project
	if err := yaml.Unmarshal(data, &proj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if proj.Identity.Name != "My Sample App" {
		t.Errorf("expected the generated fallback's Identity.Name, got %q", proj.Identity.Name)
	}
}
