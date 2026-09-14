// Package debrpm builds .deb and .rpm packages via nfpm — a pure-Go library,
// so unlike fpm this needs no Ruby install and (since nfpm writes RPMs itself
// via google/rpmpack rather than shelling out to rpmbuild) builds .rpm
// reliably from macOS too, not just from Linux.
package debrpm

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goreleaser/nfpm/v2"
	"github.com/goreleaser/nfpm/v2/files"

	_ "github.com/goreleaser/nfpm/v2/deb" // self-registers the "deb" packager
	_ "github.com/goreleaser/nfpm/v2/rpm" // self-registers the "rpm" packager

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

const defaultArch = "amd64"

func init() {
	packager.Register(packager.TargetDEB, NewDeb)
	packager.Register(packager.TargetRPM, NewRPM)
}

// NewDeb returns the .deb backend.
func NewDeb() packager.Packager { return format{name: packager.TargetDEB} }

// NewRPM returns the .rpm backend.
func NewRPM() packager.Packager { return format{name: packager.TargetRPM} }

// format implements packager.Packager for one nfpm-supported format. Both
// deb and rpm share every step except the format string handed to
// nfpm.Get, since nfpm normalizes arch/filename/script conventions per
// format internally.
type format struct{ name packager.Target }

func (f format) Target() packager.Target { return f.name }

// HostSupported is always nil: nfpm is a pure-Go library with no OS-specific
// external tool, so both formats build from any host OS.
func (f format) HostSupported() error { return nil }

// Preflight is always nil for the same reason — nothing to check for.
func (f format) Preflight() error { return nil }

func (f format) Build(ctx context.Context, proj *packproject.Project, opts packager.BuildOptions, progress packager.ProgressFunc) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	arch := opts.Arch
	if arch == "" {
		arch = defaultArch
	}

	bin, ok := proj.BinaryFor("linux", arch)
	if !ok {
		return "", fmt.Errorf("%s: no linux/%s binary registered in project.binaries", f.name, arch)
	}

	if progress != nil {
		if proj.InstallExperience.LaunchAfterInstall {
			progress(f.name, "note: install_experience.launch_after_install has no effect for "+string(f.name)+" - no installer-time \"launch it now\" convention exists on Linux, and auto-launching a GUI app from a postinstall script could break a headless/CI install")
		}
		if proj.InstallExperience.DesktopShortcut {
			progress(f.name, "note: install_experience.desktop_shortcut has no effect for "+string(f.name)+" - a real app-menu .desktop entry is always installed regardless of this setting (see below), since that's how a Linux app appears in the launcher at all, not an optional extra the way a literal desktop icon is on Windows")
		}
		if proj.InstallExperience.AutostartAtLogin {
			progress(f.name, "note: install_experience.autostart_at_login has no effect for "+string(f.name)+" yet - Windows-only for now (a real equivalent exists via an XDG autostart .desktop entry, tracked separately if this comes up)")
		}
	}

	info, cleanup, err := buildInfo(proj, arch, bin)
	if err != nil {
		return "", err
	}
	defer cleanup()

	pkgr, err := nfpm.Get(string(f.name))
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.name, err)
	}

	outDir := proj.Output.Dir
	if opts.OutputDir != "" {
		outDir = opts.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("%s: creating output dir: %w", f.name, err)
	}

	outPath := filepath.Join(outDir, pkgr.ConventionalFileName(info))
	out, err := os.Create(outPath) //nolint:gosec // outPath is derived from validated config, not user input
	if err != nil {
		return "", fmt.Errorf("%s: creating output file: %w", f.name, err)
	}
	defer out.Close()

	if progress != nil {
		progress(f.name, "packaging "+outPath)
	}

	if err := pkgr.Package(info, out); err != nil {
		out.Close()
		os.Remove(outPath) // don't leave a stale, truncated package behind on failure
		return "", fmt.Errorf("%s: %w", f.name, err)
	}

	return outPath, nil
}

// buildInfo maps a Project into the nfpm.Info this format needs, and returns
// a cleanup func that removes any temp hook-script files it wrote — nfpm's
// Scripts fields are paths to script files, while our Hooks are inline text,
// so the inline text has to land on disk somewhere for nfpm to read.
func buildInfo(proj *packproject.Project, arch string, bin packproject.BinaryEntry) (*nfpm.Info, func(), error) {
	installDir := proj.Install.Linux
	binName := packager.Slug(proj.Identity.Name)

	// installDir itself (e.g. /opt/AppName) is never the explicit
	// destination of anything below - only ever the parent of the binary,
	// License.txt, and any Payload entries - so on its own it only ever
	// gets registered as an unowned "implicit" directory (a plain parent-
	// of-file), which rpm's builder skips entirely and never cleans up on
	// erase (same root cause as excludeFilteredTree's own directory-entry
	// fix above, one level up: confirmed via a real `rpm -e` leaving this
	// one directory - and only this one - behind, empty, after that fix).
	// An explicit TypeDir entry here fixes it the same way; nfpm safely
	// upgrades any already-implicit entry at the same destination rather
	// than treating this as a collision (see files.PrepareForPackager's
	// own TypeDir case).
	contents := files.Contents{
		{Destination: installDir, Type: files.TypeDir},
		{Source: proj.ResolvePath(bin.Path), Destination: filepath.Join(installDir, binName), Type: files.TypeFile},
	}

	if proj.Identity.LicenseFile != "" {
		contents = append(contents, &files.Content{
			Source:      proj.ResolvePath(proj.Identity.LicenseFile),
			Destination: filepath.Join(installDir, "License.txt"),
			Type:        files.TypeFile,
		})
	}

	for _, entry := range proj.Payload {
		if len(entry.OS) > 0 && !slices.Contains(entry.OS, "linux") {
			continue
		}

		if entry.Recursive && len(entry.Excludes) > 0 {
			// nfpm's own files.Content has no exclude concept — TypeTree
			// just hands nfpm a directory to walk itself — so an entry with
			// Excludes must be pre-walked and filtered here instead, adding
			// one TypeFile entry per surviving file rather than a single
			// TypeTree entry for the whole directory.
			filtered, err := excludeFilteredTree(proj.ResolvePath(entry.Source), filepath.Join(installDir, entry.Dest), entry)
			if err != nil {
				return nil, nil, fmt.Errorf("payload %q: %w", entry.Source, err)
			}
			contents = append(contents, filtered...)
			continue
		}

		if entry.Recursive {
			contents = append(contents, &files.Content{
				Source:      proj.ResolvePath(entry.Source),
				Destination: filepath.Join(installDir, entry.Dest),
				Type:        files.TypeTree,
			})
			continue
		}

		// Dest is always a destination directory, same as for a Recursive
		// entry — the installed filename comes from Source's own basename,
		// matching winmsi/winexe's interpretation (see
		// packproject.PayloadEntry's own doc comment).
		contents = append(contents, &files.Content{
			Source:      proj.ResolvePath(entry.Source),
			Destination: filepath.Join(installDir, entry.Dest, filepath.Base(entry.Source)),
			Type:        files.TypeFile,
		})
	}

	var cleanups []func()
	cleanup := func() {
		for _, c := range cleanups {
			c()
		}
	}

	// Always installs a real app-menu .desktop entry (+ PNG icon, if set) -
	// see desktopEntry's own comment on why this isn't gated behind
	// InstallExperience.DesktopShortcut the way the Windows Desktop
	// shortcut toggle is.
	desktopContents, desktopCleanup, err := desktopIntegrationContents(proj, installDir, binName)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cleanups = append(cleanups, desktopCleanup)
	contents = append(contents, desktopContents...)

	preInstall, c, err := writeHookScript(proj.Hooks.PreInstall)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("pre_install hook: %w", err)
	}
	if c != nil {
		cleanups = append(cleanups, c)
	}

	// PostInstall isn't used by any Hooks field today, so this is always
	// exactly the desktop/icon/mime-cache refresh - best-effort (|| true),
	// so a minimal/headless box missing these tools doesn't fail the install.
	hasIcon := proj.Identity.Icons.PNG != ""
	hasFileAssociations := len(proj.FileAssociations) > 0
	postInstall, c, err := writeHookScript(desktopIntegrationRefreshScript(hasIcon, hasFileAssociations))
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("post_install (desktop integration refresh): %w", err)
	}
	if c != nil {
		cleanups = append(cleanups, c)
	}

	// PostRemove already carries the project's own Hooks.PostUninstall, so
	// the same refresh commands are appended after it rather than
	// replacing it - both run in the same script, user hook first.
	postRemoveScript := strings.TrimRight(proj.Hooks.PostUninstall, "\n")
	if postRemoveScript != "" {
		postRemoveScript += "\n"
	}
	postRemoveScript += desktopIntegrationRefreshScript(hasIcon, hasFileAssociations)
	postRemove, c, err := writeHookScript(postRemoveScript)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("post_uninstall hook: %w", err)
	}
	if c != nil {
		cleanups = append(cleanups, c)
	}

	info := nfpm.WithDefaults(&nfpm.Info{
		Name:        binName,
		Arch:        arch,
		Version:     proj.Identity.Version,
		Maintainer:  firstNonEmpty(proj.Identity.Publisher, proj.Identity.Vendor),
		Description: proj.Identity.Description,
		Vendor:      proj.Identity.Vendor,
		Homepage:    proj.Identity.URL,
		License:     proj.Identity.License,
		Overridables: nfpm.Overridables{
			Contents: contents,
			Scripts: nfpm.Scripts{
				PreInstall:  preInstall,
				PostInstall: postInstall,
				PostRemove:  postRemove,
			},
		},
	})

	return info, cleanup, nil
}

// excludeFilteredTree walks srcDir and returns one files.Content{Type:
// TypeFile} per file whose path relative to srcDir doesn't match
// entry.ExcludesMatch, plus one files.Content{Type: TypeDir} per surviving
// directory — see buildInfo's own comment on why this exists: nfpm's
// TypeTree has no exclude concept of its own.
//
// The directory entries matter, not just cosmetically: a plain TypeFile
// list gives rpm/deb no ownership record for the directories those files
// sit in, only an unowned "implicit" parent-of-file (nfpm's own TypeTree
// expansion emits explicit TypeDir entries for this exact reason, since a
// real .rpm's own %files section is what actually removes a directory on
// uninstall). This was a real bug, found via `rpm -e` on a real machine:
// every payload file got removed, but every directory it had lived in was
// left behind, empty, forever — since this func used to just skip
// directories entirely (`if d.IsDir() { return nil }`, no Content emitted
// at all). Confirmed the fix by rebuilding a real .rpm and checking
// `rpm -qlv` actually lists these paths with directory mode bits now.
func excludeFilteredTree(srcDir, destDir string, entry packproject.PayloadEntry) (files.Contents, error) {
	var out files.Contents
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		if rel != "." && entry.ExcludesMatch(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		dest := filepath.Join(destDir, rel)
		if d.IsDir() {
			out = append(out, &files.Content{
				Destination: dest,
				Type:        files.TypeDir,
			})
			return nil
		}
		out = append(out, &files.Content{
			Source:      path,
			Destination: dest,
			Type:        files.TypeFile,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// writeHookScript writes non-empty inline hook text to a temp shell script
// nfpm can point a Scripts field at, returning "" with no cleanup for an
// empty hook (nfpm treats an empty Scripts field as "no script").
func writeHookScript(content string) (path string, cleanup func(), err error) {
	if strings.TrimSpace(content) == "" {
		return "", nil, nil
	}

	f, err := os.CreateTemp("", "packman-hook-*.sh")
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	if !strings.HasPrefix(content, "#!") {
		content = "#!/bin/sh\nset -e\n" + content
	}
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Chmod(0o755); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}

	name := f.Name()
	return name, func() { os.Remove(name) }, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
