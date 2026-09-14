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

	if err := validateWindowsExeName(p); err != nil {
		errs = append(errs, err)
	}

	if p.Windows.InstallScope != "" && p.Windows.InstallScope != InstallScopeAllUsers && p.Windows.InstallScope != InstallScopeCurrentUser {
		errs = append(errs, fmt.Errorf("windows.install_scope %q is not valid (want %q or %q)",
			p.Windows.InstallScope, InstallScopeAllUsers, InstallScopeCurrentUser))
	}

	if err := validateFileAssociations(p.FileAssociations); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// validateFileAssociations requires a leading dot (the field's own doc
// comment says extensions are stored with one, so a value without it is
// almost certainly a typo, not a deliberate choice) and rejects duplicate
// extensions (the second one would just silently overwrite the first
// one's registry/desktop entries at build time otherwise).
func validateFileAssociations(assocs []FileAssociation) error {
	var errs []error
	seen := make(map[string]bool, len(assocs))
	for _, a := range assocs {
		if a.Extension == "" {
			errs = append(errs, errors.New("file_associations: extension is required"))
			continue
		}
		if !strings.HasPrefix(a.Extension, ".") {
			errs = append(errs, fmt.Errorf("file_associations: extension %q must start with a dot (e.g. %q)", a.Extension, "."+a.Extension))
			continue
		}
		if a.Extension == "." {
			errs = append(errs, errors.New("file_associations: extension \".\" has nothing after the dot"))
			continue
		}
		key := strings.ToLower(a.Extension)
		if seen[key] {
			errs = append(errs, fmt.Errorf("file_associations: extension %q is registered more than once", a.Extension))
			continue
		}
		seen[key] = true
	}
	return errors.Join(errs...)
}

// validateWindowsExeName catches a real, silent bug found via hands-on
// Windows testing: Windows.ExeName was set to the installer's own output
// filename (e.g. "AppSetup.exe") instead of the actual app binary's
// filename ("App.exe") - an easy mix-up given how similar the two names
// look. NSIS/MSI's File directive installs a binary under its own source
// basename (no rename), so ExeName must match that exactly: it drives the
// uninstaller's taskkill (silently fails to find the real running
// process otherwise), every Start Menu/Desktop shortcut target on both
// backends, and NSIS's MUI_FINISHPAGE_RUN launch-after-install action
// (which just silently does nothing if the path it builds doesn't exist -
// no error, no crash, the exact symptom that surfaced this). Checked
// against every registered "windows" binary, not just one arch.
func validateWindowsExeName(p *Project) error {
	if p.Windows.ExeName == "" {
		return nil
	}
	var errs []error
	for _, b := range p.Binaries {
		if b.OS != "windows" {
			continue
		}
		if base := filepath.Base(b.Path); base != p.Windows.ExeName {
			errs = append(errs, fmt.Errorf(
				"windows.exe_name %q does not match the windows/%s binary's own filename %q - "+
					"it must be the actual app binary's filename (NSIS/MSI install it under that name unchanged), "+
					"not the installer's own output filename; a mismatch silently breaks the uninstaller's taskkill, "+
					"every Start Menu/Desktop shortcut, and launch-after-install",
				p.Windows.ExeName, b.Arch, base))
		}
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
