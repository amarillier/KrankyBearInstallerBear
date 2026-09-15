package packager

import (
	"context"
	"fmt"

	"installerbear/internal/packproject"
)

// targetOS maps a target to the OS its binary/arch selection is driven by
// (macpkg by darwin binaries, deb/rpm by linux binaries, winexe/winmsi by
// windows binaries) — see Run's multi-arch loop below.
func targetOS(t Target) string {
	switch t {
	case TargetDEB, TargetRPM:
		return "linux"
	case TargetMacPkg:
		return "darwin"
	case TargetWinExe, TargetWinMSI:
		return "windows"
	default:
		return ""
	}
}

// Run builds every target in pkgrs against proj, in order. One target
// failing (at HostSupported, Preflight, or Build) never stops the others —
// e.g. a missing makensis on this host still lets deb/rpm succeed. Each
// target's HostSupported()/Preflight()/Build() outcome is reported through
// onEvent as it happens, in addition to being returned in the result slice.
//
// A target builds once per architecture registered for its OS (see
// packproject.ArchesFor) — register both a darwin/amd64 and a darwin/arm64
// binary and checking macpkg produces two .pkg files, no separate arch
// picker needed. A target whose OS has no binary registered at all still
// gets one Build attempt with an empty arch, so the backend's own
// "no <os>/<arch> binary registered" error surfaces clearly rather than
// the target silently vanishing from the results.
//
// Once every target/arch has been attempted, proj.Output.PostBuildHook (if
// set) runs once against the full results slice — see runPostBuildHook's
// own doc comment. This is the one orchestration point both the CLI and
// the GUI already share, so neither caller needs to remember a separate
// step to get it.
func Run(ctx context.Context, proj *packproject.Project, pkgrs []Packager, onEvent ProgressFunc) []BuildResult {
	// Resolve any Payload glob pattern (see packproject.ExpandedPayload's
	// own doc comment) into its concrete matches once, up front, on a
	// shallow copy of proj — never the caller's own *proj, whose Payload
	// is what the GUI edits and saves back to installerbear.yaml as the
	// original pattern text, not today's expansion of it. Every backend's
	// own Build below only ever sees the already-expanded, literal
	// result, exactly as if the project had been authored with each
	// match spelled out by hand - no backend needs to know wildcards
	// exist at all. Validate (already run by both the GUI and the CLI
	// before Run is ever reached) exercises the same expansion, so a
	// bad glob pattern should already have been caught there; this is
	// just defensive.
	expanded, err := proj.ExpandedPayload()
	if err != nil {
		var results []BuildResult
		for _, pkgr := range pkgrs {
			results = append(results, BuildResult{Target: pkgr.Target(), Err: fmt.Errorf("expanding payload: %w", err)})
		}
		return results
	}
	projCopy := *proj
	projCopy.Payload = expanded
	proj = &projCopy

	var results []BuildResult

	for _, pkgr := range pkgrs {
		target := pkgr.Target()

		if err := pkgr.HostSupported(); err != nil {
			emit(onEvent, target, fmt.Sprintf("skipped: %v", err))
			results = append(results, BuildResult{Target: target, Err: err, Skipped: true})
			continue
		}
		if err := pkgr.Preflight(); err != nil {
			emit(onEvent, target, fmt.Sprintf("skipped: %v", err))
			results = append(results, BuildResult{Target: target, Err: err, Skipped: true})
			continue
		}

		arches := proj.ArchesFor(targetOS(target))
		if len(arches) == 0 {
			arches = []string{""}
		}

		for _, arch := range arches {
			tag := arch
			if tag == "" {
				tag = "default"
			}
			emit(onEvent, target, fmt.Sprintf("building (%s)...", tag))
			outPath, err := pkgr.Build(ctx, proj, BuildOptions{OutputDir: proj.Output.Dir, Arch: arch}, onEvent)
			if err != nil {
				emit(onEvent, target, fmt.Sprintf("failed (%s): %v", tag, err))
			} else {
				emit(onEvent, target, fmt.Sprintf("done (%s): %s", tag, outPath))
			}
			results = append(results, BuildResult{Target: target, Arch: arch, OutputPath: outPath, Err: err})
		}
	}

	runPostBuildHook(ctx, proj, results, onEvent)

	return results
}

// Summary classifies a Run's results for exit-code purposes: 0 all
// succeeded, 1 all failed/skipped, 2 partial.
func Summary(results []BuildResult) (succeeded, failed int) {
	for _, r := range results {
		if r.Err == nil && !r.Skipped {
			succeeded++
		} else {
			failed++
		}
	}
	return succeeded, failed
}

// ExitCode maps a Run's results to the CLI's documented exit codes.
func ExitCode(results []BuildResult) int {
	succeeded, failed := Summary(results)
	switch {
	case failed == 0:
		return 0
	case succeeded == 0:
		return 1
	default:
		return 2
	}
}

func emit(onEvent ProgressFunc, target Target, line string) {
	if onEvent != nil {
		onEvent(target, line)
	}
}
