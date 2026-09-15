package packproject

import "testing"

func TestAppliesToOS_EmptyMeansEveryOSAndArch(t *testing.T) {
	e := PayloadEntry{}
	for _, tc := range [][2]string{{"windows", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}} {
		if !e.AppliesToOS(tc[0], tc[1]) {
			t.Errorf("AppliesToOS(%q, %q) = false, want true for an empty OS list", tc[0], tc[1])
		}
	}
}

func TestAppliesToOS_BareOSMatchesEveryArch(t *testing.T) {
	e := PayloadEntry{OS: []string{"windows"}}
	if !e.AppliesToOS("windows", "amd64") {
		t.Error("expected a bare \"windows\" entry to match windows/amd64")
	}
	if !e.AppliesToOS("windows", "arm64") {
		t.Error("expected a bare \"windows\" entry to match windows/arm64 too")
	}
	if e.AppliesToOS("darwin", "amd64") {
		t.Error("expected a bare \"windows\" entry to not match darwin")
	}
}

func TestAppliesToOS_ArchScopedEntryMatchesOnlyThatArch(t *testing.T) {
	e := PayloadEntry{OS: []string{"windows/arm64"}}
	if !e.AppliesToOS("windows", "arm64") {
		t.Error("expected windows/arm64 to match windows/arm64")
	}
	if e.AppliesToOS("windows", "amd64") {
		t.Error("expected windows/arm64 to NOT match windows/amd64")
	}
	if e.AppliesToOS("darwin", "arm64") {
		t.Error("expected windows/arm64 to NOT match darwin/arm64 (OS itself differs)")
	}
}

func TestAppliesToOS_MixOfBareAndArchScopedEntries(t *testing.T) {
	e := PayloadEntry{OS: []string{"windows/arm64", "darwin"}}
	if !e.AppliesToOS("windows", "arm64") {
		t.Error("expected windows/arm64 to match")
	}
	if e.AppliesToOS("windows", "amd64") {
		t.Error("expected windows/amd64 to NOT match (only windows/arm64 and darwin are listed)")
	}
	if !e.AppliesToOS("darwin", "amd64") {
		t.Error("expected darwin/amd64 to match the bare \"darwin\" entry")
	}
	if !e.AppliesToOS("darwin", "arm64") {
		t.Error("expected darwin/arm64 to match the bare \"darwin\" entry too")
	}
}

func TestAppliesToOS_MacAndMacosAreAliasesForDarwin(t *testing.T) {
	for _, alias := range []string{"mac", "macos", "Mac", "MacOS", "MACOS"} {
		e := PayloadEntry{OS: []string{alias}}
		if !e.AppliesToOS("darwin", "arm64") {
			t.Errorf("expected OS entry %q to match darwin", alias)
		}
	}
}

func TestAppliesToOS_MacAliasWorksWithArchScope(t *testing.T) {
	e := PayloadEntry{OS: []string{"mac/arm64"}}
	if !e.AppliesToOS("darwin", "arm64") {
		t.Error("expected mac/arm64 to match darwin/arm64")
	}
	if e.AppliesToOS("darwin", "amd64") {
		t.Error("expected mac/arm64 to NOT match darwin/amd64")
	}
}

func TestAppliesToOS_CaseInsensitiveOSAndArch(t *testing.T) {
	e := PayloadEntry{OS: []string{"Windows/ARM64"}}
	if !e.AppliesToOS("windows", "arm64") {
		t.Error("expected case-insensitive matching for both OS and arch")
	}
}

func TestAppliesToOS_UnknownOSNeverMatches(t *testing.T) {
	e := PayloadEntry{OS: []string{"widnows"}} // typo
	if e.AppliesToOS("windows", "amd64") {
		t.Error("expected a typo'd OS name to never match, not be silently coerced")
	}
}
