// Package packproject defines the per-project packaging config (installerbear.yaml)
// shared by every backend under internal/packager and by both the GUI and CLI.
package packproject

import "gopkg.in/yaml.v3"

// Project describes everything needed to build installers/packages for one
// application across Windows, macOS, and Linux.
type Project struct {
	Identity          Identity          `yaml:"identity"`
	Binaries          []BinaryEntry     `yaml:"binaries"`
	Payload           []PayloadEntry    `yaml:"payload,omitempty"`
	Install           InstallLocations  `yaml:"install,omitempty"`
	Hooks             Hooks             `yaml:"hooks,omitempty"`
	InstallExperience InstallExperience `yaml:"install_experience,omitempty"`
	Windows           WindowsOptions    `yaml:"windows,omitempty"`
	MacOS             MacOSOptions      `yaml:"macos,omitempty"`
	Linux             LinuxOptions      `yaml:"linux,omitempty"`
	Targets           []string          `yaml:"targets"`
	Output            OutputOptions     `yaml:"output,omitempty"`
	// FileAssociations registers a file extension with this app - "double-
	// click a .foo file, it opens in this app" - see FileAssociation's own
	// doc comment for exactly what each backend does with one, including
	// macOS's own conditional support (needs a real Info.plist to exist,
	// or Info-plist.txt to promote to one).
	FileAssociations []FileAssociation `yaml:"file_associations,omitempty"`

	// BaseDir is the directory the project file lives in. Every backend
	// resolves relative Source/LicenseFile/icon paths against it. Set by
	// Load(); leave empty (paths resolve against the process cwd) when
	// constructing a Project by hand, e.g. in tests.
	BaseDir string `yaml:"-"`

	// sourceNode is the raw YAML document tree captured by
	// parseAndDefault, if this Project came from a real file - see
	// comments.go's own doc comment for what it's for (preserving
	// hand-typed comments across a load/save round-trip). Left nil for a
	// Project built by hand (a brand-new "New Project", the in-code
	// generated sample, every test in this codebase), which is exactly
	// right: there's no original comment layout to preserve for something
	// that was never loaded from a file.
	sourceNode *yaml.Node `yaml:"-"`
}

// Identity holds the app metadata common to every packaging backend.
type Identity struct {
	Name        string `yaml:"name"`
	ID          string `yaml:"id"`
	Version     string `yaml:"version"`
	Publisher   string `yaml:"publisher,omitempty"`
	Vendor      string `yaml:"vendor,omitempty"`
	URL         string `yaml:"url,omitempty"`
	Description string `yaml:"description,omitempty"`
	// License is a short license identifier (e.g. "MIT", "GPL v3") for
	// backends that record one as package metadata (currently debrpm's
	// nfpm.Info.License) — distinct from LicenseFile, which is the actual
	// license text shipped with the install.
	License     string  `yaml:"license,omitempty"`
	LicenseFile string  `yaml:"license_file,omitempty"`
	Icons       IconSet `yaml:"icons,omitempty"`
}

// IconSet holds per-format icon paths; each is optional and only required by
// the backends that need it (winexe/winmsi want ICO, macpkg wants ICNS).
type IconSet struct {
	ICO  string `yaml:"ico,omitempty"`
	ICNS string `yaml:"icns,omitempty"`
	PNG  string `yaml:"png,omitempty"`
}

// BinaryEntry is one compiled binary for a given OS/arch pair. OS/Arch use
// Go's own GOOS/GOARCH spelling ("windows", "darwin", "linux"; "amd64", "arm64").
type BinaryEntry struct {
	OS   string `yaml:"os"`
	Arch string `yaml:"arch"`
	Path string `yaml:"path"`
}

// PayloadEntry maps one extra file or directory into the installed layout,
// relative to the per-OS install root in InstallLocations. This is the direct
// analogue of today's Inno [Files] entries / fpm src=dest arguments.
type PayloadEntry struct {
	// Source may be a literal path, or a filepath.Match glob pattern (any
	// of "*", "?", "[" present) - the same pattern dialect Excludes
	// already uses, deliberately, rather than introducing a second one a
	// project author would need to learn. A pattern is resolved against
	// BaseDir once per build (see ExpandedPayload) into one concrete
	// PayloadEntry per match, each inheriting this entry's own Dest/
	// Recursive/OS/Excludes - every backend under internal/packager only
	// ever sees the expanded, literal result, never the pattern itself.
	// Matching zero files is not an error - a pattern is inherently
	// "whatever's there right now," not a promise something will be.
	Source string `yaml:"source"`
	// Dest is always a destination *directory*, relative to the install
	// root ("" for the root itself) — for both a Recursive entry (Source's
	// own contents land under it) and a single-file entry (the installed
	// filename is always Source's own basename, never taken from Dest).
	// Every backend under internal/packager must honor this the same way.
	Dest      string `yaml:"dest"`
	Recursive bool   `yaml:"recursive,omitempty"`
	// OS restricts this entry to specific OSes/arches - see AppliesToOS
	// (osmatch.go) for exactly how each value is matched. Empty means
	// every OS/arch, unchanged from before arch-scoping existed. A bare
	// OS name ("windows") matches every arch of that OS; an "os/arch"
	// pair ("windows/arm64") matches only that exact arch, the same
	// slash convention Docker's --platform/`go tool dist list` already
	// use. "mac"/"macos" (case-insensitive) are accepted as aliases for
	// "darwin" on either side of the slash.
	OS   []string `yaml:"os,omitempty"`
	Mode string   `yaml:"mode,omitempty"`
	// Excludes lists filepath.Match glob patterns (matched against each
	// file's path relative to Source) skipped when copying a Recursive
	// entry — the analogue of Inno's [Files] "Excludes:" attribute. Ignored
	// for a literal (non-glob) non-recursive entry, since there's nothing
	// to filter out of a single file; for a *glob-pattern* Source though,
	// Excludes also filters candidates out of the match set itself, even
	// when Recursive is false - "include broadly via Source, exclude
	// specifically via Excludes" for a flat set of matched files.
	Excludes []string `yaml:"excludes,omitempty"`
}

// FileAssociation registers one file extension with this app: double-
// clicking a matching file opens it with this app's own binary, passed
// the file's path as its first argument (Windows: `"<exe>" "%1"`; Linux:
// `Exec=... %f`). Deliberately just two fields, not a fully generic
// arbitrary-registry-entry escape hatch - covers the common "my app has
// its own document/project file type" case simply, matching this
// project's existing preference for structured fields over free-form
// config wherever the common case is well-defined (InstallExperience's
// own doc comment makes the same argument for install-time toggles).
//
// Windows (both winexe/winmsi): writes to HKCR - Windows' own registry
// virtualization automatically redirects an unprivileged process's HKCR
// writes to HKCU\Software\Classes, so this works the same way regardless
// of WindowsOptions.InstallScope, with no extra code needed for that
// split (a well-documented OS behavior, not verified against real
// hardware from this dev machine the way the rest of this project's wixl/
// NSIS findings have been empirically confirmed - flagged here
// deliberately, not silently assumed). winmsi implements this via WiX's
// native <ProgId>/<Extension>/<Verb> elements (confirmed to compile to
// real Registry-table rows via a real compiled test .msi - wixl crashes
// if <Extension> is nested directly under <File> instead, a real,
// non-obvious gotcha); winexe writes the same registry keys by hand,
// since NSIS has no built-in file-association directive.
//
// Linux (deb/rpm): adds a MimeType= entry to the .desktop file this
// project already generates unconditionally (see debrpm's own
// desktopfile.go) plus a shared-mime-info XML package
// (/usr/share/mime/packages/<name>.xml, refreshed via
// update-mime-database the same way update-desktop-database already
// refreshes the .desktop entry itself).
//
// macOS: conditionally supported. Real file-type association there needs
// CFBundleDocumentTypes in a genuine Info.plist, and this project
// deliberately never synthesizes a whole Info.plist from scratch (matching
// Allan's own established package.sh convention - see macpkg's own doc
// comment); but when the project directory has a real Info.plist, or its
// own Info-plist.txt placeholder (even without renaming it - configuring
// FileAssociations at all is already an explicit opt-in to needing a real
// plist), macpkg merges a CFBundleDocumentTypes array into a copy of it
// rather than leaving the feature unreachable there. A hand-written
// CFBundleDocumentTypes already present in that file is always left alone,
// never fought or duplicated. With neither file present, there's genuinely
// nowhere to write this without synthesizing an Info.plist outright, so
// macpkg logs a clear note instead of silently ignoring the setting,
// matching every other platform-specific InstallExperience-style setting.
type FileAssociation struct {
	// Extension includes the leading dot, e.g. ".myp" - matching how a
	// file extension actually reads, rather than making every author
	// remember whether to include it.
	Extension string `yaml:"extension"`
	// Description is the human-readable file type name shown in
	// Explorer/file managers (e.g. "My App Project File"). Optional -
	// falls back to a generic "<AppName> File" when blank.
	Description string `yaml:"description,omitempty"`
}

// InstallLocations gives the per-OS install root. Any left blank get a
// sensible default filled in by Defaults().
type InstallLocations struct {
	Windows string `yaml:"windows,omitempty"`
	MacOS   string `yaml:"macos,omitempty"`
	Linux   string `yaml:"linux,omitempty"`
}

// Hooks holds optional inline shell snippets run on the TARGET machine
// (baked into the built macOS `.pkg`/Linux `.deb`/`.rpm` package itself,
// executed later by its own install/uninstall action - contrast
// Output.PostBuildHook, which runs immediately on THIS build machine
// instead). Both default to empty (no hook) — most projects need neither.
// Windows has no equivalent: NSIS/MSI have no shell interpreter to run
// script text in, so both fields are simply unavailable there, not a
// setting that could apply but doesn't.
//
// Runs under /bin/sh by default (a "#!/bin/sh\nset -e\n" header is
// prepended automatically) - start the text with its own shebang line
// (e.g. "#!/usr/bin/env python3") to use a different interpreter instead,
// the same convention any real script file follows; InstallerBear only
// adds the default header when the text doesn't already start with "#!".
type Hooks struct {
	// PreInstall runs before files are installed, on both macOS (the
	// .pkg's own preinstall script) and Linux (.deb/.rpm's own preinst).
	PreInstall string `yaml:"pre_install,omitempty"`
	// PostUninstall runs after removal, Linux only - a .pkg install has
	// no OS-level uninstall action at all for anything to hook into, so
	// this has no effect on macOS (a clear progress note is logged during
	// a macOS build when it's set, rather than silently doing nothing).
	// On Linux, InstallerBear's own desktop-database/icon-cache/
	// shared-mime-info refresh commands (see debrpm's own
	// desktopfile.go) run right after this text, in the same script -
	// keep this one to plain POSIX shell so those still work regardless
	// of any custom shebang used here.
	PostUninstall string `yaml:"post_uninstall,omitempty"`
}

// InstallExperience covers "what happens once the files are down" -
// launch-after-install and an optional desktop shortcut today, more later
// (open-a-file, autostart) once these are proven. Deliberately two
// structured booleans rather than one generic post-install hook string:
// unlike Hooks.PreInstall/PostUninstall (POSIX shell text, meaningful on
// macpkg/debrpm since both run on Unix), NSIS/MSI have no shell
// interpreter at all, so each backend needs to wire these up its own
// native way rather than interpreting a portable script.
//
// LaunchAfterInstall is a genuine end-user choice: opting it in here adds
// a real, checked-by-default "Launch <App> now?" checkbox to the
// winexe/winmsi finish page (both have a native, no-extra-dependency
// mechanism for this - NSIS's MUI_FINISHPAGE_RUN, wixl's
// WIXUI_EXITDIALOGOPTIONALCHECKBOX+CustomAction, confirmed empirically
// against a real compiled .msi's CustomAction/ControlEvent tables). Not
// applicable to macpkg/debrpm - no installer-time "launch it now" UI
// convention exists there, and auto-launching a GUI app from a postinstall
// script could actively break a headless/CI package install.
//
// DesktopShortcut always means "offer a Desktop shortcut in addition to
// the Start Menu one both backends already create unconditionally" when
// set, but the two backends differ in *how* the person installing gets a
// say: winexe/NSIS gives them a real, checked-by-default Components-page
// checkbox (NSIS can do this cheaply - an optional Section + a Components
// page). winmsi/wixl can't offer the same interactive choice: wixl's
// bundled UI extension only ships WixUI_Minimal, not the fuller
// WixUI_FeatureTree/Mondo variants a real "create a desktop icon?"
// checkbox would need, so there this is a plain author-time yes/no baked
// into the installer instead - confirmed as an acceptable asymmetry
// rather than withholding the winexe checkbox for consistency. Windows-only
// for now (macOS has no real "desktop icon" convention distinct from
// /Applications; Linux .desktop-file generation is tracked separately, a
// bigger feature of its own).
//
// AutostartAtLogin follows the exact same split as DesktopShortcut, for
// the same underlying reason (wixl's UI limitation): a real,
// unchecked-by-default Components-page checkbox on winexe (writing/
// removing a per-user HKCU Run value - not HKLM, since autostarting for
// every account on the machine isn't implied by one user opting in), an
// author-time-only yes/no on winmsi. Unchecked by default (unlike
// DesktopShortcut, which defaults checked): autostart is a much bigger
// behavioral change for an end user to discover after the fact than an
// extra shortcut is, so asking explicitly the first time seems safer than
// defaulting it on. Windows-only, same reasoning as DesktopShortcut - less
// value on macOS/Linux, tracked separately if it ever comes up there.
type InstallExperience struct {
	LaunchAfterInstall bool `yaml:"launch_after_install,omitempty"`
	DesktopShortcut    bool `yaml:"desktop_shortcut,omitempty"`
	AutostartAtLogin   bool `yaml:"autostart_at_login,omitempty"`
}

// WindowsOptions covers Setup.exe (NSIS) and .msi (WiX) specifics.
type WindowsOptions struct {
	// UpgradeGUID is the stable identifier carried across every version of
	// this app — equivalent to today's Inno [Setup] AppId. It becomes WiX's
	// UpgradeCode; it must NOT be reused as the per-build MSI ProductCode.
	UpgradeGUID string `yaml:"upgrade_guid"`
	ExeName     string `yaml:"exe_name"`
	// InstallScope picks between InstallScopeAllUsers (default) and
	// InstallScopeCurrentUser — an author-time-only choice on both
	// winexe/winmsi, the same way DesktopShortcut/AutostartAtLogin are on
	// winmsi: a real runtime "install for me or everyone?" picker needs a
	// dialog neither backend's bundled UI has (wixl's WixUI_Minimal has no
	// InstallScopeDlg/WixUI_Advanced equivalent - confirmed by checking the
	// installed msitools build's own bundled ext/ui directory - and a real
	// NSIS equivalent would need bundling the external UAC plugin to
	// elevate only after the choice is made, out of proportion for this).
	//
	// All-users (the default, and this project's pre-existing-only
	// behavior before this field existed) requires admin elevation and
	// installs into a machine-wide location every account can see/run
	// (Program Files on winexe, ProgramFiles64Folder on winmsi) - this is
	// what RequestExecutionLevel admin / ALLUSERS=1 have always meant here.
	//
	// Current-user needs no elevation at all and installs into a location
	// only the account that ran the installer can see/run
	// ($LOCALAPPDATA on winexe, LocalAppDataFolder on winmsi) - useful on a
	// locked-down machine where the person installing isn't an admin.
	// Switching to it changes more than just the install path: winexe's
	// RequestExecutionLevel becomes "user" (from "admin") and its Programs
	// & Features registration moves from HKLM to HKCU (a non-elevated
	// process can't write HKLM at all), and its shortcuts stay in the
	// per-user Start Menu/Desktop (NSIS's default shell-folder context)
	// rather than being redirected to the all-users one via
	// SetShellVarContext all; winmsi's ALLUSERS property is omitted
	// entirely (not set to "0" - that's not a valid MSI value) rather than
	// "1".
	InstallScope string `yaml:"install_scope,omitempty"`
}

// Known WindowsOptions.InstallScope values.
const (
	InstallScopeAllUsers    = "all_users"
	InstallScopeCurrentUser = "current_user"
)

// MacOSOptions covers .pkg (pkgbuild) specifics.
type MacOSOptions struct {
	BundleExecutable string `yaml:"bundle_executable,omitempty"`
	MinSystemVersion string `yaml:"min_system_version,omitempty"`
	Category         string `yaml:"category,omitempty"`
}

// LinuxOptions covers .deb/.rpm specifics beyond Identity.
type LinuxOptions struct {
	DesktopCategories string `yaml:"desktop_categories,omitempty"`
	DesktopComment    string `yaml:"desktop_comment,omitempty"`
}

// OutputOptions controls where and how built artifacts are named.
type OutputOptions struct {
	Dir              string `yaml:"dir,omitempty"`
	FilenameTemplate string `yaml:"filename_template,omitempty"`
	// CleanOldVersions opts into removing, after a successful build, any
	// sibling file in Dir that looks like an older-version build of the
	// same installer (same filename except for the version) — e.g. a
	// leftover 0.2.0 installer once the project has moved on to 0.3.0. Off
	// by default: never deletes anything unless explicitly turned on, and
	// even then only after the user confirms exactly what will be removed
	// (see buildpanel.go's offerCleanupOldInstallers).
	CleanOldVersions bool `yaml:"clean_old_versions,omitempty"`
	// PostBuildHook is inline shell script text run once, on this build
	// machine, right after every requested target/arch has finished
	// building — unlike Hooks.PreInstall/PostUninstall (baked into the
	// installer package itself, run later on a different, target
	// machine), this runs immediately, locally, as part of the same
	// build. Only runs when every target succeeded (see
	// packager.Summary) — skipped with a clear progress note otherwise,
	// since acting on "all the artifacts" rarely makes sense with some
	// missing. Every successfully built artifact's path is handed to the
	// script via INSTALLERBEAR_OUTPUT_<TARGET> environment variables (see
	// packager's own postBuildHookEnv doc comment for the exact naming,
	// including the _<ARCH> suffix used when a target built more than one
	// arch), plus INSTALLERBEAR_ARTIFACTS (all of them,
	// space-separated) and a few identity basics. A general escape hatch
	// for things this tool doesn't implement natively — uploading to
	// GitHub Releases, code signing, notarization, and so on — rather
	// than growing a dedicated feature for each one.
	PostBuildHook string `yaml:"post_build_hook,omitempty"`
}

// Known target names, shared by config Targets, the CLI --target flag, and
// the packager registry key.
const (
	TargetDEB    = "deb"
	TargetRPM    = "rpm"
	TargetMacPkg = "macpkg"
	TargetWinExe = "winexe"
	TargetWinMSI = "winmsi"
)

// AllTargets lists every target this tool knows how to build, in a stable order.
var AllTargets = []string{TargetDEB, TargetRPM, TargetMacPkg, TargetWinExe, TargetWinMSI}

// TargetGroups maps package.sh-style group aliases to concrete targets.
var TargetGroups = map[string][]string{
	"linux":   {TargetDEB, TargetRPM},
	"mac":     {TargetMacPkg},
	"windows": {TargetWinExe, TargetWinMSI},
	"all":     AllTargets,
}
