package innoimport

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PkgConfigResult is what a best-effort parse of a KrankyBear-template
// project's build-config.sh/package.sh, plus a standalone Info.plist
// convention neither of those files owns, proposes. Every string field is
// "" if not found.
type PkgConfigResult struct {
	Name, Version, URL, Publisher, Vendor, Description string
	DesktopComment, DesktopCategories                  string
	// BundleID is the reverse-DNS identifier (packproject's Identity.ID)
	// read straight out of an existing Info.plist/Info-plist.txt's own
	// CFBundleIdentifier, when one is present — this project's own
	// convention keeps that file at the project root, alongside
	// build-config.sh, specifically so a real bundle ID never needs
	// guessing at (see suggestBundleID in identityform.go, which is only
	// ever a draft precisely because nothing else gives a real answer).
	BundleID string
	// License is KB_LICENSE_DEFAULT (e.g. "GPL v3") — a short identifier,
	// not a file, so it maps to packproject's Identity.License rather than
	// Identity.LicenseFile.
	License string
	// ISSPath is the project-base-dir-relative path build-config.sh's own
	// KB_INNO_ISS points at, or "" if that variable wasn't set — lets the
	// caller chain straight into ParseISS without guessing an Inno/*.iss
	// convention.
	ISSPath string
	// Skipped reports variables this parser recognized but has nowhere to
	// put. Currently always empty — kept so a future recognized-but-
	// unmappable variable has somewhere to report to without a signature
	// change, the same way ISSResult.Skipped already does for .iss content.
	Skipped []string
}

// exportRE matches this template's own build-config.sh convention: a bare
// `export KB_KEY="value"` or `export KB_KEY=value` line (see this repo's
// own build-config.sh). Deliberately narrow — this is not a general bash
// parser, just this one fixed shape.
var exportRE = regexp.MustCompile(`^export\s+(KB_\w+)=(.*)$`)

// descriptionRE matches package.sh's own `--description "..."` line inside
// its COMMON_ARGS array — the one piece of real signal package.sh itself
// carries that build-config.sh doesn't already have a KB_ variable for.
var descriptionRE = regexp.MustCompile(`--description\s+"([^"]*)"`)

// bundleIDRE matches a plist's <key>CFBundleIdentifier</key> and captures
// the <string> value on the following line — a small, targeted regex
// rather than a real plist parser, since this is the one value being
// extracted from what's otherwise an XML file.
var bundleIDRE = regexp.MustCompile(`(?s)<key>\s*CFBundleIdentifier\s*</key>\s*<string>\s*([^<\s]+)\s*</string>`)

// infoPlistNames is the priority order checked for an existing Info.plist —
// this template's own convention renames it to the ".txt" extension (see
// package.sh's own comment: "prevent fpm bundle detection issues"), but a
// project that never went through that rename, or one not descended from
// this template at all, may still have a plain Info.plist.
var infoPlistNames = []string{"Info-plist.txt", "Info.plist", "Info-Plist.txt"}

// ParseTemplatePackageConfig best-effort-parses dir/build-config.sh,
// dir/package.sh, and an existing Info.plist/Info-plist.txt into a
// PkgConfigResult. A directory missing any one of these is a normal
// outcome for that field, not a failure — this only returns an error for a
// real I/O problem reading a file that does exist.
func ParseTemplatePackageConfig(dir string) (PkgConfigResult, error) {
	var res PkgConfigResult

	vars, err := parseKBExports(filepath.Join(dir, "build-config.sh"))
	switch {
	case err == nil:
		res.Name = vars["KB_PROJECT_TITLE"]
		res.Version = vars["KB_VERSION_DEFAULT"]
		res.URL = vars["KB_HOMEPAGE"]
		res.Publisher = vars["KB_MAINTAINER_DEFAULT"]
		res.Vendor = vars["KB_VENDOR_DEFAULT"]
		res.DesktopComment = vars["KB_DESKTOP_COMMENT"]
		res.DesktopCategories = vars["KB_DESKTOP_CATEGORIES"]
		res.ISSPath = filepath.ToSlash(vars["KB_INNO_ISS"])
		res.License = vars["KB_LICENSE_DEFAULT"]
	case !os.IsNotExist(err):
		return res, err
	}

	if desc, ok := findDescription(filepath.Join(dir, "package.sh")); ok {
		res.Description = desc
	}
	res.BundleID = findBundleID(dir)

	return res, nil
}

// findBundleID checks dir's root for a well-known Info.plist filename (see
// infoPlistNames' priority order) and extracts its CFBundleIdentifier.
// Returns "" if no such file exists, or it doesn't contain that key —
// both normal outcomes, never an error.
func findBundleID(dir string) string {
	for _, name := range infoPlistNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if m := bundleIDRE.FindStringSubmatch(string(data)); m != nil {
			return m[1]
		}
	}
	return ""
}

// parseKBExports reads path and returns every `export KB_KEY="value"` (or
// unquoted) assignment it finds, tolerant of comments and blank lines.
func parseKBExports(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := exportRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		if idx := strings.Index(value, "#"); idx >= 0 {
			value = strings.TrimSpace(value[:idx]) // strip a trailing "# comment"
		}
		vars[m[1]] = strings.Trim(value, `"`)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

// findDescription greps path for package.sh's own `--description "..."`
// line. Returns ok=false if path doesn't exist or contains no such line —
// both normal outcomes, never an error.
func findDescription(path string) (desc string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	m := descriptionRE.FindStringSubmatch(string(data))
	if m == nil {
		return "", false
	}
	return m[1], true
}
