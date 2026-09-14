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
	if _, err := BuildAppBundle(proj, arch, stagingDir); err != nil {
		return "", err
	}

	if proj.Hooks.PostUninstall != "" {
		p.emit(progress, "note: post_uninstall hook has no effect for macpkg - a .pkg install has no OS-level uninstall action to hook into")
	}
	if proj.InstallExperience.LaunchAfterInstall {
		p.emit(progress, "note: install_experience.launch_after_install has no effect for macpkg - no installer-time \"launch it now\" convention exists on macOS, and auto-launching a GUI app from a postinstall script could break a headless install")
	}
	if proj.InstallExperience.DesktopShortcut {
		p.emit(progress, "note: install_experience.desktop_shortcut has no effect for macpkg - macOS has no \"desktop icon\" concept distinct from /Applications")
	}
	if proj.InstallExperience.AutostartAtLogin {
		p.emit(progress, "note: install_experience.autostart_at_login has no effect for macpkg yet - Windows-only for now (a real equivalent exists on macOS via a LaunchAgent plist, tracked separately if this comes up)")
	}
	if len(proj.FileAssociations) > 0 {
		if hasPlistSource(proj) {
			p.emit(progress, "note: file_associations added to the macOS Info.plist (CFBundleDocumentTypes) - found a real Info.plist or Info-plist.txt in the project directory to add them to")
		} else {
			p.emit(progress, "note: file_associations has no effect for macpkg yet - real file-type association needs CFBundleDocumentTypes in a genuine Info.plist, and this project directory has neither a real Info.plist nor Info-plist.txt to add it to; add one (see this file's own doc comment) to enable it")
		}
	}

	outDir := proj.Output.Dir
	if opts.OutputDir != "" {
		outDir = opts.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("macpkg: creating output dir: %w", err)
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s.pkg", packager.Slug(proj.Identity.Name), proj.Identity.Version, arch))

	// --root (not --component): --component requires a valid bundle with a
	// real Info.plist and hard-refuses otherwise ("is not a valid bundle
	// component") - confirmed empirically. --root just packages whatever's
	// in stagingDir as-is, no such validation, matching what this
	// project's previous fpm/osxpkg-based packaging has always done and
	// letting Info.plist stay genuinely optional (see bundle.go). appPath
	// is stagingDir's only entry, so this packages the exact same content
	// --component would have, just without requiring Info.plist to do it.
	args := []string{
		"--root", stagingDir,
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
