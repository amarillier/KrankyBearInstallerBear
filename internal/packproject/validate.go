package packproject

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var guidRE = regexp.MustCompile(`^\{?[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}?$`)

// Validate checks the project for the errors that would otherwise surface as
// confusing failures deep inside a packager backend. baseDir resolves
// relative Source paths in Payload/Binaries/Identity (normally the directory
// the project YAML file lives in); pass "" to skip existence/symlink checks
// (e.g. when validating a config that isn't tied to a filesystem yet).
func (p *Project) Validate(baseDir string) error {
	var errs []error

	if p.Identity.Name == "" {
		errs = append(errs, errors.New("identity.name is required"))
	}
	if p.Identity.Version == "" {
		errs = append(errs, errors.New("identity.version is required"))
	}
	if p.Identity.ID == "" {
		errs = append(errs, errors.New("identity.id is required"))
	}

	for _, t := range p.Targets {
		if !isKnownTarget(t) {
			errs = append(errs, fmt.Errorf("targets: unknown target %q (want one of %s)", t, strings.Join(AllTargets, ", ")))
		}
	}

	if needsWindowsGUID(p.Targets) {
		if p.Windows.UpgradeGUID == "" {
			errs = append(errs, errors.New("windows.upgrade_guid is required when building winexe or winmsi"))
		} else if !guidRE.MatchString(p.Windows.UpgradeGUID) {
			errs = append(errs, fmt.Errorf("windows.upgrade_guid %q is not a valid GUID", p.Windows.UpgradeGUID))
		}
	}

	if err := validatePayload(p.Payload, baseDir); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func isKnownTarget(t string) bool {
	for _, known := range AllTargets {
		if t == known {
			return true
		}
	}
	return false
}

func needsWindowsGUID(targets []string) bool {
	for _, t := range targets {
		if t == TargetWinExe || t == TargetWinMSI {
			return true
		}
	}
	return false
}

// validatePayload ports the checks package.sh's validate_fpm_files ran by
// hand: duplicate destinations, dest path-traversal, missing sources,
// broken/circular symlinks, and a Recursive flag that doesn't match
// whether Source is actually a directory — all of which fpm/Inno (or, for
// the Recursive mismatch, a packager backend like winexe's NSIS "File /r")
// only ever caught by failing mid-build with a confusing error, or in
// macpkg/winmsi/debrpm's case by not failing at all and just silently
// producing a wrong layout (a "LICENSE" file nested inside a spurious
// "LICENSE" directory, say).
func validatePayload(entries []PayloadEntry, baseDir string) error {
	var errs []error
	seenDest := make(map[string]string, len(entries))

	for _, e := range entries {
		dest := filepath.ToSlash(filepath.Clean(e.Dest))
		if dest == ".." || strings.HasPrefix(dest, "../") || strings.HasPrefix(dest, "/") {
			errs = append(errs, fmt.Errorf("payload dest %q escapes the install root", e.Dest))
		}

		// Dest is always a destination *directory* (see PayloadEntry's own
		// doc comment) — so two non-recursive entries only actually
		// collide if they'd install the same final file (same Dest *and*
		// the same Source basename, e.g. two different source dirs each
		// contributing their own "opengl32.dll" into the same Dest); two
		// files with different basenames sharing one Dest is the normal,
		// intended case (several files landing in one folder), not a
		// collision. A Recursive entry's whole tree is the installed
		// extent, so those still collide on Dest alone.
		collisionKey := dest
		if !e.Recursive && e.Source != "" {
			collisionKey = dest + "/" + filepath.Base(e.Source)
		}
		if firstSrc, dup := seenDest[collisionKey]; dup {
			errs = append(errs, fmt.Errorf("payload dest %q used by both %q and %q", e.Dest, firstSrc, e.Source))
		} else {
			seenDest[collisionKey] = e.Source
		}

		if baseDir == "" || e.Source == "" {
			continue
		}
		src := e.Source
		if !filepath.IsAbs(src) {
			src = filepath.Join(baseDir, src)
		}
		info, lerr := os.Lstat(src)
		if lerr != nil {
			errs = append(errs, fmt.Errorf("payload source %q does not exist", e.Source))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, rerr := os.Readlink(src)
			if rerr != nil || target == "" || target == filepath.Base(src) {
				errs = append(errs, fmt.Errorf("payload source %q is a circular or broken symlink", e.Source))
				continue
			}
			if _, terr := os.Stat(src); terr != nil {
				errs = append(errs, fmt.Errorf("payload source %q symlink target does not exist", e.Source))
				continue
			}
		}

		// Stat (not Lstat) so a symlink is judged by what it actually
		// resolves to, matching what a backend's own copy step will do.
		if finalInfo, serr := os.Stat(src); serr == nil {
			switch {
			case e.Recursive && !finalInfo.IsDir():
				errs = append(errs, fmt.Errorf("payload source %q is marked recursive but is a file, not a directory", e.Source))
			case !e.Recursive && finalInfo.IsDir():
				errs = append(errs, fmt.Errorf("payload source %q is a directory but not marked recursive", e.Source))
			}
		}
	}

	return errors.Join(errs...)
}
