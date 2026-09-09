package packproject

import "testing"

func TestExcludesMatchDirectoryWildcard(t *testing.T) {
	e := PayloadEntry{Excludes: []string{"mesa-win/*"}}

	cases := map[string]bool{
		"mesa-win/opengl32.dll":      true,
		"mesa-win/sub/deep/file.dll": true,
		"mesa-win":                   true,
		"other/opengl32.dll":         false,
		"mesa-windows/opengl32.dll":  false, // must not prefix-match past the "/"
	}
	for rel, want := range cases {
		if got := e.ExcludesMatch(rel); got != want {
			t.Errorf("ExcludesMatch(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestExcludesMatchBackslashPattern(t *testing.T) {
	e := PayloadEntry{Excludes: []string{`mesa-win\*`}}
	if !e.ExcludesMatch("mesa-win/opengl32.dll") {
		t.Error("expected backslash-separated pattern to match a forward-slash relPath")
	}
}

func TestExcludesMatchFullPathGlob(t *testing.T) {
	e := PayloadEntry{Excludes: []string{"assets/i18n/*.md"}}
	if !e.ExcludesMatch("assets/i18n/README.md") {
		t.Error("expected full-path glob to match")
	}
	if e.ExcludesMatch("assets/images/README.md") {
		t.Error("full-path glob must not match a different directory")
	}
}

func TestExcludesMatchBasenameGlobAnyDepth(t *testing.T) {
	e := PayloadEntry{Excludes: []string{"*.tmp"}}
	if !e.ExcludesMatch("build.tmp") {
		t.Error("expected top-level match")
	}
	if !e.ExcludesMatch("deep/nested/build.tmp") {
		t.Error("expected a no-separator pattern to match at any depth")
	}
	if e.ExcludesMatch("build.tmp.bak") {
		t.Error("must not match a file that only contains the pattern as a substring")
	}
}

func TestExcludesMatchNoPatterns(t *testing.T) {
	var e PayloadEntry
	if e.ExcludesMatch("anything") {
		t.Error("an entry with no Excludes should never match")
	}
}
