package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// targetLabels gives each target name a friendlier display than the bare
// config string.
var targetLabels = map[string]string{
	packproject.TargetDEB:    "Linux .deb",
	packproject.TargetRPM:    "Linux .rpm",
	packproject.TargetMacPkg: "macOS .pkg",
	packproject.TargetWinExe: "Windows Setup.exe",
	packproject.TargetWinMSI: "Windows .msi",
}

// buildTargetRows lays out one Check + status Label per known target, in
// packproject.AllTargets order. Preflight status is checked in a goroutine
// (never on the UI thread) and applied via fyne.Do.
func (e *editor) buildTargetRows() fyne.CanvasObject {
	rows := container.NewVBox()
	for _, name := range packproject.AllTargets {
		name := name // capture per iteration
		check := widget.NewCheck(targetLabels[name], func(checked bool) { e.toggleTarget(name, checked) })
		status := widget.NewLabel("checking...")
		e.targetChecks[name] = check
		e.targetStatus[name] = status
		rows.Add(container.NewBorder(nil, nil, check, status, nil))
	}
	return rows
}

// toggleTarget adds/removes name from proj.Targets, idempotently — safe to
// call from a SetChecked-triggered OnChanged during refresh as well as a
// real user click, so no e.loading guard is needed here (unlike
// binariesform.go's setBinary).
func (e *editor) toggleTarget(name string, checked bool) {
	has := false
	idx := -1
	for i, t := range e.proj.Targets {
		if t == name {
			has = true
			idx = i
			break
		}
	}
	switch {
	case checked && !has:
		e.proj.Targets = append(e.proj.Targets, name)
	case !checked && has:
		e.proj.Targets = append(e.proj.Targets[:idx], e.proj.Targets[idx+1:]...)
	}
}

// refreshTargetChecks sets each checkbox from proj.Targets and kicks off a
// fresh preflight check for every target.
func (e *editor) refreshTargetChecks() {
	selected := make(map[string]bool, len(e.proj.Targets))
	for _, t := range e.proj.Targets {
		selected[t] = true
	}
	for name, check := range e.targetChecks {
		check.SetChecked(selected[name])
	}
	e.checkAllPreflight()
}

// disablePreflightAutoCheck lets tests skip spawning checkAllPreflight's
// background goroutine entirely, so a test can close its window right after
// building the editor without racing that goroutine's later fyne.Do calls
// against the now-torn-down canvas. Never set outside tests.
var disablePreflightAutoCheck = false

// checkAllPreflight runs every target's HostSupported/Preflight off the UI
// thread, one at a time in a single goroutine (deliberately not one
// goroutine per target: several concurrent fyne.Do calls landing at once
// was enough to crash Fyne's text-shaping layer under real testing here —
// sequential is both simpler and avoids that entirely), reporting each
// result via its own fyne.Do as it completes.
func (e *editor) checkAllPreflight() {
	if disablePreflightAutoCheck {
		return
	}
	go func() {
		for _, name := range packproject.AllTargets {
			text, ok := preflightStatus(name)
			fyne.Do(func() {
				label := e.targetStatus[name]
				label.SetText(text)
				if ok {
					label.Importance = widget.SuccessImportance
				} else {
					label.Importance = widget.DangerImportance
				}
				label.Refresh()
			})
		}
	}()
}

// preflightStatus mirrors cmdDoctor's own logic in cli.go so the GUI and CLI
// can never disagree about whether a target is ready.
func preflightStatus(name string) (text string, ready bool) {
	pkgrs, err := packager.Resolve([]packager.Target{packager.Target(name)})
	if err != nil {
		return "not implemented yet", false
	}
	p := pkgrs[0]
	if err := p.HostSupported(); err != nil {
		return "unsupported here: " + err.Error(), false
	}
	if err := p.Preflight(); err != nil {
		return "missing tool: " + err.Error(), false
	}
	return "ready", true
}
