package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packager"
)

// buildLogTheme keeps the build log fully legible: a disabled widget.Entry
// (used here so the log reads as non-editable) renders its text in
// theme.ColorNameDisabled, which both bundled themes deliberately make
// near-invisible against their own background (that's the "grayed out"
// look, by design, for actually-disabled controls) — not what a read-only
// log display wants. Overriding just this one color, scoped to this single
// widget via container.NewThemeOverride, fixes the log without touching
// disabled buttons/checkboxes anywhere else in the app.
type buildLogTheme struct{ fyne.Theme }

func (t *buildLogTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameDisabled {
		return t.Theme.Color(theme.ColorNameForeground, v)
	}
	return t.Theme.Color(name, v)
}

// buildBuildTab is the target checklist plus Start/Cancel and a streaming
// log. Starting a build launches packager.Run on a goroutine and returns
// immediately — Start's OnTapped body must never block, or it freezes
// Fyne's whole event loop (the GUI equivalent of the quit-hang bug this
// project's own CLAUDE.md documents for the audio-player case).
func (e *editor) buildBuildTab() fyne.CanvasObject {
	targetRows := e.buildTargetRows()
	refreshBtn := widget.NewButton("Re-check Tools", func() { e.checkAllPreflight() })

	e.logEntry = widget.NewMultiLineEntry()
	e.logEntry.Disable()
	e.logEntry.Wrapping = fyne.TextWrapWord

	e.startBtn = widget.NewButton("Start Build", func() { e.startBuild() })
	e.startBtn.Importance = widget.SuccessImportance
	e.cancelBtn = widget.NewButton("Cancel", func() { e.cancelBuild() })
	e.cancelBtn.Disable()
	e.buildStatus = widget.NewLabel("")

	e.cleanOldVersionsCheck = widget.NewCheck("Remove older-version installers from the output folder after a successful build", func(v bool) {
		e.proj.Output.CleanOldVersions = v
	})

	copyLogBtn := widget.NewButton("Copy Log", func() { e.copyBuildLog() })
	saveLogBtn := widget.NewButton("Save Log...", func() { e.saveBuildLog() })

	top := container.NewVBox(
		sectionHeader("Targets"),
		targetRows,
		refreshBtn,
		widget.NewSeparator(),
		container.NewHBox(e.startBtn, e.cancelBtn, e.buildStatus),
		e.cleanOldVersionsCheck,
	)

	logDisplay := container.NewThemeOverride(e.logEntry, &buildLogTheme{Theme: theme.Current()})
	logScroll := container.NewVScroll(logDisplay)
	logScroll.SetMinSize(fyne.NewSize(0, 240))

	return container.NewBorder(top, nil, nil, nil, container.NewBorder(
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Build Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(copyLogBtn, saveLogBtn),
		),
		nil, nil, nil, logScroll,
	))
}

func (e *editor) refreshBuildTab() {
	e.refreshTargetChecks()
	e.cleanOldVersionsCheck.SetChecked(e.proj.Output.CleanOldVersions)
}

// copyBuildLog puts the build log's current full text on the system
// clipboard — via the app rather than the deprecated Window.Clipboard(),
// same clipboard either way.
func (e *editor) copyBuildLog() {
	e.app.Clipboard().SetContent(e.logEntry.Text)
}

// saveBuildLog writes the build log's current full text to a file the user
// picks — handy for attaching to a bug report once a build's already
// scrolled past what's comfortable to copy/paste by hand.
func (e *editor) saveBuildLog() {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		if _, err := writer.Write([]byte(e.logEntry.Text)); err != nil {
			dialog.ShowError(err, e.win)
		}
	}, e.win)
	fd.SetFileName("build-log.txt")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".log"}))
	fd.Show()
}

// startBuild validates+resolves everything synchronously (fast, no I/O
// beyond a filesystem stat or two) so a bad config fails immediately with a
// clear dialog, then hands off to a goroutine for the actual build.
func (e *editor) startBuild() {
	if e.build.IsRunning() {
		return
	}

	e.proj.Defaults()
	if err := e.proj.Validate(e.proj.BaseDir); err != nil {
		dialog.ShowError(err, e.win)
		return
	}

	targets := make([]packager.Target, 0, len(e.proj.Targets))
	for _, t := range e.proj.Targets {
		targets = append(targets, packager.Target(t))
	}
	if len(targets) == 0 {
		dialog.ShowError(fmt.Errorf("select at least one target on the Build tab"), e.win)
		return
	}
	pkgrs, err := packager.Resolve(targets)
	if err != nil {
		dialog.ShowError(err, e.win)
		return
	}

	ctx, ok := e.build.Start(context.Background())
	if !ok {
		return
	}

	e.logEntry.SetText("")
	e.startBtn.Disable()
	e.startBtn.Importance = widget.WarningImportance
	e.startBtn.Refresh()
	e.cancelBtn.Enable()
	e.buildStatus.SetText("building...")

	go e.runBuild(ctx, pkgrs)
}

// runBuild does the actual work off the UI thread; every touch of a widget
// is wrapped in fyne.Do (this project's CLAUDE.md: "every UI update from a
// non-main goroutine must be wrapped in fyne.Do").
func (e *editor) runBuild(ctx context.Context, pkgrs []packager.Packager) {
	results := packager.Run(ctx, e.proj, pkgrs, func(target packager.Target, line string) {
		e.build.AppendLog(fmt.Sprintf("[%s] %s", target, line))
		snapshot := e.build.LogSnapshot()
		fyne.Do(func() { e.logEntry.SetText(strings.Join(snapshot, "\n")) })
	})
	e.build.Finish()

	succeeded, failed := packager.Summary(results)
	fyne.Do(func() {
		e.startBtn.Enable()
		if failed == 0 {
			e.startBtn.Importance = widget.SuccessImportance
		} else {
			e.startBtn.Importance = widget.DangerImportance
		}
		e.startBtn.Refresh()
		e.cancelBtn.Disable()
		e.buildStatus.SetText(fmt.Sprintf("%d succeeded, %d failed/skipped", succeeded, failed))
	})

	if succeeded > 0 {
		e.offerCleanupOldInstallers(results)
	}
}

func (e *editor) cancelBuild() {
	if e.build.Cancel() {
		e.buildStatus.SetText("canceling...")
	}
}

// offerCleanupOldInstallers is a no-op unless the Build tab's checkbox is
// on. When it is, and a just-finished build turned up sibling files in the
// output folder that look like an older-version build of the same
// installer (e.g. a leftover 0.2.0 .deb once the project's moved on to
// 0.3.0 — see packager.StaleInstallers), it asks before deleting anything:
// removing files is not undoable, so an opt-in checkbox alone isn't
// enough — the user still sees and confirms the exact list every time.
func (e *editor) offerCleanupOldInstallers(results []packager.BuildResult) {
	if !e.proj.Output.CleanOldVersions {
		return
	}
	stale := packager.StaleInstallers(results, e.proj.Identity.Version)
	if len(stale) == 0 {
		return
	}

	fyne.Do(func() {
		msg := "Remove these older-version installer file(s) from the output folder?\n\n" + strings.Join(stale, "\n")
		dialog.ShowConfirm("Clean Up Old Installers", msg, func(ok bool) {
			if !ok {
				return
			}
			for _, path := range stale {
				if err := os.Remove(path); err != nil {
					e.build.AppendLog(fmt.Sprintf("[cleanup] failed to remove %s: %v", path, err))
				} else {
					e.build.AppendLog(fmt.Sprintf("[cleanup] removed %s", path))
				}
			}
			e.logEntry.SetText(strings.Join(e.build.LogSnapshot(), "\n"))
		}, e.win)
	})
}
