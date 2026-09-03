package packager

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"KrankyBear InstallerBear": "krankybear-installerbear",
		"Test App":                 "test-app",
		"already-kebab":            "already-kebab",
		"  spaced  out  ":          "spaced-out",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
