// Package innoimport best-effort-parses an existing Inno Setup .iss script
// (see iss.go) and the KrankyBear-template's own build-config.sh/package.sh
// (see packageconfig.go) into InstallerBear Identity/Windows/Payload/
// Binaries fields, so a project already packaged by hand doesn't need
// retyping to move onto InstallerBear. Every parse here is best-effort:
// anything not confidently understood is reported in the result's Skipped
// list rather than guessed at or silently dropped — see each function's
// own doc comment for exactly what is and isn't handled.
package innoimport

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

// ISSResult is what a best-effort parse of one .iss file proposes. Every
// string field is "" if not found. Path fields (LicenseFile, IconICO,
// WindowsBinary, and every Payload candidate's Source) are relative to
// baseDir, matching how the rest of packproject.Project stores paths.
type ISSResult struct {
	Name, Version, Publisher, URL string
	LicenseFile, IconICO          string
	UpgradeGUID, ExeName          string
	// WindowsBinary is the resolved path to the [Files] Source line whose
	// filename matched ExeName, or "" if none was found.
	WindowsBinary string

	Payload []projectscan.PayloadCandidate
	// FileAssociations is every file-type association ParseISS recognized
	// in [Registry] - see parseFileAssociations' own doc comment for
	// exactly what shape is understood (both the classic direct HKCR\.ext
	// form and the modern Inno-wizard-generated HKA\Software\Classes\...\
	// OpenWithProgids form - confirmed against this very repo's own real
	// Inno/KrankyBearInstallerBear.iss fixture, which uses the latter).
	FileAssociations []packproject.FileAssociation
	// Skipped lists, in human-readable form, everything the parser noticed
	// but didn't import: unsupported [Files] wildcard forms, DestDir
	// tokens other than "{app}", [Registry] lines that don't match a
	// recognized file-association shape, and a count of lines in sections
	// this importer doesn't understand at all ([Icons]/[Tasks]/[Run]/
	// [UninstallRun]/[UninstallDelete]).
	Skipped []string
}

var (
	defineRE  = regexp.MustCompile(`^#define\s+(\w+)\s+"([^"]*)"\s*$`)
	macroRE   = regexp.MustCompile(`\{#(\w+)\}`)
	sectionRE = regexp.MustCompile(`^\[([A-Za-z]+)\]$`)
)

// ParseISS best-effort-parses the .iss script at issPath into an ISSResult,
// resolving every path it finds relative to baseDir (the InstallerBear
// project's own base directory — typically the .iss's own parent's parent,
// per this template's own Inno/<App>.iss convention, but ParseISS itself
// doesn't assume that; the caller passes whatever baseDir it's importing
// into).
//
// Handles:
//   - Simple `#define NAME "value"` macros, substituted into `{#NAME}`
//     references elsewhere. A define whose value isn't a plain quoted
//     string (e.g. a Pascal-Script call like StringChange(...)) is left
//     unresolved rather than guessed at.
//   - [Setup]'s AppId (-> UpgradeGUID, unescaping Inno's "{{" literal-brace
//     doubling), AppName, AppVersion, AppPublisher, AppPublisherURL (else
//     AppURL), LicenseFile, SetupIconFile.
//   - [Files] lines whose Source resolves inside baseDir: a plain file
//     becomes a Payload candidate (or, if its name matches the ExeName
//     define, WindowsBinary instead); a "<dir>\*" or "<dir>\*.*" wildcard
//     with the recursesubdirs flag becomes a recursive Payload candidate,
//     carrying over any Excludes attribute. Any other wildcard form (no
//     recursesubdirs, or an extension filter like "*.dll") isn't
//     representable by a single PayloadEntry and is reported in Skipped
//     instead of guessed at.
//   - [Registry] lines that form a recognizable file-type association —
//     see parseFileAssociations' own doc comment for the two shapes
//     understood. Any [Registry] line that doesn't fit either shape is
//     reported in Skipped, not silently dropped.
//
// Does not handle [Icons], [Tasks], [Run], [UninstallRun], or
// [UninstallDelete] at all — packproject has no equivalent for any of them
// yet (desktop-icon opt-in, post-install launch, custom wizard pages, ...).
// Lines in those sections are counted and reported in Skipped, never
// silently dropped without a trace.
func ParseISS(issPath, baseDir string) (ISSResult, error) {
	f, err := os.Open(issPath)
	if err != nil {
		return ISSResult{}, fmt.Errorf("opening %s: %w", issPath, err)
	}
	defer f.Close()

	// Absolute regardless of whether the caller's issPath was — relInnoPath
	// joins this with each Inno-relative path and re-relativizes the result
	// against baseDir (always absolute; see packproject.Load's own
	// parseAndDefault), and filepath.Rel errors outright if it's handed one
	// absolute and one relative path to compare.
	issDir, err := filepath.Abs(filepath.Dir(issPath))
	if err != nil {
		return ISSResult{}, fmt.Errorf("resolving %s: %w", issPath, err)
	}
	defines := map[string]string{}
	setup := map[string]string{}
	var fileLines []string
	var registryLines []string
	otherSectionLines := 0

	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if m := defineRE.FindStringSubmatch(trimmed); m != nil {
			defines[m[1]] = m[2]
			continue
		}
		if m := sectionRE.FindStringSubmatch(trimmed); m != nil {
			section = m[1]
			continue
		}
		switch section {
		case "Setup":
			key, value, ok := strings.Cut(trimmed, "=")
			if ok {
				setup[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		case "Files":
			fileLines = append(fileLines, trimmed)
		case "Registry":
			registryLines = append(registryLines, trimmed)
		case "":
			// preamble (the #define block, comments) — nothing to record
		default:
			otherSectionLines++
		}
	}
	if err := scanner.Err(); err != nil {
		return ISSResult{}, fmt.Errorf("reading %s: %w", issPath, err)
	}

	expand := func(v string) string {
		return macroRE.ReplaceAllStringFunc(v, func(tok string) string {
			if val, ok := defines[tok[2:len(tok)-1]]; ok {
				return val
			}
			return tok
		})
	}

	var res ISSResult
	res.Name = expand(setup["AppName"])
	res.Version = expand(setup["AppVersion"])
	res.Publisher = expand(setup["AppPublisher"])
	res.URL = expand(firstNonEmpty(setup["AppPublisherURL"], setup["AppURL"]))
	res.ExeName = expand(defines["MyAppExeName"])
	if appID := expand(setup["AppId"]); appID != "" {
		res.UpgradeGUID = strings.ReplaceAll(appID, "{{", "{")
	}
	if lic := expand(setup["LicenseFile"]); lic != "" {
		if rel, ok := relInnoPath(issDir, baseDir, lic); ok {
			res.LicenseFile = rel
		} else {
			res.Skipped = append(res.Skipped, "LicenseFile is outside the project: "+lic)
		}
	}
	if icon := expand(setup["SetupIconFile"]); icon != "" {
		if rel, ok := relInnoPath(issDir, baseDir, icon); ok {
			res.IconICO = rel
		} else {
			res.Skipped = append(res.Skipped, "SetupIconFile is outside the project: "+icon)
		}
	}

	for _, line := range fileLines {
		res.addFileLine(line, issDir, baseDir, expand)
	}

	assocs, unrecognizedRegistryLines := parseFileAssociations(registryLines, expand)
	res.FileAssociations = assocs
	if unrecognizedRegistryLines > 0 {
		res.Skipped = append(res.Skipped, fmt.Sprintf(
			"%d line(s) in [Registry] were not recognized as a file association and were not imported",
			unrecognizedRegistryLines))
	}

	if otherSectionLines > 0 {
		res.Skipped = append(res.Skipped, fmt.Sprintf(
			"%d line(s) in [Icons]/[Tasks]/[Run]/[UninstallRun]/[UninstallDelete] were not imported (no matching InstallerBear feature yet)",
			otherSectionLines))
	}

	return res, nil
}

// addFileLine parses one [Files] line and appends whatever it resolves to
// (a Payload candidate, the WindowsBinary, or a Skipped note) onto res.
func (res *ISSResult) addFileLine(line, issDir, baseDir string, expand func(string) string) {
	attrs := parseInnoAttrs(line)
	rawSource := expand(attrs["Source"])
	if rawSource == "" {
		return
	}

	destDir, ok := innoDestDir(expand(attrs["DestDir"]))
	if !ok {
		res.Skipped = append(res.Skipped, "[Files] unsupported DestDir: "+attrs["DestDir"]+" (Source: "+rawSource+")")
		return
	}

	flags := strings.Fields(attrs["Flags"])
	recursive := containsStr(flags, "recursesubdirs")

	sourceSlash := toSlashPath(rawSource)
	switch {
	case strings.HasSuffix(sourceSlash, "/*") || strings.HasSuffix(sourceSlash, "/*.*"):
		if !recursive {
			res.Skipped = append(res.Skipped, "[Files] wildcard without recursesubdirs isn't representable: "+rawSource)
			return
		}
		dirSlash := strings.TrimSuffix(strings.TrimSuffix(sourceSlash, "/*.*"), "/*")
		rel, ok := relInnoPath(issDir, baseDir, dirSlash)
		if !ok {
			res.Skipped = append(res.Skipped, "[Files] Source is outside the project: "+rawSource)
			return
		}
		res.Payload = append(res.Payload, projectscan.PayloadCandidate{
			Source:    rel,
			Dest:      destDir,
			Recursive: true,
			OS:        payloadOS(rel, destDir),
			Excludes:  innoExcludes(expand(attrs["Excludes"])),
		})
	case strings.Contains(sourceSlash, "*"):
		res.Skipped = append(res.Skipped, "[Files] unsupported wildcard: "+rawSource)
	default:
		rel, ok := relInnoPath(issDir, baseDir, sourceSlash)
		if !ok {
			res.Skipped = append(res.Skipped, "[Files] Source is outside the project: "+rawSource)
			return
		}
		if res.ExeName != "" && filepath.Base(rel) == res.ExeName {
			res.WindowsBinary = rel
			return
		}
		if rel == res.LicenseFile || rel == res.IconICO {
			return // already modeled elsewhere — don't propose it twice
		}
		res.Payload = append(res.Payload, projectscan.PayloadCandidate{
			Source: rel,
			Dest:   destDir,
			OS:     payloadOS(rel, destDir),
		})
	}
}

// payloadOS decides whether an imported Payload candidate should be
// restricted to windows. A .iss is inherently a Windows-only artifact, but
// that doesn't mean every file it lists is Windows-specific content — this
// project's own package.sh bundles the very same ReleaseNotes.txt/LICENSE/
// assets/images into its macOS and Linux packages too, so defaulting every
// imported candidate to windows-only would silently drop them from a
// later mac/linux build. The one real exception is the KrankyBear
// template's own Mesa3D software-OpenGL fallback (see this repo's own
// CLAUDE.md "Mesa3D OpenGL fallback (Windows)" section) — a genuinely
// Windows-only feature, always staged under a "mesa-fallback"/"mesa-win"
// path by every project that carries it, which is the one signal worth
// keying off of here.
func payloadOS(source, dest string) []string {
	if strings.Contains(strings.ToLower(source), "mesa") || strings.Contains(strings.ToLower(dest), "mesa") {
		return []string{"windows"}
	}
	return nil
}

// relInnoPath resolves an Inno path (backslash-separated, "..\"-relative to
// issDir) against issDir, then re-expresses it relative to baseDir with
// forward slashes — matching how the rest of packproject stores paths. ok
// is false if the resolved path falls outside baseDir entirely.
//
// baseDir is resolved to absolute here too (issDir already is, by the time
// ParseISS calls this) — filepath.Rel refuses to compare one absolute and
// one relative path outright, so a relative baseDir would otherwise make
// every single path in the .iss look like it falls outside the project.
func relInnoPath(issDir, baseDir, innoPath string) (rel string, ok bool) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", false
	}
	abs := filepath.Join(issDir, filepath.FromSlash(toSlashPath(innoPath)))
	r, err := filepath.Rel(absBase, abs)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", false
	}
	return filepath.ToSlash(r), true
}

// innoDestDir strips a leading "{app}" token from an Inno DestDir value and
// converts it to a forward-slash, baseDir-relative-style path ("" for the
// install root itself). ok is false for any other Inno path constant
// ("{autopf}", "{userappdata}", ...), which isn't representable.
func innoDestDir(destDir string) (dir string, ok bool) {
	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return "", true
	}
	if strings.Contains(destDir, "{") {
		if !strings.HasPrefix(destDir, "{app}") {
			return "", false
		}
		destDir = strings.TrimPrefix(destDir, "{app}")
	}
	destDir = strings.TrimPrefix(toSlashPath(destDir), "/")
	return destDir, true
}

// innoExcludes splits Inno's own ";"-separated Excludes attribute value
// into individual patterns, converted to forward slashes.
func innoExcludes(raw string) []string {
	var out []string
	for _, pat := range strings.Split(raw, ";") {
		if pat = toSlashPath(strings.TrimSpace(pat)); pat != "" {
			out = append(out, pat)
		}
	}
	return out
}

func toSlashPath(s string) string { return strings.ReplaceAll(s, `\`, "/") }

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
