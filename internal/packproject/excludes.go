package packproject

import (
	"path/filepath"
	"strings"
)

// toSlash normalizes both "/" (every platform) and "\" (a literal
// separator in a Windows-style Inno path or Excludes pattern, which
// filepath.ToSlash leaves untouched on non-Windows since it only rewrites
// the current OS's own separator) to "/", so matching behaves the same
// regardless of which OS InstallerBear itself runs on.
func toSlash(s string) string {
	return strings.ReplaceAll(s, `\`, "/")
}

// ExcludesMatch reports whether relPath (a Payload entry's Source-relative
// path, using either slash) should be skipped per e.Excludes — the
// analogue of Inno's [Files] "Excludes:" attribute on a recursive copy.
// Shared by every backend's own recursive-copy code so a Payload entry
// behaves identically regardless of target.
//
// A pattern is matched three ways, broadest first:
//  1. "<dir>/*" excludes <dir> and everything under it at any depth (Inno's
//     own recursesubdirs+Excludes semantics — a plain filepath.Match "*"
//     wouldn't cross the "/" a nested file's path introduces).
//  2. A pattern containing "/" is matched via filepath.Match against the
//     full relPath.
//  3. A pattern with no "/" is matched via filepath.Match against just
//     relPath's base name, so e.g. "*.tmp" excludes matches at any depth.
func (e PayloadEntry) ExcludesMatch(relPath string) bool {
	relPath = toSlash(relPath)
	for _, raw := range e.Excludes {
		pattern := toSlash(raw)

		if dir, ok := strings.CutSuffix(pattern, "/*"); ok {
			if relPath == dir || strings.HasPrefix(relPath, dir+"/") {
				return true
			}
		}

		if strings.Contains(pattern, "/") {
			if ok, _ := filepath.Match(pattern, relPath); ok {
				return true
			}
			continue
		}

		if ok, _ := filepath.Match(pattern, filepath.Base(relPath)); ok {
			return true
		}
	}
	return false
}
