package projectscan

import (
	"os"
	"path/filepath"

	"installerbear/internal/packproject"
)

// PayloadCandidate is one proposed packproject.PayloadEntry from a
// directory scan. It is never applied automatically — the caller must
// present these for user review/edit/removal before adding any of them to
// Project.Payload, since payload destination correctness matters more here
// than for the other scan-based conveniences.
type PayloadCandidate struct {
	Source    string // relative to proj.BaseDir when known, else absolute
	Dest      string // best-effort guess only (the entry's own basename); always editable
	Recursive bool   // true for directories, false for files
	// OS and Excludes carry straight through to the resulting
	// packproject.PayloadEntry when a candidate is accepted (see
	// buildPayloadEntries) — left empty by ScanPayloadCandidates itself
	// (a directory scan has no OS/exclude information to propose), but
	// set by internal/innoimport's .iss importer, which does.
	OS       []string
	Excludes []string
}

// ScanPayloadCandidates lists the immediate (non-recursive) children of dir
// and proposes one PayloadCandidate per entry, skipping any entry whose
// resolved path already matches proj.Identity.LicenseFile, one of
// proj.Identity.Icons.*, or an existing proj.Payload[i].Source — so
// re-running the scan, or scanning a directory that also holds the
// license/icon already wired up elsewhere, doesn't propose duplicates.
func ScanPayloadCandidates(dir string, proj *packproject.Project) []PayloadCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	covered := coveredPaths(proj)

	var candidates []PayloadCandidate
	for _, entry := range entries {
		abs := filepath.Clean(filepath.Join(dir, entry.Name()))
		if covered[abs] {
			continue
		}

		source := abs
		if proj.BaseDir != "" {
			if rel, err := filepath.Rel(proj.BaseDir, abs); err == nil {
				source = rel
			}
		}

		// Dest defaults to the entry's own basename only for a folder (its
		// contents land under a same-named subdirectory) - for a plain
		// file, "" (the install root) is correct instead, since a
		// non-recursive entry's installed filename already comes from
		// Source's own basename automatically; defaulting it to the same
		// basename again would nest the file one level deeper than
		// intended (e.g. ".../ReleaseNotes.md/ReleaseNotes.md" instead of
		// ".../ReleaseNotes.md") - the same fix as payloadtable.go's own
		// defaultPayloadDest, applied here too since this scan proposes
		// candidates independently of that dialog.
		dest := ""
		if entry.IsDir() {
			dest = entry.Name()
		}
		candidates = append(candidates, PayloadCandidate{
			Source:    source,
			Dest:      dest,
			Recursive: entry.IsDir(),
		})
	}
	return candidates
}

// coveredPaths resolves every path already referenced elsewhere in proj
// (Identity.LicenseFile, Identity.Icons.*, existing Payload sources) to an
// absolute, cleaned form, for exact-match exclusion in ScanPayloadCandidates.
func coveredPaths(proj *packproject.Project) map[string]bool {
	covered := make(map[string]bool)
	add := func(rel string) {
		if rel == "" {
			return
		}
		covered[filepath.Clean(proj.ResolvePath(rel))] = true
	}

	add(proj.Identity.LicenseFile)
	add(proj.Identity.Icons.ICO)
	add(proj.Identity.Icons.ICNS)
	add(proj.Identity.Icons.PNG)
	for _, p := range proj.Payload {
		add(p.Source)
	}
	return covered
}
