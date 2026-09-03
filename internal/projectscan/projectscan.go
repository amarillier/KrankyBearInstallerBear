// Package projectscan best-effort-detects InstallerBear Identity defaults
// from the conventions a project directory typically already follows: a git
// remote/committer identity, a LICENSE file, and an icon under
// assets/images/. Every field of Defaults is optional — a directory that
// follows none of these conventions just gets a zero Defaults, never an
// error.
package projectscan

import (
	"os"
	"path/filepath"
	"strings"

	"installerbear/internal/gitconfig"
	"installerbear/internal/packager"
)

// Defaults is what a best-effort scan of a freshly chosen project base
// directory can propose for Identity fields. Every value is "" ("nothing
// found, leave blank") unless stated otherwise. Paths are relative to the
// scanned directory, matching how packproject.Project.ResolvePath treats
// Identity.LicenseFile/Icons.*.
type Defaults struct {
	URL         string
	Publisher   string
	LicenseFile string
	IconICO     string
	IconICNS    string
	IconPNG     string
}

// licenseNames is the fixed priority order used when more than one
// LICENSE-like file is present — first match wins.
var licenseNames = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"}

// Scan best-effort-inspects dir for a git remote/identity, a LICENSE file,
// and icons under assets/images/. Never returns an error — a directory with
// none of these conventions simply yields a zero Defaults.
func Scan(dir string) Defaults {
	var d Defaults

	info := gitconfig.Read(dir)
	if owner, repo, ok := packager.ParseGitHubURL(info.RemoteOriginURL); ok {
		d.URL = "https://github.com/" + owner + "/" + repo
	}
	d.Publisher = info.UserName

	d.LicenseFile = findLicenseFile(dir)
	d.IconICO, d.IconICNS, d.IconPNG = findIcons(dir)

	return d
}

// findLicenseFile checks dir's root for a well-known license filename
// (case-insensitively), in licenseNames' priority order, and returns the
// matched name (relative to dir) or "".
func findLicenseFile(dir string) string {
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
	for _, want := range licenseNames {
		if name, ok := byLower[strings.ToLower(want)]; ok {
			return name
		}
	}
	return ""
}

// findIcons checks dir/assets/images for the first *.ico, *.icns, and *.png
// file (independently, one per extension), returning each as a path
// relative to dir, or "" if that extension has no match.
func findIcons(dir string) (ico, icns, png string) {
	imagesDir := filepath.Join(dir, "assets", "images")
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		return "", "", ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		rel := filepath.Join("assets", "images", e.Name())
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".ico":
			if ico == "" {
				ico = rel
			}
		case ".icns":
			if icns == "" {
				icns = rel
			}
		case ".png":
			if png == "" {
				png = rel
			}
		}
	}
	return ico, icns, png
}
