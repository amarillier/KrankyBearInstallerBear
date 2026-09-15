package packproject

import (
	"fmt"
	"path/filepath"
	"strings"
)

// payloadexpand.go resolves a Payload entry's Source glob pattern (see
// PayloadEntry's own doc comment) into concrete, literal entries - Allan's
// own idea, for a project with "a larger number of files" where listing
// each one by hand (as this repo's own installerbear.yaml does for
// installerbear.yaml/sample-installerbear.yaml) gets tedious, wanting the
// same glob dialect Excludes already uses rather than a second one to
// learn (real regex was considered and deliberately not used, for that
// consistency).

// isGlobPattern reports whether s contains any filepath.Match
// metacharacter - the same three Excludes already recognizes.
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// ExpandedPayload resolves payload into the concrete, literal entries
// every backend actually copies: any entry whose Source is a glob pattern
// is replaced by one entry per current match under baseDir (each
// inheriting that entry's own Dest/Recursive/OS/Excludes, with Source set
// to the match's own baseDir-relative path); every other entry passes
// through unchanged. A pattern matching nothing at all simply contributes
// no entries - not an error, see PayloadEntry's own doc comment.
//
// Deliberately pure - never mutates payload itself. Project.Payload is
// what the GUI edits and what gets saved back to installerbear.yaml, so
// baking today's matches into it here would silently freeze an
// intentional "whatever matches *.yaml at build time" pattern into a
// stale literal list the next time the project is saved. Callers that
// need the expanded result for something the pattern must survive past
// (Validate's own existence/collision checks, and packager.Run's actual
// build) work against this function's own return value instead - see
// Validate and packager.Run for exactly how.
func ExpandedPayload(payload []PayloadEntry, baseDir string) ([]PayloadEntry, error) {
	out := make([]PayloadEntry, 0, len(payload))
	for _, e := range payload {
		if !isGlobPattern(e.Source) {
			out = append(out, e)
			continue
		}

		pattern := filepath.Join(baseDir, filepath.FromSlash(e.Source))
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("payload source %q: invalid pattern: %w", e.Source, err)
		}

		for _, m := range matches {
			rel, err := filepath.Rel(baseDir, m)
			if err != nil {
				continue // outside baseDir entirely - shouldn't happen for a Join'd pattern, skip defensively
			}
			rel = filepath.ToSlash(rel)
			// ExcludesMatch itself already falls back to matching just the
			// basename for a pattern with no "/" (see its own doc comment),
			// so passing the full relPath handles both a plain "*.tmp"
			// -style pattern and a "dir/*.tmp"-style one correctly.
			if e.ExcludesMatch(rel) {
				continue
			}
			expanded := e
			expanded.Source = rel
			out = append(out, expanded)
		}
	}
	return out, nil
}

// ExpandedPayload is the Project-scoped convenience form of the package
// function above, resolving against p's own BaseDir.
func (p *Project) ExpandedPayload() ([]PayloadEntry, error) {
	return ExpandedPayload(p.Payload, p.BaseDir)
}
