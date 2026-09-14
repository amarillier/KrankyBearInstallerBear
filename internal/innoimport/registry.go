package innoimport

import (
	"regexp"
	"strings"

	"installerbear/internal/packproject"
)

// classKeyRE matches a bare "Software\Classes\<name>" subkey with no
// further path segments - the ProgID's own root key, where Inno writes
// the file type's human-readable description as the (Default) value
// (ValueName == "").
var classKeyRE = regexp.MustCompile(`(?i)^Software\\Classes\\([^\\]+)$`)

// openWithProgIDsRE matches "Software\Classes\<ext>\OpenWithProgids" -
// the modern Inno-wizard-generated association shape (confirmed against
// this repo's own real Inno/KrankyBearInstallerBear.iss fixture), where
// ValueName carries the ProgID being associated with <ext>.
var openWithProgIDsRE = regexp.MustCompile(`(?i)^Software\\Classes\\(\.[^\\]+)\\OpenWithProgids$`)

// shellOpenCommandRE matches "Software\Classes\<name>\shell\open\command"
// - present for a real, launchable file-type association. Used here only
// as a sanity check that a bare class key found via classKeyRE really is
// an app's file-type ProgID, not some unrelated registry entry that
// happens to have an empty ValueName and a non-blank ValueData.
var shellOpenCommandRE = regexp.MustCompile(`(?i)^Software\\Classes\\([^\\]+)\\shell\\open\\command$`)

// bareShellOpenCommandRE is shellOpenCommandRE's classic-shape
// counterpart: "<ProgID>\shell\open\command" with no "Software\Classes\"
// prefix, root-gated to HKCR by its only caller (the same reasoning as
// the classic class key above - HKCR itself already is the classes root).
var bareShellOpenCommandRE = regexp.MustCompile(`(?i)^([^\\]+)\\shell\\open\\command$`)

// directExtRE matches the older, simpler direct HKCR\.ext form (no
// Software\Classes prefix, no OpenWithProgids indirection) - not
// confirmed against a real fixture in this repo (the one committed here
// uses the newer OpenWithProgids form instead), but a well-documented,
// commonly hand-written alternative, so both are recognized rather than
// assuming every real-world .iss uses the newer wizard-generated shape.
var directExtRE = regexp.MustCompile(`^\.[A-Za-z0-9]+$`)

// parseFileAssociations recognizes a classic Inno Setup file-association
// [Registry] block and turns it into packproject.FileAssociation values.
// Two shapes are understood:
//
//  1. The modern Inno-wizard-generated form:
//     Root: HKA; Subkey: "Software\Classes\.ext\OpenWithProgids"; ValueName: "ProgID"; ...
//     Root: HKA; Subkey: "Software\Classes\ProgID"; ValueName: ""; ValueData: "Description"; ...
//     Root: HKA; Subkey: "Software\Classes\ProgID\shell\open\command"; ...
//
//  2. The older, simpler direct form some hand-written .iss scripts use
//     instead:
//     Root: HKCR; Subkey: ".ext"; ValueName: ""; ValueData: "ProgID"
//     Root: HKCR; Subkey: "ProgID"; ValueName: ""; ValueData: "Description"
//     Root: HKCR; Subkey: "ProgID\shell\open\command"; ...
//
// Both require a matching "<ProgID>\shell\open\command" line to exist
// before accepting a candidate — a bare ProgID key with an empty
// ValueName and non-blank ValueData could in principle be some other
// registry entry entirely, not necessarily a file-type association;
// requiring the shell\open\command sibling is a real, if imperfect,
// sanity check rather than accepting any coincidentally-shaped pair.
//
// A note on macro expansion: expand only resolves simple `#define NAME
// "literal"` macros (see ParseISS's own doc comment) — a real .iss whose
// ProgID or Description come from a Pascal-Script expression (Inno's own
// project wizard generates exactly this: `{#MyAppAssocKey}` expands from
// `StringChange(MyAppAssocName, " ", "") + MyAppAssocExt`, which expand
// can't evaluate) is left as the literal, unresolved "{#Token}" text
// rather than guessed at. This still correlates correctly here (the same
// unresolved token appears consistently everywhere it's referenced, so
// the association is still recognized), it just surfaces a slightly ugly
// candidate for the user to review and likely reject — exactly the
// review-before-apply safety net this whole import feature already relies
// on elsewhere, not a new problem this introduces.
//
// Returns the recognized associations plus how many of the input lines
// were NOT part of a successfully recognized, confirmed pair — the
// caller reports that remainder in ISSResult.Skipped rather than silently
// dropping it.
func parseFileAssociations(lines []string, expand func(string) string) ([]packproject.FileAssociation, int) {
	type parsedLine struct {
		root, subkey, valueName, valueData string
	}
	parsed := make([]parsedLine, 0, len(lines))
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		parsed = append(parsed, parsedLine{
			root:      strings.ToUpper(strings.TrimSpace(expand(attrs["Root"]))),
			subkey:    expand(attrs["Subkey"]),
			valueName: expand(attrs["ValueName"]),
			valueData: expand(attrs["ValueData"]),
		})
	}

	progIDsWithShellOpen := map[string]bool{}
	descriptions := map[string]string{} // ProgID -> Description
	extToProgID := map[string]string{}  // ext (with leading dot) -> ProgID
	consumed := make([]bool, len(parsed))

	// First pass: shell\open\command lines confirm a ProgID is real - both
	// the modern "Software\Classes\<ProgID>\shell\open\command" shape and
	// the classic direct shape's own bare "<ProgID>\shell\open\command"
	// (no "Software\Classes\" prefix, same reason the classic class key
	// above has none - HKCR itself already is the classes root).
	for i, p := range parsed {
		if m := shellOpenCommandRE.FindStringSubmatch(p.subkey); m != nil {
			progIDsWithShellOpen[m[1]] = true
			consumed[i] = true
		} else if p.root == "HKCR" {
			if m := bareShellOpenCommandRE.FindStringSubmatch(p.subkey); m != nil {
				progIDsWithShellOpen[m[1]] = true
				consumed[i] = true
			}
		}
	}

	// Second pass: bare class keys (Description) and ext->ProgID mappings
	// (both shapes).
	for i, p := range parsed {
		if consumed[i] {
			continue
		}
		switch {
		case classKeyRE.MatchString(p.subkey) && p.valueName == "" && p.valueData != "":
			// The modern shape's own class key: "Software\Classes\<ProgID>".
			descriptions[classKeyRE.FindStringSubmatch(p.subkey)[1]] = p.valueData
			consumed[i] = true
		case p.root == "HKCR" && !strings.Contains(p.subkey, `\`) && !strings.HasPrefix(p.subkey, ".") && p.valueName == "" && p.valueData != "":
			// The classic direct shape's own class key: HKCR itself IS the
			// classes root, so the ProgID is written as a bare subkey with
			// no "Software\Classes\" prefix at all - distinguished from the
			// extension entry just below by not starting with a dot.
			descriptions[p.subkey] = p.valueData
			consumed[i] = true
		case openWithProgIDsRE.MatchString(p.subkey) && p.valueName != "":
			ext := openWithProgIDsRE.FindStringSubmatch(p.subkey)[1]
			extToProgID[strings.ToLower(ext)] = p.valueName
			consumed[i] = true
		case p.root == "HKCR" && directExtRE.MatchString(p.subkey) && p.valueName == "" && p.valueData != "":
			extToProgID[strings.ToLower(p.subkey)] = p.valueData
			consumed[i] = true
		}
	}

	var out []packproject.FileAssociation
	usedProgIDs := map[string]bool{}
	for ext, progID := range extToProgID {
		if !progIDsWithShellOpen[progID] {
			continue // no confirmed shell\open\command sibling - not confident enough
		}
		out = append(out, packproject.FileAssociation{
			Extension:   ext,
			Description: descriptions[progID],
		})
		usedProgIDs[progID] = true
	}

	unrecognized := 0
	for _, c := range consumed {
		if !c {
			unrecognized++
		}
	}
	// A class key/shell-open-command pair that never got a matching
	// ext->ProgID mapping isn't a usable association either (e.g. the
	// extension line itself didn't match either recognized shape) -
	// count those lines as unrecognized too, rather than treating
	// "matched some sub-pattern" alone as success.
	for progID := range descriptions {
		if !usedProgIDs[progID] {
			unrecognized++
		}
	}

	return out, unrecognized
}
