// Package packproject defines the per-project packaging config (installerbear.yaml)
// shared by every backend under internal/packager and by both the GUI and CLI.
package packproject

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

	// BaseDir is the directory the project file lives in. Every backend
	// resolves relative Source/LicenseFile/icon paths against it. Set by
	// Load(); leave empty (paths resolve against the process cwd) when
	// constructing a Project by hand, e.g. in tests.
	BaseDir string `yaml:"-"`
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
	Source string `yaml:"source"`
	// Dest is always a destination *directory*, relative to the install
	// root ("" for the root itself) — for both a Recursive entry (Source's
	// own contents land under it) and a single-file entry (the installed
	// filename is always Source's own basename, never taken from Dest).
	// Every backend under internal/packager must honor this the same way.
	Dest      string   `yaml:"dest"`
	Recursive bool     `yaml:"recursive,omitempty"`
	OS        []string `yaml:"os,omitempty"` // empty = all OSes
	Mode      string   `yaml:"mode,omitempty"`
	// Excludes lists filepath.Match glob patterns (matched against each
	// file's path relative to Source) skipped when copying a Recursive
	// entry — the analogue of Inno's [Files] "Excludes:" attribute. Ignored
	// when Recursive is false.
	Excludes []string `yaml:"excludes,omitempty"`
}

// InstallLocations gives the per-OS install root. Any left blank get a
// sensible default filled in by Defaults().
type InstallLocations struct {
	Windows string `yaml:"windows,omitempty"`
	MacOS   string `yaml:"macos,omitempty"`
	Linux   string `yaml:"linux,omitempty"`
}

// Hooks holds optional inline shell snippets run around install/uninstall.
// Both default to empty (no hook) — most projects need neither.
type Hooks struct {
	PreInstall    string `yaml:"pre_install,omitempty"`
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
}

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
