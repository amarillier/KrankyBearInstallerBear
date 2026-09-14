package main

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var helpWindow fyne.Window
var helpManagedWindow *managedWindow // hide-all/show-all tracking, see windowregistry.go

// showHelp displays comprehensive help documentation
// Reusable pattern from KrankyBearClock - customize these for your app:
//   - appName: Your application name
//   - resourceKrankyBearInstallerBearPng: Your embedded icon resource
//   - helpText: Your application's help content (see below for structure)
//   - GitHub and License URLs
//
// Help text structure recommendation:
//   - Use section headers with visual separators (━━━)
//   - Group related features together
//   - Include tips, tricks, and known limitations
//   - Add keyboard shortcuts
//   - Provide links to external resources
func showHelp(a fyne.App) {
	if helpWindow != nil && helpWindow.Content().Visible() {
		helpWindow.Show()
		helpWindow.RequestFocus()
		helpManagedWindow.open = true
		return
	}

	helpWindow = a.NewWindow(appName + " - Help")
	helpWindow.SetIcon(resourceKrankyBearInstallerBearPng)
	helpManagedWindow = registerManagedWindow(helpWindow)

	helpText := `` + appName + ` - Help

OVERVIEW:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
InstallerBear turns a set of already-built binaries into native installers for
Windows, macOS, and Linux from one shared project file (installerbear.yaml).
A headless CLI built into the same binary can drive the same builds from a
script or CI pipeline - run with -help or -? for CLI usage.

FEATURES:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Identity tab - name, bundle ID, version, publisher, vendor, URL, description,
  license, icons, plus Windows/macOS/Linux-specific settings, install/build
  hooks, and output folder.
• Binaries tab - one file picker per OS x architecture. Fill in more than one
  architecture for an OS to get multi-arch output automatically. "Scan
  folder..." can guess several of these at once from filename conventions.
• Payload tab - bundle extra files/folders alongside the binary, with
  recursive copy and OS filtering. "Scan folder..." proposes entries from a
  directory for you to review before anything is added.
• File Associations tab - register the app to open file extensions
  (Windows registry entries / Windows .msi ProgId / Linux shared-mime-info
  + .desktop MimeType=). Works on macOS too when the project has a real
  Info.plist or Info-plist.txt to add CFBundleDocumentTypes to.
• Import Existing Config... (File menu, or "installerbear import" on the
  CLI) - best-effort-imports an existing Inno Setup .iss and/or
  KrankyBear-template build-config.sh/package.sh: Identity, Windows Upgrade
  GUID/exe name, Payload, File Associations, and Install Experience
  toggles (Desktop shortcut/Run at startup/Launch after install). Shows
  current -> proposed before anything is applied; safe to re-run.
• Build tab - checkboxes per target with live preflight status (is the tool
  installed, is the target supported here), Re-check Tools, Start/Cancel, and
  a streaming build log.
• Packaging backends - Windows Setup.exe (NSIS), Windows .msi (wixl), macOS
  .pkg (real .app bundle + pkgbuild), Linux .deb/.rpm (nfpm). Each target
  builds independently, so one failure doesn't stop the others.
• Headless CLI - "installerbear build/validate/doctor/list-targets" for CI
  use, sharing the exact same preflight checks as the GUI. Run with -help or
  -? for full usage.

SMART FEATURES:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✨ Theme Support: Light, Dark, or System theme (View menu) - matches your preference.
✨ New Project smart defaults: pick a folder and InstallerBear best-effort-fills
  Publisher, URL, License file, and icons from its git remote/identity,
  LICENSE file, and assets/images icons - never overwrites what you've
  already typed.
✨ Generate ID: drafts a reverse-DNS bundle ID from your GitHub URL or
  Publisher/Vendor.
✨ Generate GUID: produces a fresh Windows Upgrade GUID.
✨ Start Build button doubles as a status light: green (ready), orange
  (building), green/red (last build's result).
✨ Update checker: quiet automatic check once per day, plus an always-available
  manual "Check for Updates".
✨ Window size is remembered across launches.
✨ Toolbar buttons for New/Open/Save/Save As sit above the tabs, alongside
  the File menu.
✨ Tray "Show/Hide All Windows" brings back or hides the whole window stack
  (main window plus any open About/Help/Update window) in one click.
✨ New Sample Project... (File menu + tray) writes a real, fully-featured
  example installerbear.yaml to a location you pick and opens it - a
  working starting point instead of a blank project.
✨ Release Notes viewer (Help menu + tray): opens the installed release
  notes in their own window - no need to go hunting for the file.
✨ Hand-typed "#" comments in installerbear.yaml survive being opened and
  saved by this app - useful for notes, or temporarily commenting out a
  payload/binary/file-association entry. Manual-editing only: there's no
  GUI for adding or editing a comment, this just stops the app from
  silently deleting one you typed directly into the file.
✨ Hooks (Identity tab): Pre-install/Post-uninstall run on the TARGET
  machine, baked into the macOS .pkg/Linux .deb-.rpm package itself. Runs
  as /bin/sh by default (start your own text with a shebang, e.g.
  #!/usr/bin/env python3, to use a different interpreter instead - same
  convention as a normal script file). Windows has no shell interpreter,
  so both fields are simply unavailable there. Pre-install runs on both
  macOS and Linux, before files are installed. Post-uninstall runs on
  Linux only (a .pkg install has no OS-level uninstall action to hook
  into) - on Linux, InstallerBear's own desktop/icon-cache refresh
  commands run right after your own post-uninstall text in the same
  script, so keep that one to plain POSIX shell.
  Post-build hook (Output section) instead runs immediately on THIS build
  machine right after a successful build, with the built artifacts' paths
  passed in as INSTALLERBEAR_OUTPUT_<TARGET> environment variables - a
  general escape hatch for uploading to GitHub Releases, code signing,
  notarization, and anything else this tool doesn't do natively.

KEYBOARD SHORTCUTS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Standard system shortcuts apply:
• Cmd/Ctrl+Q - Quit
• Cmd/Ctrl+W - Close window
• Cmd/Ctrl+M - Minimize
No app-specific shortcuts yet - use the File menu for New/Open/Save/Save As.

KNOWN LIMITATIONS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• No code signing for any output format yet (a post-build hook can call
  your own signing/notarization tools once you have certs, though).
• No custom installer wizard pages. File associations are supported on
  Windows/Linux always, and on macOS when a real Info.plist or
  Info-plist.txt exists to add CFBundleDocumentTypes to - see the File
  Associations tab.
• Windows ARM64 .msi isn't possible - wixl (the tool this project uses to
  build .msi) has no ARM64 support at all. Setup.exe should already work
  for it, untested on real ARM64 Windows hardware.

MORE INFORMATION:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
For documentation, bug reports, or feature requests:
📦 GitHub: https://github.com/amarillier/KrankyBearInstallerBear
📄 License: https://github.com/amarillier/KrankyBearInstallerBear/blob/main/LICENSE
📝 Release Notes: Help → Release Notes

FREE SOFTWARE - Use anywhere, anytime, any purpose!
No registration, no tracking, no phone-home (except manual update checks).
`

	helpLabel := widget.NewLabel(helpText)
	helpLabel.Wrapping = fyne.TextWrapWord

	// Links - update URLs for your project
	githubURL, _ := url.Parse("https://github.com/amarillier/KrankyBearInstallerBear")
	githubLink := widget.NewHyperlink("Visit GitHub Repository", githubURL)
	githubLink.Alignment = fyne.TextAlignCenter

	licenseURL, _ := url.Parse("https://github.com/amarillier/KrankyBearInstallerBear/blob/main/LICENSE")
	licenseLink := widget.NewHyperlink("View License", licenseURL)
	licenseLink.Alignment = fyne.TextAlignCenter

	// Create scrollable area with minimum size for better readability
	scrollContent := container.NewScroll(helpLabel)
	scrollContent.SetMinSize(fyne.NewSize(750, 550))

	// Layout with better proportions
	header := container.NewVBox(
		widget.NewLabelWithStyle(appName+" - Help", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(container.NewHBox(githubLink, licenseLink)),
	)

	content := container.NewBorder(header, footer, nil, nil, scrollContent)

	helpWindow.SetContent(container.NewPadded(content))
	helpWindow.Resize(fyne.NewSize(850, 700))

	helpWindow.SetCloseIntercept(func() {
		helpManagedWindow.open = false
		helpWindow.Hide()
	})

	helpManagedWindow.open = true
	helpWindow.Show()
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
