package main

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packager"
)

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

	top := container.NewVBox(
		sectionHeader("Targets"),
		targetRows,
		refreshBtn,
		widget.NewSeparator(),
		container.NewHBox(e.startBtn, e.cancelBtn, e.buildStatus),
	)

	logScroll := container.NewVScroll(e.logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 240))

	return container.NewBorder(top, nil, nil, nil, container.NewBorder(
		widget.NewLabelWithStyle("Build Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, logScroll,
	))
}

func (e *editor) refreshBuildTab() {
	e.refreshTargetChecks()
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
}

func (e *editor) cancelBuild() {
	if e.build.Cancel() {
		e.buildStatus.SetText("canceling...")
	}
}
