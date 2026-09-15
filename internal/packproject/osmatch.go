package packproject

import "strings"

// osmatch.go centralizes how a PayloadEntry's OS filter is matched against
// a real build's (OS, Arch) - previously each of the four backends
// hand-rolled its own identical `len(entry.OS) > 0 &&
// !slices.Contains(entry.OS, "<its own OS>")` check, which only ever
// compared OS, never arch, and had no room for a friendlier spelling of
// "darwin". Consolidating it here means all four backends can gain both
// at once, with nothing left to drift out of sync between them.

// AppliesToOS reports whether e should be included when building for
// targetOS/targetArch (Go's own GOOS/GOARCH spelling, e.g. "windows"/
// "amd64"). An empty OS list means every OS/arch, unchanged from before.
//
// Each entry in OS is either a bare OS name ("windows" - matches every
// arch of that OS, the same as today) or an "os/arch" pair
// ("windows/arm64" - matches only that exact arch), the same slash
// convention Docker's --platform and `go tool dist list` already use, not
// a new syntax to invent. "mac"/"macos" (case-insensitive) are accepted
// as aliases for "darwin" on either side of the slash, since most authors
// don't think of macOS by its Go GOOS name.
func (e PayloadEntry) AppliesToOS(targetOS, targetArch string) bool {
	if len(e.OS) == 0 {
		return true
	}
	for _, raw := range e.OS {
		wantOS, wantArch, hasArch := strings.Cut(raw, "/")
		if !canonicalOSEqual(wantOS, targetOS) {
			continue
		}
		if hasArch && !strings.EqualFold(wantArch, targetArch) {
			continue
		}
		return true
	}
	return false
}

// canonicalOSEqual compares two OS names case-insensitively, treating
// "mac"/"macos" as equal to "darwin" on either side.
func canonicalOSEqual(a, b string) bool {
	return strings.EqualFold(canonicalOS(a), canonicalOS(b))
}

// CanonicalOS is canonicalOS exported for callers outside this package -
// the GUI's OS/arch checkbox picker (see payloadtable.go's showPayloadDialog)
// uses it to classify a Payload entry's existing, possibly hand-typed OS
// values (which may use the mac/macos alias) into checkbox state without
// duplicating this alias table a second time.
func CanonicalOS(os string) string {
	return canonicalOS(os)
}

// canonicalOS normalizes a user-typed OS name to Go's own GOOS spelling
// where a friendlier alias exists ("mac"/"macos" -> "darwin"); anything
// else is returned lowercased but otherwise unchanged, so an unrecognized
// value (a typo, most likely) still fails to match rather than being
// silently coerced into something else.
func canonicalOS(os string) string {
	switch strings.ToLower(strings.TrimSpace(os)) {
	case "mac", "macos":
		return "darwin"
	default:
		return strings.ToLower(strings.TrimSpace(os))
	}
}
