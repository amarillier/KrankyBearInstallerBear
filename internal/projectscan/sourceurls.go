package projectscan

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"installerbear/internal/packager"
)

// sourceURLFiles is the fixed set of source files checked for a project
// homepage/repo URL when git's own remote doesn't have one. Every
// KrankyBear-template project embeds its GitHub URL in its About/Help
// dialogs and update checker (see this very project's own help.go, about.go,
// update.go), so a project copied from the template before `git remote add`
// was ever run — or one that was never git-init'd at all — still has the
// URL sitting in plain source.
var sourceURLFiles = []string{"help.go", "about.go", "update.go"}

// urlRE matches a bare http(s) URL up to the first whitespace, quote, or Go
// delimiter — enough for both a quoted source literal
// ("https://github.com/owner/repo") and free text in a help string.
var urlRE = regexp.MustCompile("https?://[^\\s\"'`)]+")

// findSourceURL best-effort-scans sourceURLFiles in dir's root for a
// project homepage/repo URL. Returns "" if none of the files exist, or none
// contain anything that looks like one.
//
// A github.com URL is normalized to its repo root via
// packager.ParseGitHubURL, which already discards any trailing path (e.g.
// "/blob/main/LICENSE", "/releases/latest" — see its own doc comment). This
// project's own template links GitHub in at least three different forms
// (repo root, license blob, releases page), so normalizing means it doesn't
// matter which one happens to be found first.
//
// For any other host — GitLab, a personal site, since other authors follow
// other conventions than this template's own — there's no equivalent
// path-trimming rule, so the shortest URL found (fewest path segments) is
// preferred instead, on the assumption that a homepage/repo link is plainer
// than a deep link to one specific file or page within it.
func findSourceURL(dir string) string {
	var urls []string
	for _, name := range sourceURLFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, raw := range urlRE.FindAllString(string(data), -1) {
			urls = append(urls, strings.TrimRight(raw, ".,;:"))
		}
	}

	var best string
	bestSegments := -1
	for _, raw := range urls {
		if owner, repo, ok := packager.ParseGitHubURL(raw); ok {
			return "https://github.com/" + owner + "/" + repo
		}
		segments := pathSegments(raw)
		if bestSegments == -1 || segments < bestSegments {
			best = raw
			bestSegments = segments
		}
	}
	return best
}

// pathSegments counts the non-empty "/"-separated components of raw's URL
// path, ignoring any query string or fragment. An unparseable URL counts as
// maximally deep (so it never wins over a URL that did parse).
func pathSegments(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return int(^uint(0) >> 1)
	}
	trimmed := strings.Trim(u.Path, "/")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}
