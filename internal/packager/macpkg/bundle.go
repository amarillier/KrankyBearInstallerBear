package macpkg

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// BuildAppBundle assembles a standard macOS .app bundle under destDir: the
// binary at Contents/MacOS/<BundleExecutable>, a lowercase CLI-name symlink
// alongside it (mirroring this project's own historical fpm-based
// convention), every "darwin"-targeted Payload entry copied under
// Contents/MacOS/, an optional Resources/<Name>.icns, and a real Info.plist.
// It never shells out — pure Go, so it's unit-testable on any OS even though
// only macpkg's Build (which calls pkgbuild afterwards) is darwin-only.
// Returns the bundle's own path, <destDir>/<Name>.app.
func BuildAppBundle(proj *packproject.Project, arch, destDir string) (string, error) {
	bin, ok := proj.BinaryFor("darwin", arch)
	if !ok {
		return "", fmt.Errorf("macpkg: no darwin/%s binary registered in project.binaries", arch)
	}

	appPath := filepath.Join(destDir, proj.Identity.Name+".app")
	macosDir := filepath.Join(appPath, "Contents", "MacOS")
	resourcesDir := filepath.Join(appPath, "Contents", "Resources")

	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		return "", fmt.Errorf("macpkg: creating bundle: %w", err)
	}

	execName := proj.MacOS.BundleExecutable
	if err := copyFile(proj.ResolvePath(bin.Path), filepath.Join(macosDir, execName), 0o755); err != nil {
		return "", fmt.Errorf("macpkg: copying binary: %w", err)
	}

	// Skip the symlink when it's case-insensitively identical to execName:
	// both macOS's default APFS volume format and Windows' NTFS are
	// case-insensitive, so e.g. "installerbear" and "KrankyBearInstallerBear"
	// name the same file — os.Symlink would fail with "file exists", not
	// silently create a second entry.
	if cliName := packager.Slug(proj.Identity.Name); !strings.EqualFold(cliName, execName) {
		if err := os.Symlink(execName, filepath.Join(macosDir, cliName)); err != nil {
			return "", fmt.Errorf("macpkg: creating CLI symlink: %w", err)
		}
	}

	if proj.Identity.LicenseFile != "" {
		if err := copyFile(proj.ResolvePath(proj.Identity.LicenseFile), filepath.Join(macosDir, "License.txt"), 0o644); err != nil {
			return "", fmt.Errorf("macpkg: copying license: %w", err)
		}
	}

	for _, entry := range proj.Payload {
		if !entry.AppliesToOS("darwin", arch) {
			continue
		}
		src := proj.ResolvePath(entry.Source)
		var err error
		if entry.Recursive {
			err = copyTree(src, filepath.Join(macosDir, entry.Dest), entry.ExcludesMatch)
		} else {
			// Dest is always a destination directory, same as for a
			// Recursive entry — the installed filename comes from Source's
			// own basename, matching winmsi/winexe's interpretation (see
			// packproject.PayloadEntry's own doc comment).
			dst := filepath.Join(macosDir, entry.Dest, filepath.Base(entry.Source))
			err = copyFile(src, dst, 0o644)
		}
		if err != nil {
			return "", fmt.Errorf("macpkg: copying payload %q: %w", entry.Source, err)
		}
	}

	if proj.Identity.Icons.ICNS != "" {
		if err := os.MkdirAll(resourcesDir, 0o755); err != nil {
			return "", fmt.Errorf("macpkg: creating Resources: %w", err)
		}
		iconDst := filepath.Join(resourcesDir, proj.Identity.Name+".icns")
		if err := copyFile(proj.ResolvePath(proj.Identity.Icons.ICNS), iconDst, 0o644); err != nil {
			return "", fmt.Errorf("macpkg: copying icon: %w", err)
		}
	}

	// Only copy a real Info.plist into the bundle when the project's own
	// directory already has one - i.e. the user deliberately renamed their
	// Info-plist.txt placeholder to activate it. Never synthesize one from
	// scratch: this project's own historical package.sh/fpm convention
	// ships every app with no real Info.plist at all (just the placeholder
	// text, verbatim) specifically to avoid it, and some IT security
	// scanning apparently flags a real one - and pkgbuild's --root mode
	// (see build.go) doesn't require one the way --component mode does, so
	// there's no technical reason to force it either.
	//
	// Exception: when FileAssociations are configured, Info-plist.txt is
	// promoted to a real functional Info.plist too, even without being
	// renamed - a real CFBundleDocumentTypes entry is an OS-level
	// requirement for file-type association, so a project that
	// deliberately opts into FileAssociations has already made that
	// tradeoff explicitly; a project that hasn't configured any keeps
	// Info-plist.txt exactly as inert as before.
	plistSrc := filepath.Join(proj.BaseDir, "Info.plist")
	if !fileExists(plistSrc) && len(proj.FileAssociations) > 0 {
		if candidate := filepath.Join(proj.BaseDir, "Info-plist.txt"); fileExists(candidate) {
			plistSrc = candidate
		}
	}
	if fileExists(plistSrc) {
		content, err := os.ReadFile(plistSrc)
		if err != nil {
			return "", fmt.Errorf("macpkg: reading %s: %w", filepath.Base(plistSrc), err)
		}
		icnsFileName := ""
		if proj.Identity.Icons.ICNS != "" {
			icnsFileName = proj.Identity.Name + ".icns"
		}
		if merged, injected := injectFileAssociationDocTypes(content, proj, icnsFileName); injected {
			content = merged
		}
		if err := os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), content, 0o644); err != nil {
			return "", fmt.Errorf("macpkg: writing Info.plist: %w", err)
		}
	}

	// Info-plist.txt/Readme-plist.txt are placeholder/documentation files
	// (not consulted for any build logic - the real Info.plist above is
	// the only one that matters functionally) that this project's own
	// historical package.sh copies straight into Contents/ verbatim, as a
	// sibling of MacOS/, for anyone poking around the installed bundle.
	// Auto-copy them here too, if present, so every project migrated onto
	// macpkg keeps that same layout with no per-project Payload config
	// needed - Payload entries can't reach Contents/ directly anyway (they
	// always land under Contents/MacOS/, see the loop above).
	for _, name := range []string{"Info-plist.txt", "Readme-plist.txt"} {
		src := filepath.Join(proj.BaseDir, name)
		if !fileExists(src) {
			continue
		}
		if err := copyFile(src, filepath.Join(appPath, "Contents", name), 0o644); err != nil {
			return "", fmt.Errorf("macpkg: copying %s: %w", name, err)
		}
	}

	return appPath, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// copyTree copies srcDir's contents into dstDir, skipping any file whose
// path relative to srcDir excludeMatch reports true for — see
// packproject.PayloadEntry.ExcludesMatch, the caller's usual excludeMatch.
func copyTree(srcDir, dstDir string, excludeMatch func(relPath string) bool) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel != "." && excludeMatch(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		dst := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, dst, info.Mode())
	})
}
