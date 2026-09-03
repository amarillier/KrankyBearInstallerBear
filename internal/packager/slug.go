package packager

import "strings"

// Slug lowercases a display name into the kebab-case form Linux/macOS CLI
// naming conventions expect (no spaces, no uppercase), e.g. "KrankyBear
// Template" -> "krankybear-installerbear". Shared by every backend that needs a
// package/executable-safe name derived from Identity.Name.
func Slug(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash && b.Len() > 0:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
