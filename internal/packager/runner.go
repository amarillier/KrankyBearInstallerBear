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
func Run(ctx context.Context, proj *packproject.Project, pkgrs []Packager, onEvent ProgressFunc) []BuildResult {
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
