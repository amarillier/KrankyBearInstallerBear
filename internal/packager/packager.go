// Package packager defines the shared backend interface every packaging
// target (deb, rpm, macpkg, winexe, winmsi) implements, plus the orchestration
// logic (Run) that both the CLI and the GUI drive identically.
//
// Nothing under this package or its subpackages may import fyne.io/fyne —
// that boundary is what keeps `packman build`/`doctor`/`validate` usable on a
// headless CI runner with no display.
package packager

import (
	"context"

	"installerbear/internal/packproject"
)

// Target names a single buildable output. These match packproject's target
// strings so config/CLI/registry never need translation between them.
type Target string

const (
	TargetDEB    Target = packproject.TargetDEB
	TargetRPM    Target = packproject.TargetRPM
	TargetMacPkg Target = packproject.TargetMacPkg
	TargetWinExe Target = packproject.TargetWinExe
	TargetWinMSI Target = packproject.TargetWinMSI
)

// BuildOptions carries the per-run overrides that don't belong in the
// checked-in project file (output directory override, architecture selection).
type BuildOptions struct {
	OutputDir string
	Arch      string
}

// ProgressFunc receives one line of build output at a time. The GUI wraps its
// implementation in fyne.Do; the CLI just prints it.
type ProgressFunc func(target Target, line string)

// Packager builds one target. HostSupported reports a hard platform
// restriction (e.g. macpkg only builds on darwin) and must be checked before
// Preflight or Build ever run. Preflight reports a missing external tool
// (e.g. no makensis on PATH). Both return nil when the target can proceed.
type Packager interface {
	Target() Target
	HostSupported() error
	Preflight() error
	Build(ctx context.Context, proj *packproject.Project, opts BuildOptions, progress ProgressFunc) (outputPath string, err error)
}

// BuildResult is one target/arch combination's outcome from a Run. Arch is
// "" for a Skipped result (HostSupported/Preflight failed before any arch
// was even chosen) or when the project has no binary registered at all for
// the target's OS (the backend's own default-arch fallback produced its
// usual "no binary registered" error instead of a real per-arch attempt).
type BuildResult struct {
	Target     Target
	Arch       string
	OutputPath string
	Err        error
	Skipped    bool // true when HostSupported/Preflight failed, not Build itself
}
