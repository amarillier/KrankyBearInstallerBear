package main

import (
	"os"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// iconThumbnailSize is deliberately small — this is a "does this look like
// the icon I meant to pick" glance, not a preview pane, so it stays inside
// the Icons form row rather than pushing everything else down.
const iconThumbnailSize = 48

// buildIdentityTab lays out Identity/Windows/macOS/Linux/Output as separate
// stacked Forms (rather than one giant one) so each OS-specific section
// reads as its own group. Every field's OnChanged writes straight into
// e.proj — see editorwidgets.go's bindEntry.
func (e *editor) buildIdentityTab() fyne.CanvasObject {
	e.nameEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Name = v; e.updateWindowTitle() })
	e.idEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.ID = v })
	generateID := widget.NewButton("Generate ID", func() { e.idEntry.SetText(suggestBundleID(e.proj)) })
	e.versionEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Version = v })
	e.publisherEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Publisher = v })
	e.vendorEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Vendor = v })
	e.urlEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.URL = v })
	e.descEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Description = v })
	e.licenseNameEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.License = v })
	e.licenseNameEntry.SetPlaceHolder("e.g. MIT, GPL v3 (used as Linux package metadata)")
	e.licenseEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.LicenseFile = v })

	identityForm := widget.NewForm(
		widget.NewFormItem("Name", e.nameEntry),
		widget.NewFormItem("ID", container.NewBorder(nil, nil, nil, generateID, e.idEntry)),
		widget.NewFormItem("Version", e.versionEntry),
		widget.NewFormItem("Publisher", e.publisherEntry),
		widget.NewFormItem("Vendor", e.vendorEntry),
		widget.NewFormItem("URL", e.urlEntry),
		widget.NewFormItem("Description", e.descEntry),
		widget.NewFormItem("License", e.licenseNameEntry),
		widget.NewFormItem("License file", newBrowseFileRow(e.win, e.licenseEntry)),
	)

	e.icoEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Icons.ICO = v })
	e.icnsEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Identity.Icons.ICNS = v })
	e.pngIconThumbnail = canvas.NewImageFromFile("")
	e.pngIconThumbnail.FillMode = canvas.ImageFillContain
	e.pngIconThumbnail.SetMinSize(fyne.NewSize(iconThumbnailSize, iconThumbnailSize))
	e.pngIconThumbnail.Hide()
	e.pngEntry = bindEntry(widget.NewEntry(), func(v string) {
		e.proj.Identity.Icons.PNG = v
		e.refreshPNGIconThumbnail(v)
	})

	iconsForm := widget.NewForm(
		widget.NewFormItem("Windows icon (.ico)", newBrowseFileRow(e.win, e.icoEntry, ".ico")),
		widget.NewFormItem("macOS icon (.icns)", newBrowseFileRow(e.win, e.icnsEntry, ".icns")),
		widget.NewFormItem("Linux icon (.png)", container.NewBorder(nil, nil, nil, e.pngIconThumbnail,
			newBrowseFileRow(e.win, e.pngEntry, ".png"))),
	)

	e.guidEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Windows.UpgradeGUID = v })
	generateGUID := widget.NewButton("Generate GUID", func() {
		e.guidEntry.SetText("{" + uuid.NewString() + "}")
	})
	e.exeNameEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Windows.ExeName = v })

	e.launchAfterInstallCheck = widget.NewCheck("", func(v bool) { e.proj.InstallExperience.LaunchAfterInstall = v })
	e.desktopShortcutCheck = widget.NewCheck("", func(v bool) { e.proj.InstallExperience.DesktopShortcut = v })
	e.autostartAtLoginCheck = widget.NewCheck("", func(v bool) { e.proj.InstallExperience.AutostartAtLogin = v })

	windowsForm := widget.NewForm(
		widget.NewFormItem("Upgrade GUID", container.NewBorder(nil, nil, nil, generateGUID, e.guidEntry)),
		widget.NewFormItem("Exe name", e.exeNameEntry),
		// Both Setup.exe and .msi always create a Start Menu shortcut; these
		// three are the only Windows-specific install-experience choices
		// exposed today (see packproject.InstallExperience's own doc
		// comment on why they're author-time YAML settings, not something
		// exposed identically on macOS/Linux - both backends there just log
		// a note that these have no effect rather than silently ignoring
		// them, see macpkg/debrpm's own Build()).
		widget.NewFormItem("Launch after install", e.launchAfterInstallCheck),
		widget.NewFormItem("Desktop shortcut", e.desktopShortcutCheck),
		widget.NewFormItem("Run at startup (autostart)", e.autostartAtLoginCheck),
	)

	e.macExecEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.MacOS.BundleExecutable = v })
	e.macMinOSEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.MacOS.MinSystemVersion = v })
	e.macCategoryEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.MacOS.Category = v })

	macForm := widget.NewForm(
		widget.NewFormItem("Bundle executable", e.macExecEntry),
		widget.NewFormItem("Min system version", e.macMinOSEntry),
		widget.NewFormItem("Category", e.macCategoryEntry),
	)

	e.linuxCategoriesEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Linux.DesktopCategories = v })
	e.linuxCommentEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Linux.DesktopComment = v })

	linuxForm := widget.NewForm(
		widget.NewFormItem("Desktop categories", e.linuxCategoriesEntry),
		widget.NewFormItem("Desktop comment", e.linuxCommentEntry),
	)

	e.outputDirEntry = bindEntry(widget.NewEntry(), func(v string) { e.proj.Output.Dir = v })
	outputForm := widget.NewForm(
		widget.NewFormItem("Output directory", newBrowseFolderRow(e.win, e.outputDirEntry)),
	)

	return container.NewVScroll(container.NewVBox(
		sectionHeader("Identity"), identityForm,
		sectionHeader("Icons"), iconsForm,
		sectionHeader("Windows"), windowsForm,
		sectionHeader("macOS"), macForm,
		sectionHeader("Linux"), linuxForm,
		sectionHeader("Output"), outputForm,
	))
}

// refreshPNGIconThumbnail shows a small live preview of the Linux (.png)
// icon next to its field — typing a path or using Browse both go through
// this, since bindEntry/newBrowseFileRow both ultimately fire the same
// OnChanged. .ico/.icns get no such preview: Go has no standard decoder for
// either format, and pulling in a third-party one per format for a thumbnail
// isn't worth it (deliberately kept lean rather than adding dependencies
// for a small benefit).
//
// A blank or unreadable path hides the thumbnail rather than showing
// Fyne's broken-image placeholder — path is resolved against proj.BaseDir
// first (Icons.PNG is stored relative like every other project path), and
// stat'd before handing it to canvas.Image so a stale/typo'd path never
// reaches Fyne's own image decoding at all.
func (e *editor) refreshPNGIconThumbnail(pngPath string) {
	resolved := e.proj.ResolvePath(pngPath)
	if resolved == "" {
		e.pngIconThumbnail.Hide()
		return
	}
	if info, err := os.Stat(resolved); err != nil || info.IsDir() {
		e.pngIconThumbnail.Hide()
		return
	}
	e.pngIconThumbnail.File = resolved
	e.pngIconThumbnail.Refresh()
	e.pngIconThumbnail.Show()
}

// suggestBundleID drafts a starting-point reverse-DNS identifier — this is
// what proj.Identity.ID actually becomes on macOS (pkgbuild --identifier
// and Info.plist's CFBundleIdentifier; see internal/packager/macpkg), so
// unlike the Windows Upgrade GUID (a real random token by design) this
// deliberately isn't random: reverse-DNS is Apple's own convention.
//
// Preference order: a github.com/<owner>/... URL gives the precise
// com.github.<owner> form (the actual convention this tool's own author
// uses, confirmed against this very project's own main.go appID constant);
// otherwise an email-looking Publisher/Vendor has its domain reversed
// (someone@example.com -> com.example); otherwise Publisher/Vendor is just
// slugged under a generic com. prefix. Either way it's a draft the user is
// expected to edit, not a guaranteed-final value.
func suggestBundleID(proj *packproject.Project) string {
	domain := githubOwnerDomain(proj.Identity.URL)

	if domain == "" {
		source := proj.Identity.Publisher
		if source == "" {
			source = proj.Identity.Vendor
		}
		switch {
		case source == "":
			domain = "com.example"
		default:
			if at := strings.LastIndex(source, "@"); at >= 0 && at < len(source)-1 {
				parts := strings.Split(strings.ToLower(source[at+1:]), ".")
				if len(parts) > 1 {
					slices.Reverse(parts)
					domain = strings.Join(parts, ".")
					break
				}
			}
			domain = "com." + packager.Slug(source)
		}
	}

	name := packager.Slug(proj.Identity.Name)
	if name == "" {
		name = "app"
	}
	return domain + "." + name
}

// githubOwnerDomain returns "com.github.<owner>" when rawURL points at a
// github.com/<owner>/<repo> repo, or "" if it doesn't parse or isn't a
// GitHub URL — the precise-signal case suggestBundleID prefers over
// guessing from Publisher/Vendor.
func githubOwnerDomain(rawURL string) string {
	owner, _, ok := packager.ParseGitHubURL(rawURL)
	if !ok {
		return ""
	}
	return "com.github." + packager.Slug(owner)
}

func sectionHeader(text string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

// refreshIdentityTab pushes e.proj's current values into every Identity-tab
// widget — SetText fires each one's OnChanged too, which just writes the
// same value straight back into e.proj (see bindEntry); harmless.
func (e *editor) refreshIdentityTab() {
	e.nameEntry.SetText(e.proj.Identity.Name)
	e.idEntry.SetText(e.proj.Identity.ID)
	e.versionEntry.SetText(e.proj.Identity.Version)
	e.publisherEntry.SetText(e.proj.Identity.Publisher)
	e.vendorEntry.SetText(e.proj.Identity.Vendor)
	e.urlEntry.SetText(e.proj.Identity.URL)
	e.descEntry.SetText(e.proj.Identity.Description)
	e.licenseNameEntry.SetText(e.proj.Identity.License)
	e.licenseEntry.SetText(e.proj.Identity.LicenseFile)
	e.icoEntry.SetText(e.proj.Identity.Icons.ICO)
	e.icnsEntry.SetText(e.proj.Identity.Icons.ICNS)
	e.pngEntry.SetText(e.proj.Identity.Icons.PNG)
	e.guidEntry.SetText(e.proj.Windows.UpgradeGUID)
	e.exeNameEntry.SetText(e.proj.Windows.ExeName)
	e.launchAfterInstallCheck.SetChecked(e.proj.InstallExperience.LaunchAfterInstall)
	e.desktopShortcutCheck.SetChecked(e.proj.InstallExperience.DesktopShortcut)
	e.autostartAtLoginCheck.SetChecked(e.proj.InstallExperience.AutostartAtLogin)
	e.macExecEntry.SetText(e.proj.MacOS.BundleExecutable)
	e.macMinOSEntry.SetText(e.proj.MacOS.MinSystemVersion)
	e.macCategoryEntry.SetText(e.proj.MacOS.Category)
	e.linuxCategoriesEntry.SetText(e.proj.Linux.DesktopCategories)
	e.linuxCommentEntry.SetText(e.proj.Linux.DesktopComment)
	e.outputDirEntry.SetText(e.proj.Output.Dir)
	e.updateWindowTitle()
}
