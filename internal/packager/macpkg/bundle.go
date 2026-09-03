package macpkg

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
		if len(entry.OS) > 0 && !slices.Contains(entry.OS, "darwin") {
			continue
		}
		src := proj.ResolvePath(entry.Source)
		dst := filepath.Join(macosDir, entry.Dest)
		var err error
		if entry.Recursive {
			err = copyTree(src, dst)
		} else {
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

	if err := writeInfoPlist(proj, filepath.Join(appPath, "Contents", "Info.plist")); err != nil {
		return "", fmt.Errorf("macpkg: writing Info.plist: %w", err)
	}

	return appPath, nil
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

func copyTree(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
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
