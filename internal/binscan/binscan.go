// Package binscan best-effort-classifies files in a directory as one of the
// six (OS, arch) binary slots InstallerBear's Binaries tab knows about, from
// filename conventions alone (no exec-bit check — that isn't reliable across
// host OSes when scanning e.g. a Linux build output from macOS).
package binscan

import (
	"os"
	"path/filepath"
	"strings"
)

// Guess is one proposed (OS, Arch) -> file match from a directory scan.
type Guess struct {
	OS, Arch, Path string
}

// osTokens is only used for the no-extension (darwin/linux) case — windows
// binaries are always recognized by their ".exe" suffix instead, since a
// bare substring match on "win" would false-positive on names like
// "windowmanager" that aren't Windows binaries at all.
var osTokens = map[string][]string{
	"darwin": {"darwin", "macos", "mac"},
	"linux":  {"linux"},
}

var archTokens = map[string][]string{
	"amd64": {"amd64", "x86_64", "x64"},
	"arm64": {"arm64", "aarch64"},
}

// ScanDir lists the (non-recursive) contents of dir and returns a best-effort
// guess for each (OS, Arch) slot it can identify. Ambiguous or unmatched
// files are simply absent from the result. Callers must only apply a guess
// to a slot that is currently empty.
func ScanDir(dir string) ([]Guess, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var guesses []Guess
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		osName, arch, ok := classify(entry.Name())
		if !ok {
			continue
		}
		guesses = append(guesses, Guess{OS: osName, Arch: arch, Path: filepath.Join(dir, entry.Name())})
	}
	return guesses, nil
}

// classify guesses the (os, arch) a single filename represents, based on
// this project's own compile-*.sh naming convention: a ".exe" suffix always
// means windows; otherwise an OS name token in the filename plus no
// extension signals darwin/linux. An arch token is matched if present;
// otherwise, if the OS is unambiguous, amd64 is assumed (this project's own
// Windows output never suffixes amd64 at all).
func classify(name string) (osName, arch string, ok bool) {
	lower := strings.ToLower(name)

	osName = matchToken(lower, osTokens)
	if strings.HasSuffix(lower, ".exe") {
		osName = "windows"
	} else if ext := filepath.Ext(lower); ext != "" {
		// Any other extension (.dll, .txt, .zip, ...) is never a binary slot.
		return "", "", false
	}
	if osName == "" {
		return "", "", false
	}

	arch = matchToken(lower, archTokens)
	if arch == "" {
		arch = "amd64"
	}
	return osName, arch, true
}

func matchToken(lower string, tokens map[string][]string) string {
	for key, aliases := range tokens {
		for _, alias := range aliases {
			if strings.Contains(lower, alias) {
				return key
			}
		}
	}
	return ""
}
