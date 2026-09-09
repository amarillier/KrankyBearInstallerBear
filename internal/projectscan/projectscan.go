// Package projectscan best-effort-detects InstallerBear Identity defaults
// from the conventions a project directory typically already follows: a git
// remote/committer identity (or, failing that, a URL embedded in its own
// help/about/update source — see sourceurls.go), a LICENSE file, a
// ReleaseNotes.txt, and an icon under assets/images/. Every field of
// Defaults is optional — a directory that follows none of these
// conventions just gets a zero Defaults, never an error.
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
	Name        string
	Version     string
	Description string
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

// Scan best-effort-inspects dir for a git remote/identity (falling back to
// a URL found in source, see findSourceURL, when there's no remote), a
// LICENSE file, a ReleaseNotes.txt, and icons under assets/images/. Never
// returns an error — a directory with none of these conventions simply
// yields a zero Defaults.
func Scan(dir string) Defaults {
	var d Defaults

	info := gitconfig.Read(dir)
	if owner, repo, ok := packager.ParseGitHubURL(info.RemoteOriginURL); ok {
		d.URL = "https://github.com/" + owner + "/" + repo
	}
	d.Publisher = info.UserName

	if d.URL == "" {
		d.URL = findSourceURL(dir)
	}

	d.LicenseFile = findLicenseFile(dir)
	d.IconICO, d.IconICNS, d.IconPNG = findIcons(dir)
	d.Name, d.Version, d.Description = parseReleaseNotes(dir)

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

// findIcons checks dir/assets/images for the first *.ico and *.icns file
// (independently, one per extension) plus a matching *.png, returning each
// as a path relative to dir, or "" if that extension has no match.
//
// The .png pick prefers whichever PNG shares its basename with the .ico or
// .icns match (rename-app.sh's own convention: <icon>.ico, <icon>.icns,
// <icon>-win.png all sharing "<icon>") over the first alphabetical PNG in
// the folder, which previously could just as easily grab an unrelated image
// (e.g. a caricature dropped in assets/images alongside the real app icon).
func findIcons(dir string) (ico, icns, png string) {
	imagesDir := filepath.Join(dir, "assets", "images")
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		return "", "", ""
	}

	var pngNames []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".ico":
			if ico == "" {
				ico = e.Name()
			}
		case ".icns":
			if icns == "" {
				icns = e.Name()
			}
		case ".png":
			pngNames = append(pngNames, e.Name())
		}
	}

	png = matchingPNG(pngNames, ico, icns)

	if ico != "" {
		ico = filepath.Join("assets", "images", ico)
	}
	if icns != "" {
		icns = filepath.Join("assets", "images", icns)
	}
	if png != "" {
		png = filepath.Join("assets", "images", png)
	}
	return ico, icns, png
}

// matchingPNG picks, from pngNames, the one whose basename (stripped of
// its extension) matches ico's or icns' basename — trying ico first, since
// rename-app.sh always names the Windows icon after the source PNG it was
// generated from. Falls back to the first (alphabetically, since ReadDir
// already returns sorted entries) PNG when neither matches, or there is no
// ico/icns to match against at all.
func matchingPNG(pngNames []string, ico, icns string) string {
	for _, stem := range []string{stemOf(ico), stemOf(icns)} {
		if stem == "" {
			continue
		}
		for _, name := range pngNames {
			if strings.EqualFold(stemOf(name), stem) {
				return name
			}
		}
	}
	if len(pngNames) > 0 {
		return pngNames[0]
	}
	return ""
}

// stemOf returns name without its extension, or "" if name is empty.
func stemOf(name string) string {
	if name == "" {
		return ""
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}
