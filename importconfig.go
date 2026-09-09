package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/innoimport"
	"installerbear/internal/projectscan"
)

// importExistingConfig lets the user point at an existing project's root
// folder and best-effort-imports its Inno .iss and/or KrankyBear-template
// build-config.sh/package.sh into the current project — the "huge help"
// migration path onto InstallerBear for a project currently packaged by
// hand (see internal/innoimport's own package doc for exactly what's
// understood, and importmerge.go for the GUI/CLI-shared merge logic; the
// CLI's own equivalent is cli.go's cmdImport). Unlike New Project's silent
// git/ReleaseNotes/icon scan, nothing here is applied until the user
// confirms via the review dialogs below — an import can legitimately want
// to *overwrite* an existing value, not just fill a blank one.
func (e *editor) importExistingConfig() {
	dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		dir := u.Path()

		// Paths are expressed relative to the current project's own
		// BaseDir where it has one already (New/Open already set it); a
		// never-saved project has none yet, so fall back to the folder
		// just picked — the common case anyway, since that's usually the
		// same project's own root.
		baseDir := e.proj.BaseDir
		if baseDir == "" {
			baseDir = dir
		}

		pkgRes, err := innoimport.ParseTemplatePackageConfig(dir)
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}

		issRes, issErr := parseProjectISS(dir, baseDir, pkgRes)
		if issErr != nil {
			dialog.ShowError(issErr, e.win)
			return
		}

		fields := buildImportFields(e.proj, pkgRes, issRes)
		payload := newPayloadCandidates(e.proj.Payload, issRes.Payload)
		skipped := append(append([]string{}, pkgRes.Skipped...), issRes.Skipped...)

		if len(fields) == 0 && len(payload) == 0 {
			msg := "Nothing recognizable was found in that folder (no build-config.sh or .iss with new values)."
			if len(skipped) > 0 {
				msg = "No importable fields found. " + fmt.Sprintf("(%d note(s) below.)", len(skipped))
			}
			dialog.ShowInformation("Import Existing Config", msg, e.win)
			return
		}

		e.showImportReviewDialog(fields, payload, skipped)
	}, e.win).Show()
}

// showImportReviewDialog lists each proposed field as current -> proposed
// with a checkbox (pre-checked only when there was no existing value, so
// overwriting one already set is a conscious choice), plus a trailing
// "not imported" section for anything found but skipped. Only checked
// fields are applied on confirm. If any Payload candidates came from the
// .iss, they're reviewed next via the Payload tab's own existing
// checkbox-list dialog (payloadtable.go's showPayloadScanReviewDialog) —
// reused as-is rather than duplicated, since it already does exactly this
// for a directory scan's candidates.
func (e *editor) showImportReviewDialog(fields []importField, payload []projectscan.PayloadCandidate, skipped []string) {
	type row struct {
		field   importField
		include *widget.Check
	}

	rows := make([]row, len(fields))
	form := container.NewVBox()
	for i, f := range fields {
		curDisplay := f.current
		if curDisplay == "" {
			curDisplay = "(blank)"
		}
		include := widget.NewCheck(fmt.Sprintf("%s: %s -> %s", f.label, curDisplay, f.proposed), nil)
		include.SetChecked(f.current == "")
		rows[i] = row{field: f, include: include}
		form.Add(include)
	}

	if len(skipped) > 0 {
		form.Add(sectionHeader("Not imported"))
		for _, s := range skipped {
			l := widget.NewLabel("• " + s)
			l.Wrapping = fyne.TextWrapWord
			form.Add(l)
		}
	}

	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(560, 360))

	dialog.ShowCustomConfirm("Review Imported Config", "Apply Selected", "Cancel", scroll, func(ok bool) {
		if !ok {
			return
		}
		for _, r := range rows {
			if r.include.Checked {
				r.field.apply(e.proj)
			}
		}
		e.refreshAll()

		if len(payload) > 0 {
			e.showPayloadScanReviewDialog(payload)
		}
	}, e.win)
}
