package packager

import (
	"net/url"
	"strings"
)

// ParseGitHubURL recognizes a github.com remote in any of the forms git
// itself accepts (an SCP-like "git@github.com:owner/repo.git", an
// "ssh://git@github.com/owner/repo.git", or a plain
// "https://github.com/owner/repo(.git)?" browse URL) and returns the owner
// and repo name, or ok=false if raw doesn't parse or isn't a GitHub remote.
// Shared by every place that needs to recognize a GitHub URL, so they all
// agree on what counts as one.
func ParseGitHubURL(raw string) (owner, repo string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}

	var host, path string
	if rest, found := strings.CutPrefix(raw, "git@"); found {
		// SCP-like syntax: git@github.com:owner/repo.git
		host, path, ok = strings.Cut(rest, ":")
		if !ok {
			return "", "", false
		}
	} else if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host = u.Host
		path = u.Path
	} else {
		return "", "", false
	}

	if !strings.EqualFold(host, "github.com") {
		return "", "", false
	}

	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	segments := strings.Split(path, "/")
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", "", false
	}
	return segments[0], segments[1], true
}
