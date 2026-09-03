// Package gitconfig best-effort-reads the handful of git config values
// InstallerBear's New Project smart defaults care about: the "origin"
// remote URL and the committer identity. It hand-parses the INI-style
// .git/config format directly rather than shelling out to the git binary
// (this must degrade silently with no git installed) or pulling in a full
// git library (go-git is only an indirect dependency today, via nfpm's
// changelog feature, and a full repository-object library is overkill for
// reading three known keys).
package gitconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Info is what can be read out of a repo-local .git/config, with
// user.name/user.email falling back to the global ~/.gitconfig if the repo
// doesn't set them itself. Every field is "" if not found — not-a-repo, or
// a repo with no remote/identity configured, is a normal, silent outcome,
// never an error.
type Info struct {
	RemoteOriginURL string
	UserName        string
	UserEmail       string
}

// Read best-effort-inspects dir/.git/config, falling back to the user's
// global ~/.gitconfig for UserName/UserEmail alone (a remote is always
// repo-specific, so it has no meaningful global fallback).
func Read(dir string) Info {
	var info Info

	if local := parseFile(filepath.Join(dir, ".git", "config")); local != nil {
		info.RemoteOriginURL = local[section("remote", "origin")]["url"]
		info.UserName = local[section("user", "")]["name"]
		info.UserEmail = local[section("user", "")]["email"]
	}

	if info.UserName == "" || info.UserEmail == "" {
		if home, err := os.UserHomeDir(); err == nil {
			if global := parseFile(filepath.Join(home, ".gitconfig")); global != nil {
				if info.UserName == "" {
					info.UserName = global[section("user", "")]["name"]
				}
				if info.UserEmail == "" {
					info.UserEmail = global[section("user", "")]["email"]
				}
			}
		}
	}

	return info
}

// section builds the internal key used to look up a parsed [type "sub"] or
// [type] header.
func section(typ, sub string) string {
	if sub == "" {
		return typ
	}
	return typ + "." + sub
}

// parseFile parses a git INI-style config file into section -> key -> value.
// Returns nil if the file can't be read at all (missing, permissions, ...)
// — always treated as "nothing found," never a hard error, since every
// caller here is a best-effort convenience.
func parseFile(path string) map[string]map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	result := make(map[string]map[string]string)
	currentSection := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = parseSectionHeader(line[1 : len(line)-1])
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || currentSection == "" {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"`)

		if result[currentSection] == nil {
			result[currentSection] = make(map[string]string)
		}
		result[currentSection][key] = value
	}
	return result
}

// parseSectionHeader turns `remote "origin"` or `user` (the contents of a
// [...] header, already stripped of its brackets) into the same "type.sub"
// / "type" key section() builds for lookups.
func parseSectionHeader(header string) string {
	typ, rest, hasSub := strings.Cut(header, " ")
	typ = strings.ToLower(strings.TrimSpace(typ))
	if !hasSub {
		return typ
	}
	sub := strings.Trim(strings.TrimSpace(rest), `"`)
	return section(typ, sub)
}
