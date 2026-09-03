package main

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var helpWindow fyne.Window

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
		return
	}

	helpWindow = a.NewWindow(appName + " - Help")
	helpWindow.SetIcon(resourceKrankyBearInstallerBearPng)

	helpText := `` + appName + ` - Help

OVERVIEW:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
InstallerBear turns a set of already-built binaries into native installers for
Windows, macOS, and Linux from one shared project file (packman.yaml). A
headless CLI ("packman") built into the same binary can drive the same builds
from a script or CI pipeline.

FEATURES:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Identity tab - name, bundle ID, version, publisher, vendor, URL, description,
  license, icons, plus Windows/macOS/Linux-specific settings and output folder.
• Binaries tab - one file picker per OS x architecture. Fill in more than one
  architecture for an OS to get multi-arch output automatically. "Scan
  folder..." can guess several of these at once from filename conventions.
• Payload tab - bundle extra files/folders alongside the binary, with
  recursive copy and OS filtering. "Scan folder..." proposes entries from a
  directory for you to review before anything is added.
• Build tab - checkboxes per target with live preflight status (is the tool
  installed, is the target supported here), Re-check Tools, Start/Cancel, and
  a streaming build log.
• Packaging backends - Windows Setup.exe (NSIS), Windows .msi (wixl), macOS
  .pkg (real .app bundle + pkgbuild), Linux .deb/.rpm (nfpm). Each target
  builds independently, so one failure doesn't stop the others.
• Headless CLI - "packman build/validate/doctor/list-targets" for CI use,
  sharing the exact same preflight checks as the GUI.

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

KEYBOARD SHORTCUTS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Standard system shortcuts apply:
• Cmd/Ctrl+Q - Quit
• Cmd/Ctrl+W - Close window
• Cmd/Ctrl+M - Minimize
No app-specific shortcuts yet - use the File menu for New/Open/Save/Save As.

KNOWN LIMITATIONS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• No code signing for any output format yet.
• No file associations or custom installer wizard pages.
• No unsaved-changes confirmation on New/Open Project yet.
• Payload entries are edited via a dialog, not inline in the table.

MORE INFORMATION:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
For documentation, bug reports, or feature requests:
📦 GitHub: https://github.com/amarillier/KrankyBearInstallerBear
📄 License: https://github.com/amarillier/KrankyBearInstallerBear/blob/main/LICENSE
📝 Release Notes: Check "Help → Check for Updates"

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
		helpWindow.Hide()
	})

	helpWindow.Show()
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
