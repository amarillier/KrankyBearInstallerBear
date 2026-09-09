// Package winexe builds a classic NSIS Setup.exe wizard installer. NSIS
// (makensis) is a small, brew/apt/choco-installable binary that can build a
// Windows installer from any host OS, so — unlike macpkg — this target has
// no HostSupported restriction; only Preflight (the tool itself missing) can
// block it.
package winexe

import (
	"context"
	"fmt"
	"io/fs"
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

		if entry.Recursive && len(entry.Excludes) > 0 {
			// NSIS's own "File /r" has no exclude syntax at all, so an entry
			// with Excludes is pre-walked and filtered here instead, adding
			// one non-recursive File entry per surviving file rather than a
			// single "File /r" for the whole directory.
			filtered, err := excludeFilteredFiles(proj.ResolvePath(entry.Source), destDir, entry)
			if err != nil {
				return "", fmt.Errorf("winexe: payload %q: %w", entry.Source, err)
			}
			data.Files = append(data.Files, filtered...)
			continue
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

// excludeFilteredFiles walks srcDir and returns one non-recursive
// nsiFileEntry per file whose path relative to srcDir doesn't match
// entry.ExcludesMatch, with DestDir set to baseDestDir plus that file's own
// (backslash-separated) subdirectory — see the Build loop's own comment on
// why: NSIS's "File /r" has no exclude syntax to filter with directly.
func excludeFilteredFiles(srcDir, baseDestDir string, entry packproject.PayloadEntry) ([]nsiFileEntry, error) {
	var out []nsiFileEntry
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if entry.ExcludesMatch(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		destDir := baseDestDir
		if sub := filepath.Dir(rel); sub != "." {
			sub = strings.ReplaceAll(filepath.ToSlash(sub), "/", `\`)
			if destDir != "" {
				destDir += `\` + sub
			} else {
				destDir = sub
			}
		}
		out = append(out, nsiFileEntry{DestDir: destDir, Source: path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (winexePackager) emit(progress packager.ProgressFunc, line string) {
	if progress != nil {
		progress(packager.TargetWinExe, line)
	}
}
