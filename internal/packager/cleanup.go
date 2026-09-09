package packager

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// versionInFilenameRE matches a dotted version-like token, optionally
// followed by a "-<iteration>" suffix (nfpm's own convention for .deb/.rpm,
// e.g. "-1"), the same shape every backend here embeds a real version
// number in — see each one's own output filename construction
// (winexe/build.go, debrpm's nfpm.ConventionalFileName, ...).
var versionInFilenameRE = `[0-9]+(?:\.[0-9]+){1,3}(?:-[0-9]+)?`

// StaleInstallers scans the directory of every successful result in
// results for sibling files that look like an older-version build of that
// same installer — same filename as the one just built, except a
// different version token in the exact spot currentVersion appears in it.
// Never touches the file just built, never recurses into subdirectories,
// and skips any result whose OutputPath doesn't literally contain
// currentVersion (nothing to safely build a pattern from). Returns an
// absolute path per stale file found, deduplicated, purely for review —
// nothing is deleted here.
func StaleInstallers(results []BuildResult, currentVersion string) []string {
	if currentVersion == "" {
		return nil
	}

	var stale []string
	seen := make(map[string]bool)

	for _, r := range results {
		if r.Err != nil || r.Skipped || r.OutputPath == "" {
			continue
		}
		dir := filepath.Dir(r.OutputPath)
		base := filepath.Base(r.OutputPath)

		idx := strings.Index(base, currentVersion)
		if idx < 0 {
			continue
		}
		prefix := base[:idx]
		suffix := base[idx+len(currentVersion):]
		re, err := regexp.Compile("^" + regexp.QuoteMeta(prefix) + "(" + versionInFilenameRE + ")" + regexp.QuoteMeta(suffix) + "$")
		if err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == base {
				continue
			}
			m := re.FindStringSubmatch(entry.Name())
			if m == nil || m[1] == currentVersion {
				continue
			}
			full := filepath.Join(dir, entry.Name())
			if !seen[full] {
				seen[full] = true
				stale = append(stale, full)
			}
		}
	}

	return stale
}
