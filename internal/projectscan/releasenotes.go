package projectscan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// releaseNotesNames is the fixed priority order used when looking for a
// ReleaseNotes file, mirroring findLicenseFile's case-insensitive approach —
// first match wins. .md first, matching this project's own convention as of
// 2026-09-12 (its own ReleaseNotes.txt was retired in favor of
// ReleaseNotes.md — nicer to read, per Allan's own call) and the same
// priority the in-app Release Notes viewer itself uses (see
// releaseNotesFileNames in ../../releasenotes.go). A scanned project on the
// older .txt convention is still found just as well; only the preference
// order changed.
var releaseNotesNames = []string{"ReleaseNotes.md", "ReleaseNotes.txt", "RELEASENOTES.md", "RELEASENOTES.txt"}

// versionLineRE matches this template's "Version X.Y.Z - <date>" heading
// (see ReleaseNotes.md itself) and captures just the version token.
var versionLineRE = regexp.MustCompile(`(?i)^Version\s+(\S+)`)

// parseReleaseNotes best-effort-extracts an app Name, current Version, and
// Description from a ReleaseNotes file in dir's root, following this
// template's own ReleaseNotes.md/.txt convention (plain line-based text
// either way - this parser doesn't care about real Markdown syntax):
//
//	Release notes
//	<Name>: <description, possibly wrapped across a couple of lines>
//
//	...
//
//	Version <X.Y.Z> - <date>
//	━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//	...
//
// Any deviation from this shape just leaves the corresponding field "" —
// this is a convenience prefill, not a strict format. Version is taken from
// the first "Version ..." line since this file lists releases newest-first.
func parseReleaseNotes(dir string) (name, version, description string) {
	path := findReleaseNotesFile(dir)
	if path == "" {
		return "", "", ""
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "", ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	name, description = parseIntro(lines)
	version = parseVersion(lines)
	return name, version, description
}

// parseIntro reads the leading non-blank block of lines (skipping a bare
// "Release notes" heading, if present) and splits its first line on the
// first ": " into name/description, appending any further lines in the
// same block to description. Returns "", "" if the block doesn't start
// with a "<Name>: " line at all.
func parseIntro(lines []string) (name, description string) {
	i := 0
	if i < len(lines) && strings.EqualFold(lines[i], "release notes") {
		i++
	}
	for i < len(lines) && lines[i] == "" {
		i++
	}

	var block []string
	for ; i < len(lines) && lines[i] != ""; i++ {
		block = append(block, lines[i])
	}
	if len(block) == 0 {
		return "", ""
	}

	head, rest, ok := strings.Cut(block[0], ": ")
	if !ok || head == "" {
		return "", ""
	}
	descParts := append([]string{rest}, block[1:]...)
	return head, strings.Join(descParts, " ")
}

// parseVersion returns the version token from the first "Version ..." line
// found, or "".
func parseVersion(lines []string) string {
	for _, line := range lines {
		if m := versionLineRE.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

// findReleaseNotesFile checks dir's root for a well-known ReleaseNotes
// filename (case-insensitively), in releaseNotesNames' priority order, and
// returns its full path, or "" if none is present.
func findReleaseNotesFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	byLower := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			byLower[strings.ToLower(e.Name())] = e.Name()
		}
	}
	for _, want := range releaseNotesNames {
		if name, ok := byLower[strings.ToLower(want)]; ok {
			return filepath.Join(dir, name)
		}
	}
	return ""
}
