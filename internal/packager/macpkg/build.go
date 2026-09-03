// Package macpkg builds a macOS .pkg installer: BuildAppBundle (pure Go)
// assembles a standard .app bundle, then Build shells out to pkgbuild —
// Apple's own tool, always present with the Xcode Command Line Tools, and
// the only real option since no Go library builds .pkg. Both only ever run
// on darwin: HostSupported rejects every other host before Preflight or
// Build is even attempted.
package macpkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

func init() {
	packager.Register(packager.TargetMacPkg, New)
}

// New returns the .pkg backend.
func New() packager.Packager { return macpkgPackager{} }

type macpkgPackager struct{}

func (macpkgPackager) Target() packager.Target { return packager.TargetMacPkg }

func (macpkgPackager) HostSupported() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macpkg only builds on macOS (pkgbuild has no cross-platform equivalent); this host is %s", runtime.GOOS)
	}
	return nil
}

func (macpkgPackager) Preflight() error {
	if _, err := exec.LookPath("pkgbuild"); err != nil {
		return fmt.Errorf("pkgbuild not found on PATH (ships with the Xcode Command Line Tools: xcode-select --install)")
	}
	return nil
}

func (p macpkgPackager) Build(ctx context.Context, proj *packproject.Project, opts packager.BuildOptions, progress packager.ProgressFunc) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	arch := opts.Arch
	if arch == "" {
		arch = runtime.GOARCH // building only ever happens on darwin itself; the host's own arch is the sane default
	}

	stagingDir, err := os.MkdirTemp("", "packman-macpkg-*")
	if err != nil {
		return "", fmt.Errorf("macpkg: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	p.emit(progress, "assembling .app bundle...")
	appPath, err := BuildAppBundle(proj, arch, stagingDir)
	if err != nil {
		return "", err
	}

	if proj.Hooks.PostUninstall != "" {
		p.emit(progress, "note: post_uninstall hook has no effect for macpkg - a .pkg install has no OS-level uninstall action to hook into")
	}

	outDir := proj.Output.Dir
	if opts.OutputDir != "" {
		outDir = opts.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("macpkg: creating output dir: %w", err)
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s.pkg", packager.Slug(proj.Identity.Name), proj.Identity.Version, arch))

	args := []string{
		"--component", appPath,
		"--install-location", filepath.Dir(proj.Install.MacOS),
		"--identifier", proj.Identity.ID,
		"--version", proj.Identity.Version,
	}

	if proj.Hooks.PreInstall != "" {
		scriptsDir, cerr := writePreinstallScript(proj.Hooks.PreInstall)
		if cerr != nil {
			return "", fmt.Errorf("macpkg: pre_install hook: %w", cerr)
		}
		defer os.RemoveAll(scriptsDir)
		args = append(args, "--scripts", scriptsDir)
	}

	args = append(args, outPath)

	p.emit(progress, "running pkgbuild...")
	if err := packager.RunCommand(ctx, "", func(line string) { p.emit(progress, line) }, "pkgbuild", args...); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("macpkg: pkgbuild: %w", err)
	}

	return outPath, nil
}

func (macpkgPackager) emit(progress packager.ProgressFunc, line string) {
	if progress != nil {
		progress(packager.TargetMacPkg, line)
	}
}

// writePreinstallScript materializes the inline Hooks.PreInstall text as the
// "preinstall" file pkgbuild's --scripts directory convention expects — the
// same file-on-disk requirement nfpm's Scripts fields have (see debrpm).
func writePreinstallScript(content string) (string, error) {
	dir, err := os.MkdirTemp("", "packman-macpkg-scripts-*")
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(content, "#!") {
		content = "#!/bin/sh\nset -e\n" + content
	}
	if err := os.WriteFile(filepath.Join(dir, "preinstall"), []byte(content), 0o755); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
