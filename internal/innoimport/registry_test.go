package innoimport

import (
	"testing"

	"installerbear/internal/packproject"
)

func noExpand(s string) string { return s }

func assocContains(t *testing.T, assocs []packproject.FileAssociation, ext, desc string) {
	t.Helper()
	for _, a := range assocs {
		if a.Extension == ext && a.Description == desc {
			return
		}
	}
	t.Errorf("expected {Extension: %q, Description: %q} in %+v", ext, desc, assocs)
}

// TestParseFileAssociations_ModernOpenWithProgidsShape covers the shape
// this repo's own real Inno/KrankyBearInstallerBear.iss fixture actually
// uses (confirmed empirically, not assumed) - Inno's current project
// wizard output when file-association support is enabled.
func TestParseFileAssociations_ModernOpenWithProgidsShape(t *testing.T) {
	lines := []string{
		`Root: HKA; Subkey: "Software\Classes\.myp\OpenWithProgids"; ValueType: string; ValueName: "MyAppMyp"; ValueData: ""; Flags: uninsdeletevalue`,
		`Root: HKA; Subkey: "Software\Classes\MyAppMyp"; ValueType: string; ValueName: ""; ValueData: "My App Project File"; Flags: uninsdeletekey`,
		`Root: HKA; Subkey: "Software\Classes\MyAppMyp\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\MyApp.exe,0"`,
		`Root: HKA; Subkey: "Software\Classes\MyAppMyp\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\MyApp.exe"" ""%1"""`,
	}

	assocs, unrecognized := parseFileAssociations(lines, noExpand)

	if len(assocs) != 1 {
		t.Fatalf("expected exactly 1 association, got %+v", assocs)
	}
	assocContains(t, assocs, ".myp", "My App Project File")
	// DefaultIcon doesn't match any recognized shape (InstallerBear's own
	// backends always use the app's own icon, not an imported one) - it's
	// expected to be counted as unrecognized, not silently swallowed.
	if unrecognized != 1 {
		t.Errorf("unrecognized = %d, want 1 (the DefaultIcon line)", unrecognized)
	}
}

// TestParseFileAssociations_ClassicDirectHKCRShape covers the older,
// simpler form some hand-written .iss scripts use instead of the wizard-
// generated OpenWithProgids indirection.
func TestParseFileAssociations_ClassicDirectHKCRShape(t *testing.T) {
	lines := []string{
		`Root: HKCR; Subkey: ".myp"; ValueType: string; ValueName: ""; ValueData: "MyAppMyp"; Flags: uninsdeletevalue`,
		`Root: HKCR; Subkey: "MyAppMyp"; ValueType: string; ValueName: ""; ValueData: "My App Project File"; Flags: uninsdeletekey`,
		`Root: HKCR; Subkey: "MyAppMyp\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\MyApp.exe"" ""%1"""`,
	}

	assocs, unrecognized := parseFileAssociations(lines, noExpand)

	if len(assocs) != 1 {
		t.Fatalf("expected exactly 1 association, got %+v", assocs)
	}
	assocContains(t, assocs, ".myp", "My App Project File")
	if unrecognized != 0 {
		t.Errorf("unrecognized = %d, want 0 (every line accounted for)", unrecognized)
	}
}

// TestParseFileAssociations_RequiresShellOpenCommandSibling guards
// against a false positive: a bare class key with an empty ValueName and
// non-blank ValueData could in principle be an unrelated registry entry,
// not a real file-type association - without a confirmed
// "<ProgID>\shell\open\command" sibling, nothing should be proposed.
func TestParseFileAssociations_RequiresShellOpenCommandSibling(t *testing.T) {
	lines := []string{
		`Root: HKCR; Subkey: ".myp"; ValueType: string; ValueName: ""; ValueData: "MyAppMyp"`,
		`Root: HKCR; Subkey: "MyAppMyp"; ValueType: string; ValueName: ""; ValueData: "My App Project File"`,
		// no shell\open\command line at all
	}

	assocs, unrecognized := parseFileAssociations(lines, noExpand)

	if len(assocs) != 0 {
		t.Errorf("expected no associations without a shell\\open\\command sibling, got %+v", assocs)
	}
	// Both lines structurally matched a recognized shape (so neither
	// counts toward the raw "line matched nothing at all" tally), but the
	// ProgID they describe never got confirmed by a shell\open\command
	// sibling - counted once, at the per-ProgID-group level, by the
	// "descriptions never used" check, not once per raw line.
	if unrecognized != 1 {
		t.Errorf("unrecognized = %d, want 1 (the unconfirmed ProgID group)", unrecognized)
	}
}

func TestParseFileAssociations_UnrelatedLinesAreAllUnrecognized(t *testing.T) {
	lines := []string{
		`Root: HKCU; Subkey: "Software\MyApp"; ValueType: string; ValueName: "SomeSetting"; ValueData: "1"`,
	}

	assocs, unrecognized := parseFileAssociations(lines, noExpand)

	if len(assocs) != 0 {
		t.Errorf("expected no associations from an unrelated registry entry, got %+v", assocs)
	}
	if unrecognized != 1 {
		t.Errorf("unrecognized = %d, want 1", unrecognized)
	}
}

func TestParseFileAssociations_EmptyInputIsFine(t *testing.T) {
	assocs, unrecognized := parseFileAssociations(nil, noExpand)
	if len(assocs) != 0 || unrecognized != 0 {
		t.Errorf("expected (nil, 0) for empty input, got (%+v, %d)", assocs, unrecognized)
	}
}

// TestParseISS_RealFixtureFileAssociation confirms ParseISS end-to-end
// against this repo's own real, committed Inno/KrankyBearInstallerBear.iss
// - the same "real repo fixture, not synthetic" convention
// TestParseISS_RealFixture (iss_test.go) already established for
// everything else this parser understands.
func TestParseISS_RealFixtureFileAssociation(t *testing.T) {
	baseDir := t.TempDir() // paths aren't what's under test here
	res, err := ParseISS("../../Inno/KrankyBearInstallerBear.iss", baseDir)
	if err != nil {
		t.Fatalf("ParseISS: %v", err)
	}

	if len(res.FileAssociations) != 1 {
		t.Fatalf("expected exactly 1 recognized file association, got %+v", res.FileAssociations)
	}
	if res.FileAssociations[0].Extension != ".exe" {
		t.Errorf("Extension = %q, want %q (this fixture's own unedited Inno-wizard placeholder value)",
			res.FileAssociations[0].Extension, ".exe")
	}
}
