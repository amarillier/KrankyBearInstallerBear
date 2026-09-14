package main

// importmerge.go holds the GUI/CLI-shared logic behind "Import Existing
// Config" (see importconfig.go for the GUI dialog and cli.go's cmdImport
// for the CLI command). Deliberately has NO fyne.io/fyne import of its own
// — cli.go's own header comment explains why cli.go itself must stay
// fyne-free; keeping this file fyne-free too means the CLI's import
// command doesn't pull in anything the GUI dialog needs but a headless
// build/CI run never would.

import (
	"os"
	"path/filepath"
	"strings"

	"installerbear/internal/innoimport"
	"installerbear/internal/packproject"
	"installerbear/internal/projectscan"
)

// importField is one Identity/Windows/Linux value an import proposed —
// current is whatever the target project already has (for display only),
// proposed is the imported value, and apply writes proposed into the right
// field of whatever *packproject.Project it's given. Building these as
// closures keeps buildImportFields a single flat list instead of a long
// switch in both the GUI dialog and the CLI's apply step.
type importField struct {
	label             string
	current, proposed string
	apply             func(*packproject.Project)
}

// parseProjectISS locates and parses the .iss belonging to a project being
// imported: pkgRes's own KB_INNO_ISS (resolved against searchDir) when
// build-config.sh set one, else a same-convention fallback glob for
// searchDir/Inno/*.iss (the layout every KrankyBear-template project uses,
// build-config.sh or not). Returns a zero ISSResult, no error, if neither
// turns up a real file — a project with no .iss at all is a normal
// outcome, just like every other best-effort scan here.
//
// baseDir is what every resulting path is expressed relative to (matching
// packproject.Project.BaseDir's own convention) — kept separate from
// searchDir since the directory being imported from and the project file
// being written to don't have to be the same directory (the CLI's -source
// and -project can legitimately differ; the GUI passes the same folder for
// both in the common case, falling back to it when the current project has
// no BaseDir of its own yet).
func parseProjectISS(searchDir, baseDir string, pkgRes innoimport.PkgConfigResult) (innoimport.ISSResult, error) {
	issPath := ""
	if pkgRes.ISSPath != "" {
		issPath = filepath.Join(searchDir, filepath.FromSlash(pkgRes.ISSPath))
	} else if matches, _ := filepath.Glob(filepath.Join(searchDir, "Inno", "*.iss")); len(matches) > 0 {
		issPath = matches[0]
	}
	if issPath == "" {
		return innoimport.ISSResult{}, nil
	}
	if _, err := os.Stat(issPath); err != nil {
		return innoimport.ISSResult{}, nil
	}
	return innoimport.ParseISS(issPath, baseDir)
}

// buildImportFields merges pkg and iss into the list of fields worth
// proposing, comparing each against current's existing value. Preferring
// iss's own value over pkg's when both propose one for the same field —
// the .iss is the actual shipped-installer config, generally the more
// authoritative record of what's really out there, while build-config.sh
// is the developer-facing source of truth that's supposed to match it. A
// field with nothing new to propose (blank, or identical to what current
// already has) is left out entirely rather than shown as a no-op.
func buildImportFields(current *packproject.Project, pkg innoimport.PkgConfigResult, iss innoimport.ISSResult) []importField {
	var fields []importField
	add := func(label, curVal, proposed string, apply func(*packproject.Project)) {
		if proposed == "" || proposed == curVal {
			return
		}
		fields = append(fields, importField{label: label, current: curVal, proposed: proposed, apply: apply})
	}

	name := firstNonEmptyStr(iss.Name, pkg.Name)
	add("Name", current.Identity.Name, name, func(p *packproject.Project) { p.Identity.Name = name })

	version := firstNonEmptyStr(iss.Version, pkg.Version)
	add("Version", current.Identity.Version, version, func(p *packproject.Project) { p.Identity.Version = version })

	add("ID", current.Identity.ID, pkg.BundleID, func(p *packproject.Project) { p.Identity.ID = pkg.BundleID })

	publisher := firstNonEmptyStr(iss.Publisher, pkg.Publisher)
	add("Publisher", current.Identity.Publisher, publisher, func(p *packproject.Project) { p.Identity.Publisher = publisher })

	add("Vendor", current.Identity.Vendor, pkg.Vendor, func(p *packproject.Project) { p.Identity.Vendor = pkg.Vendor })

	url := firstNonEmptyStr(iss.URL, pkg.URL)
	add("URL", current.Identity.URL, url, func(p *packproject.Project) { p.Identity.URL = url })

	add("Description", current.Identity.Description, pkg.Description, func(p *packproject.Project) { p.Identity.Description = pkg.Description })

	add("License", current.Identity.License, pkg.License, func(p *packproject.Project) { p.Identity.License = pkg.License })

	add("License file", current.Identity.LicenseFile, iss.LicenseFile, func(p *packproject.Project) { p.Identity.LicenseFile = iss.LicenseFile })

	add("Windows icon (.ico)", current.Identity.Icons.ICO, iss.IconICO, func(p *packproject.Project) { p.Identity.Icons.ICO = iss.IconICO })

	add("Windows Upgrade GUID", current.Windows.UpgradeGUID, iss.UpgradeGUID, func(p *packproject.Project) { p.Windows.UpgradeGUID = iss.UpgradeGUID })

	add("Windows exe name", current.Windows.ExeName, iss.ExeName, func(p *packproject.Project) { p.Windows.ExeName = iss.ExeName })

	add("Linux desktop comment", current.Linux.DesktopComment, pkg.DesktopComment, func(p *packproject.Project) { p.Linux.DesktopComment = pkg.DesktopComment })

	add("Linux desktop categories", current.Linux.DesktopCategories, pkg.DesktopCategories, func(p *packproject.Project) { p.Linux.DesktopCategories = pkg.DesktopCategories })

	if iss.WindowsBinary != "" {
		curBin, _ := current.BinaryFor("windows", "amd64")
		add("Windows binary (amd64)", curBin.Path, iss.WindowsBinary, func(p *packproject.Project) {
			upsertBinary(p, "windows", "amd64", iss.WindowsBinary)
		})
	}

	// InstallExperience toggles recognized from [Tasks]/[Run] (see
	// installexperience.go) - addToggle only ever proposes turning one on
	// (never off), and only when it isn't already on, matching add()'s own
	// "nothing new to propose is left out entirely" rule but for bools
	// instead of strings. current is deliberately "" rather than "off" -
	// the same reason FileAssociation candidates use "" below: a non-empty
	// current reads to cmdImport as "already set, skip unless -overwrite",
	// which would silently skip every one of these on a plain `import`
	// with no flags (found via a real end-to-end CLI test failure, not
	// assumed).
	addToggle := func(label string, curVal, proposed bool, apply func(*packproject.Project)) {
		if !proposed || curVal {
			return
		}
		fields = append(fields, importField{label: label, current: "", proposed: "enable", apply: apply})
	}
	addToggle("Desktop shortcut (Install Experience)", current.InstallExperience.DesktopShortcut, iss.DesktopShortcut,
		func(p *packproject.Project) { p.InstallExperience.DesktopShortcut = true })
	addToggle("Run at startup (Install Experience)", current.InstallExperience.AutostartAtLogin, iss.AutostartAtLogin,
		func(p *packproject.Project) { p.InstallExperience.AutostartAtLogin = true })
	addToggle("Launch after install (Install Experience)", current.InstallExperience.LaunchAfterInstall, iss.LaunchAfterInstall,
		func(p *packproject.Project) { p.InstallExperience.LaunchAfterInstall = true })

	// FileAssociations don't fit add()'s "one scalar value, maybe
	// overwrite" shape (a project can have any number of them, each an
	// independent add-or-skip, the same shape Payload candidates already
	// have) - built as plain importFields directly instead, one per new
	// candidate, each with current == "" so both the GUI (defaults the
	// checkbox to checked) and the CLI (treats it as "fill", not gated
	// behind -overwrite) read it the same way a brand-new list item
	// should behave, not like overwriting an existing scalar.
	for _, assoc := range newFileAssociationCandidates(current.FileAssociations, iss.FileAssociations) {
		assoc := assoc
		proposed := assoc.Extension
		if assoc.Description != "" {
			proposed += " (" + assoc.Description + ")"
		}
		fields = append(fields, importField{
			label:    "File association",
			current:  "",
			proposed: proposed,
			apply: func(p *packproject.Project) {
				p.FileAssociations = append(p.FileAssociations, assoc)
			},
		})
	}

	return fields
}

// newFileAssociationCandidates filters candidates down to ones whose
// Extension isn't already registered in existing - the same
// already-present de-duplication newPayloadCandidates already does for
// Payload, so re-running an import doesn't pile up duplicate associations
// (matched case-insensitively, matching packproject.Validate's own
// duplicate-extension check).
func newFileAssociationCandidates(existing, candidates []packproject.FileAssociation) []packproject.FileAssociation {
	haveExt := make(map[string]bool, len(existing))
	for _, e := range existing {
		haveExt[strings.ToLower(e.Extension)] = true
	}

	var out []packproject.FileAssociation
	for _, c := range candidates {
		if haveExt[strings.ToLower(c.Extension)] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// newPayloadCandidates filters candidates down to ones whose Source isn't
// already the Source of an entry in existing — the CLI import path's own
// de-duplication, so running `installerbear import` again against the same
// project doesn't pile up duplicate Payload entries on every run (the GUI
// has a review dialog step where a human notices and unchecks a repeat;
// the CLI has no such step, so it needs to be safe to re-run unattended).
func newPayloadCandidates(existing []packproject.PayloadEntry, candidates []projectscan.PayloadCandidate) []projectscan.PayloadCandidate {
	haveSource := make(map[string]bool, len(existing))
	for _, e := range existing {
		haveSource[e.Source] = true
	}

	var out []projectscan.PayloadCandidate
	for _, c := range candidates {
		if haveSource[c.Source] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// upsertBinary sets or replaces the single BinaryEntry for (osName, arch) —
// the plain *packproject.Project analogue of binariesform.go's own
// e.setBinary, needed here since buildImportFields' apply closures work
// against a Project directly rather than through the live editor.
func upsertBinary(p *packproject.Project, osName, arch, path string) {
	for i, b := range p.Binaries {
		if b.OS == osName && b.Arch == arch {
			p.Binaries[i].Path = path
			return
		}
	}
	p.Binaries = append(p.Binaries, packproject.BinaryEntry{OS: osName, Arch: arch, Path: path})
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
