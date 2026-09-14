package debrpm

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/goreleaser/nfpm/v2/files"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// desktopFilesystemPaths are the two standard, distro-agnostic locations a
// .deb/.rpm needs to write into for a real app-menu entry: freedesktop.org's
// Desktop Entry spec (/usr/share/applications) and the hicolor icon theme
// spec (/usr/share/icons/hicolor/<size>/apps) — every major desktop
// environment (GNOME, KDE, XFCE, Cinnamon, MATE, ...) reads both, so this
// needs no per-DE branching. 256x256 matches this project's own convention
// (../KrankyBearTemplate's package.sh, and this app's own Windows/macOS
// icon sizing) rather than shipping multiple icon sizes — nfpm/this schema
// have no concept of a multi-resolution icon set, and a single reasonably
// large PNG scales down cleanly in practice.
const (
	desktopFileDir  = "/usr/share/applications"
	desktopIconDir  = "/usr/share/icons/hicolor/256x256/apps"
	desktopIconSize = "256x256"
	// mimePackageDir is where a distro's shared-mime-info database looks
	// for third-party MIME type definitions (the freedesktop.org shared-
	// mime-info spec) - the Linux equivalent of Windows' HKCR ProgId
	// registration for a custom file type packproject.FileAssociations
	// registers.
	mimePackageDir = "/usr/share/mime/packages"
)

// desktopEntry always gets a real app-menu entry (a .desktop file), the
// Linux equivalent of Windows' always-created Start Menu shortcut — unlike
// InstallExperience.DesktopShortcut, this isn't gated behind that toggle:
// a .desktop file is how a Linux app appears in the application launcher
// at all on every major desktop environment, not an optional extra the way
// a literal desktop-icon file is (and most modern desktop environments,
// GNOME chief among them, don't even support a literal desktop icon
// anymore). LinuxOptions.DesktopCategories/DesktopComment (already in the
// schema, previously unused by any backend) drive Categories/Comment;
// Identity.Icons.PNG (if set) is installed into the hicolor icon theme and
// referenced by name, so a real launcher icon works too, not just a
// generic fallback glyph.
func desktopEntry(proj *packproject.Project, installDir, binName string) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Version=1.0\n")
	fmt.Fprintf(&b, "Name=%s\n", proj.Identity.Name)
	if proj.Linux.DesktopComment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", proj.Linux.DesktopComment)
	}
	// Quoted per the Desktop Entry spec's own field-code/quoting rules —
	// installDir is user-editable (InstallLocations.Linux) and can contain
	// spaces even though this project's own default doesn't. %f (a single
	// local file path, per the Desktop Entry spec's own field codes) is
	// appended whenever this app has any FileAssociations, so double-
	// clicking a registered file type in a file manager actually passes
	// its path to the app instead of just launching it with no argument.
	fmt.Fprintf(&b, "Exec=\"%s/%s\"%s\n", strings.TrimRight(installDir, "/"), binName, fileAssociationExecSuffix(proj))
	if proj.Identity.Icons.PNG != "" {
		// Icon=<name>, not a path: freedesktop icon theme lookup resolves
		// this against every icon theme directory (including the one this
		// same package installs into below) by name, not literal path.
		fmt.Fprintf(&b, "Icon=%s\n", binName)
	}
	if cats := normalizeDesktopCategories(proj.Linux.DesktopCategories); cats != "" {
		fmt.Fprintf(&b, "Categories=%s\n", cats)
	}
	if mimeTypes := fileAssociationMimeTypes(proj); mimeTypes != "" {
		fmt.Fprintf(&b, "MimeType=%s\n", mimeTypes)
	}
	b.WriteString("Terminal=false\n")
	return b.String()
}

// fileAssociationExecSuffix returns " %f" when the project has any
// FileAssociations, "" otherwise - see desktopEntry's own comment on why.
func fileAssociationExecSuffix(proj *packproject.Project) string {
	if len(proj.FileAssociations) == 0 {
		return ""
	}
	return " %f"
}

// fileAssociationMimeTypes returns every FileAssociation's computed MIME
// type, semicolon-terminated per the Desktop Entry spec's own MimeType
// field convention (same requirement as Categories - see
// normalizeDesktopCategories), or "" if there are none.
func fileAssociationMimeTypes(proj *packproject.Project) string {
	if len(proj.FileAssociations) == 0 {
		return ""
	}
	var b strings.Builder
	for _, a := range proj.FileAssociations {
		b.WriteString(packager.FileAssociationContentType(proj.Identity.Name, a.Extension))
		b.WriteString(";")
	}
	return b.String()
}

// normalizeDesktopCategories ensures a trailing ";" (the Desktop Entry
// spec requires the Categories value to be semicolon-terminated, not just
// semicolon-separated) without requiring the person typing into the
// Identity tab's plain Entry field to remember that themselves — sibling
// KrankyBear projects' own build-config.sh (KB_DESKTOP_CATEGORIES) already
// store it pre-terminated by convention, so this is a no-op for values
// imported from one of those via "Import Existing Config", and only
// matters for a value typed by hand.
func normalizeDesktopCategories(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasSuffix(trimmed, ";") {
		trimmed += ";"
	}
	return trimmed
}

// mimeInfoXML generates a shared-mime-info package (freedesktop.org's
// shared-mime-info spec) registering every FileAssociation as its own
// MIME type, matched by extension glob - the Linux equivalent of the
// Windows ProgId/Extension registration winexe/winmsi both write to
// HKCR. Returns "" when there are no associations (callers skip writing
// anything in that case).
func mimeInfoXML(proj *packproject.Project) string {
	if len(proj.FileAssociations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">` + "\n")
	for _, a := range proj.FileAssociations {
		contentType := packager.FileAssociationContentType(proj.Identity.Name, a.Extension)
		desc := packager.FileAssociationDescription(a.Description, proj.Identity.Name)
		fmt.Fprintf(&b, "  <mime-type type=%q>\n", xmlEscape(contentType))
		fmt.Fprintf(&b, "    <comment>%s</comment>\n", xmlEscape(desc))
		fmt.Fprintf(&b, "    <glob pattern=\"*%s\"/>\n", xmlEscape(a.Extension))
		b.WriteString("  </mime-type>\n")
	}
	b.WriteString(`</mime-info>` + "\n")
	return b.String()
}

// xmlEscape escapes s for safe use as XML text/attribute content (&, <,
// >, quotes) - Description is free-form author text, not validated
// against XML-safety by packproject.Validate, so this hand-built XML
// (simpler than a full encoding/xml struct marshal for four fixed lines
// per association) must escape it itself rather than risk malformed XML
// from something as ordinary as an author writing "Foo & Bar Format" as
// their file type's description.
func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// writeTempFile writes content to a new temp file and returns its path plus
// a cleanup func to remove it — the same "generate on disk, point an
// nfpm files.Content Source at it" pattern writeHookScript already uses for
// inline Hooks text, generalized here since the .desktop entry needs the
// same treatment (nfpm's Content.Source must be a real file to read from).
func writeTempFile(pattern, content string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}

	name := f.Name()
	return name, func() { os.Remove(name) }, nil
}

// desktopIntegrationContents returns the .desktop file, (if set) the PNG
// icon, and (if any FileAssociations exist) a shared-mime-info package as
// files.Content entries, plus a cleanup func for the temp files it wrote.
// Always includes the .desktop entry (see desktopEntry's own comment on
// why this isn't gated behind DesktopShortcut); the icon entry is only
// added when Identity.Icons.PNG is set, and the MIME package only when
// there's at least one FileAssociation, since there's nothing to install
// otherwise either way.
func desktopIntegrationContents(proj *packproject.Project, installDir, binName string) (files.Contents, func(), error) {
	desktopPath, cleanup, err := writeTempFile("packman-desktop-*.desktop", desktopEntry(proj, installDir, binName))
	if err != nil {
		return nil, nil, fmt.Errorf("writing .desktop entry: %w", err)
	}

	contents := files.Contents{
		{
			Source:      desktopPath,
			Destination: desktopFileDir + "/" + binName + ".desktop",
			Type:        files.TypeFile,
			FileInfo:    &files.ContentFileInfo{Mode: 0o644},
		},
	}
	if proj.Identity.Icons.PNG != "" {
		contents = append(contents, &files.Content{
			Source:      proj.ResolvePath(proj.Identity.Icons.PNG),
			Destination: desktopIconDir + "/" + binName + ".png",
			Type:        files.TypeFile,
		})
	}
	if mimeXML := mimeInfoXML(proj); mimeXML != "" {
		mimePath, mimeCleanup, err := writeTempFile("packman-mime-*.xml", mimeXML)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("writing shared-mime-info package: %w", err)
		}
		prevCleanup := cleanup
		cleanup = func() { prevCleanup(); mimeCleanup() }
		contents = append(contents, &files.Content{
			Source:      mimePath,
			Destination: mimePackageDir + "/" + binName + ".xml",
			Type:        files.TypeFile,
			FileInfo:    &files.ContentFileInfo{Mode: 0o644},
		})
	}
	return contents, cleanup, nil
}

// desktopIntegrationRefreshScript best-effort-refreshes the desktop/icon/
// MIME caches so a newly installed (or removed) app's menu entry, icon,
// and file associations show up without a logout/login - every command is
// silently skipped (|| true) on a system that doesn't have it (a minimal/
// headless box with no desktop environment installed at all, or a distro
// that refreshes these automatically via its own package-manager triggers
// instead). Appended onto whatever Hooks script content already exists
// for the same nfpm script slot, never replacing it - see buildInfo's own
// callers.
func desktopIntegrationRefreshScript(includeIconCache, includeMimeDatabase bool) string {
	var b strings.Builder
	b.WriteString("update-desktop-database -q " + desktopFileDir + " 2>/dev/null || true\n")
	if includeIconCache {
		b.WriteString("gtk-update-icon-cache -q -f /usr/share/icons/hicolor 2>/dev/null || true\n")
	}
	if includeMimeDatabase {
		b.WriteString("update-mime-database /usr/share/mime 2>/dev/null || true\n")
	}
	return b.String()
}
