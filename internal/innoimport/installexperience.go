package innoimport

import (
	"fmt"
	"path/filepath"
	"strings"
)

// parseTasksSection recognizes two well-known Inno [Tasks] Name
// conventions and maps them onto packproject.InstallExperience's own
// already-shipped toggles, rather than proposing a new packaging feature -
// InstallerBear already has real Desktop-shortcut/Run-at-startup
// checkboxes (see InstallExperience's own doc comment), so importing an
// old .iss's intent to offer these just means proposing the equivalent
// toggle, the same way Import Existing Config already carries over
// Publisher/UpgradeGUID/etc.
//
//   - "desktopicon" is Inno's own project-wizard-generated task name for
//     its "Additional icons: create a desktop icon" checkbox (confirmed
//     against this repo's own real Inno/KrankyBearInstallerBear.iss,
//     itself wizard-generated) -> DesktopShortcut.
//   - Any task Name containing "startup" or "autostart" (Inno has no
//     single fixed standard name for this one the way it does for
//     desktopicon, so this is a looser, best-effort match) -> AutostartAtLogin.
//
// Any other task Name is reported in skipped rather than guessed at -
// Inno's own Quick Launch icon task, or some project-specific custom task,
// has no InstallerBear equivalent.
func parseTasksSection(lines []string, expand func(string) string) (desktopShortcut, autostartAtLogin bool, skipped []string) {
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		name := strings.ToLower(expand(attrs["Name"]))
		switch {
		case name == "desktopicon":
			desktopShortcut = true
		case strings.Contains(name, "startup") || strings.Contains(name, "autostart"):
			autostartAtLogin = true
		default:
			skipped = append(skipped, "[Tasks] not recognized, not imported: "+line)
		}
	}
	return desktopShortcut, autostartAtLogin, skipped
}

// parseIconsSection recognizes the two [Icons] shortcut locations every
// InstallerBear-built Windows installer already creates unconditionally or
// via InstallExperience (Start Menu, always; Desktop, when
// InstallExperience.DesktopShortcut is set - already inferred from
// [Tasks] above, so a matching Icons line here is just that same intent's
// own confirmation, not an independent signal worth a second toggle).
// Anything else - a Quick Launch icon, a shortcut with custom arguments,
// a shortcut to something other than the app's own exe - has no
// InstallerBear equivalent and is reported in skipped.
func parseIconsSection(lines []string, expand func(string) string) (skipped []string) {
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		name := strings.ToLower(expand(attrs["Name"]))
		switch {
		case strings.HasPrefix(name, "{autoprograms}") || strings.HasPrefix(name, "{group}"):
			// Start Menu shortcut - already created unconditionally.
		case strings.HasPrefix(name, "{autodesktop}") || strings.HasPrefix(name, "{userdesktop}") || strings.HasPrefix(name, "{commondesktop}"):
			// Desktop shortcut - already inferred from the [Tasks] entry
			// it's normally gated behind.
		default:
			skipped = append(skipped, "[Icons] not recognized, not imported: "+line)
		}
	}
	return skipped
}

// parseRunSection recognizes Inno's own "launch the app after install"
// convention - a [Run] entry pointing at the app's own exe with the
// postinstall flag (Inno's project wizard generates exactly this) - and
// maps it onto InstallExperience.LaunchAfterInstall, InstallerBear's own
// already-shipped equivalent. Any other [Run] entry (installing a
// redistributable, running some other setup step) has no InstallerBear
// equivalent - there's no general "run this during install" mechanism on
// Windows the way Hooks.PreInstall provides on macpkg/debrpm (NSIS/MSI
// have no shell interpreter to run one in) - and is reported in skipped.
func parseRunSection(lines []string, exeName string, expand func(string) string) (launchAfterInstall bool, skipped []string) {
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		filename := expand(attrs["Filename"])
		flags := strings.Fields(attrs["Flags"])
		// toSlashPath first: filepath.Base only splits on this build's own
		// OS separator, so a Windows-style "\"-separated Inno path (every
		// real .iss) would come back completely unsplit on a non-Windows
		// dev machine - the same gotcha addFileLine already avoids the
		// same way.
		if exeName != "" && containsStr(flags, "postinstall") && strings.EqualFold(filepath.Base(toSlashPath(filename)), exeName) {
			launchAfterInstall = true
			continue
		}
		skipped = append(skipped, "[Run] not recognized, not imported: "+line)
	}
	return launchAfterInstall, skipped
}

// parseUninstallRunSection recognizes the common "taskkill the running
// exe before uninstalling" pattern - every InstallerBear-built Setup.exe
// already does exactly this unconditionally (see nsi_template.go's own
// Uninstall section), so a matching line here needs nothing imported, it's
// already covered. Any other [UninstallRun] command is reported in
// skipped: there's no general "run this on uninstall" mechanism on
// Windows (same gap as [Run] above, and for the same reason).
func parseUninstallRunSection(lines []string, exeName string, expand func(string) string) (skipped []string) {
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		filename := strings.ToLower(expand(attrs["Filename"]))
		params := strings.ToLower(expand(attrs["Parameters"]))
		if filename == "{cmd}" && exeName != "" && strings.Contains(params, "taskkill") && strings.Contains(params, strings.ToLower(exeName)) {
			continue // already handled unconditionally by winexe's own uninstaller
		}
		skipped = append(skipped, "[UninstallRun] not recognized, not imported: "+line)
	}
	return skipped
}

// parseUninstallDeleteSection reports on [UninstallDelete] entries rather
// than importing anything - there's no packproject field to import into
// either way, but the accurate story differs sharply by Windows backend,
// worth surfacing rather than lumping in with a generic "not supported"
// note:
//
//   - A path resolving inside {app} (the install directory) is already
//     cleaned up for free by Setup.exe's own uninstaller, which removes
//     the entire install directory recursively (RMDir /r $INSTDIR) -
//     unlike Inno's own conservative uninstaller (which only removes
//     files it explicitly tracked from [Files], which is why a real .iss
//     needs an explicit [UninstallDelete] entry for anything else, e.g. a
//     file moved into place at runtime). counted, not treated as skipped.
//   - The same path is NOT cleaned up by .msi's uninstaller though -
//     Windows Installer only ever removes what it tracked in its own
//     Component/File tables, the same conservative model Inno has, and
//     the one real WiX mechanism for this (RemoveFile/RemoveFolder)
//     crashes wixl outright when compiled (confirmed empirically:
//     "unhandled child Component node RemoveFile") - a genuine, currently
//     unfixable tool limitation, the same class of gap as wixl's missing
//     ARM64 support.
//   - A path resolving outside {app} entirely has no InstallerBear
//     equivalent on either backend and is reported in skipped.
func parseUninstallDeleteSection(lines []string, expand func(string) string) (insideAppCount int, skipped []string) {
	for _, line := range lines {
		attrs := parseInnoAttrs(line)
		name := expand(attrs["Name"])
		if strings.HasPrefix(strings.ToLower(name), "{app}") {
			insideAppCount++
			continue
		}
		skipped = append(skipped, "[UninstallDelete] outside the install directory, not supported: "+line)
	}
	return insideAppCount, skipped
}

// uninstallDeleteNote turns a positive insideAppCount (see
// parseUninstallDeleteSection above) into the Skipped-list note explaining
// the Setup.exe-covers-it/.msi-doesn't split - kept as its own function so
// iss.go's own assembly step reads as a plain list of "gather notes" calls.
func uninstallDeleteNote(insideAppCount int) string {
	return fmt.Sprintf(
		"%d path(s) in [UninstallDelete] resolve inside the install directory - already cleaned up automatically by Setup.exe's uninstaller (which removes the whole install directory on uninstall), but NOT by .msi's uninstaller (Windows Installer only removes what it tracked, and wixl can't express WiX's RemoveFile/RemoveFolder - confirmed to crash outright when compiled)",
		insideAppCount)
}
