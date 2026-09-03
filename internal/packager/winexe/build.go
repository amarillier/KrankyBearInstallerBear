// Package winexe builds a classic NSIS Setup.exe wizard installer. NSIS
// (makensis) is a small, brew/apt/choco-installable binary that can build a
// Windows installer from any host OS, so — unlike macpkg — this target has
// no HostSupported restriction; only Preflight (the tool itself missing) can
// block it.
package winexe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

const defaultArch = "amd64"

func init() {
	packager.Register(packager.TargetWinExe, New)
}

// New returns the Setup.exe backend.
func New() packager.Packager { return winexePackager{} }

type winexePackager struct{}

func (winexePackager) Target() packager.Target { return packager.TargetWinExe }

func (winexePackager) HostSupported() error { return nil }

func (winexePackager) Preflight() error {
	if _, err := exec.LookPath("makensis"); err != nil {
		return fmt.Errorf("makensis not found on PATH (install via brew/apt/choco: brew install makensis)")
	}
	return nil
}

func (p winexePackager) Build(ctx context.Context, proj *packproject.Project, opts packager.BuildOptions, progress packager.ProgressFunc) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	arch := opts.Arch
	if arch == "" {
		arch = defaultArch
	}

	bin, ok := proj.BinaryFor("windows", arch)
	if !ok {
		return "", fmt.Errorf("winexe: no windows/%s binary registered in project.binaries", arch)
	}

	outDir := proj.Output.Dir
	if opts.OutputDir != "" {
		outDir = opts.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("winexe: creating output dir: %w", err)
	}

	// arch in the filename matters once more than one Windows arch is in
	// play (see packager.Run's multi-arch loop) — without it, a second
	// arch's build would silently overwrite the first's output file.
	outFileName := fmt.Sprintf("%sSetup_%s_%s.exe", strings.ReplaceAll(proj.Identity.Name, " ", ""), proj.Identity.Version, arch)
	outPath, err := filepath.Abs(filepath.Join(outDir, outFileName))
	if err != nil {
		return "", fmt.Errorf("winexe: %w", err)
	}

	data := nsiData{
		AppName:      proj.Identity.Name,
		AppVersion:   proj.Identity.Version,
		Publisher:    proj.Identity.Publisher,
		ExeName:      proj.Windows.ExeName,
		OutFile:      outPath,
		InstallDir:   proj.Install.Windows,
		BinarySource: proj.ResolvePath(bin.Path),
	}
	if proj.Identity.Icons.ICO != "" {
		data.IconFile = proj.ResolvePath(proj.Identity.Icons.ICO)
	}
	if proj.Identity.LicenseFile != "" {
		data.LicenseSource = proj.ResolvePath(proj.Identity.LicenseFile)
	}
	for _, entry := range proj.Payload {
		if len(entry.OS) > 0 && !slices.Contains(entry.OS, "windows") {
			continue
		}
		destDir := strings.ReplaceAll(filepath.ToSlash(filepath.Clean(entry.Dest)), "/", `\`)
		if destDir == "." {
			destDir = ""
		}
		data.Files = append(data.Files, nsiFileEntry{
			DestDir:   destDir,
			Source:    proj.ResolvePath(entry.Source),
			Recursive: entry.Recursive,
		})
	}

	stagingDir, err := os.MkdirTemp("", "packman-winexe-*")
	if err != nil {
		return "", fmt.Errorf("winexe: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	nsiPath := filepath.Join(stagingDir, "app.nsi")
	nsiFile, err := os.Create(nsiPath)
	if err != nil {
		return "", fmt.Errorf("winexe: %w", err)
	}
	if err := nsiTemplate.Execute(nsiFile, data); err != nil {
		nsiFile.Close()
		return "", fmt.Errorf("winexe: rendering .nsi: %w", err)
	}
	nsiFile.Close()

	p.emit(progress, "running makensis...")
	if err := packager.RunCommand(ctx, "", func(line string) { p.emit(progress, line) }, "makensis", nsiPath); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("winexe: makensis: %w", err)
	}

	return outPath, nil
}

func (winexePackager) emit(progress packager.ProgressFunc, line string) {
	if progress != nil {
		progress(packager.TargetWinExe, line)
	}
}
