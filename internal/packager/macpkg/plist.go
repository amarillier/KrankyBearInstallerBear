package macpkg

import (
	"bytes"
	"encoding/xml"
	"strings"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

// hasPlistSource reports whether the project directory has a file
// BuildAppBundle can use as a real, functional Info.plist source - either
// a genuine Info.plist (the project's own hand-placed one), or
// Info-plist.txt (this project's inert-by-default placeholder,
// promoted to functional only when FileAssociations need it - see
// BuildAppBundle's own comment on this). Used by build.go to decide
// whether file_associations actually took effect on macOS this build, or
// whether it's still a no-op worth a clear progress note.
func hasPlistSource(proj *packproject.Project) bool {
	return fileExists(proj.ResolvePath("Info.plist")) || fileExists(proj.ResolvePath("Info-plist.txt"))
}

// injectFileAssociationDocTypes returns plistContent with a
// CFBundleDocumentTypes array appended for each of proj.FileAssociations,
// so a real Info.plist (or Info-plist.txt promoted to one) can actually
// make file-type association work on macOS - the one platform this
// project's FileAssociations feature otherwise can't reach at all, since
// macpkg never synthesizes a plist from scratch (see FileAssociation's own
// doc comment in packproject/schema.go for the full policy this responds
// to).
//
// Returns (content, false) unchanged in every case where injecting would
// be wrong or unsafe rather than risking a corrupted plist: no
// associations configured, the file already defines its own
// CFBundleDocumentTypes (an author's own hand-written entry always wins,
// never fought or duplicated), or the file's structure doesn't match the
// standard single-root-<dict> plist shape closely enough to find a safe
// insertion point.
func injectFileAssociationDocTypes(plistContent []byte, proj *packproject.Project, icnsFileName string) ([]byte, bool) {
	if len(proj.FileAssociations) == 0 {
		return plistContent, false
	}
	if bytes.Contains(plistContent, []byte("CFBundleDocumentTypes")) {
		return plistContent, false
	}

	// A standard plist is exactly one root <dict> wrapped in
	// <plist>...</plist> - by XML nesting, the LAST "</dict>" appearing
	// before "</plist>" is guaranteed to be that root dict's own closing
	// tag, since any dict nested inside it must close before it does.
	plistEnd := bytes.LastIndex(plistContent, []byte("</plist>"))
	if plistEnd < 0 {
		return plistContent, false
	}
	dictEnd := bytes.LastIndex(plistContent[:plistEnd], []byte("</dict>"))
	if dictEnd < 0 {
		return plistContent, false
	}

	var b strings.Builder
	b.WriteString("\t<key>CFBundleDocumentTypes</key>\n\t<array>\n")
	for _, assoc := range proj.FileAssociations {
		ext := strings.TrimPrefix(assoc.Extension, ".")
		desc := packager.FileAssociationDescription(assoc.Description, proj.Identity.Name)
		b.WriteString("\t\t<dict>\n\t\t\t<key>CFBundleTypeExtensions</key>\n\t\t\t<array>\n\t\t\t\t<string>")
		_ = xml.EscapeText(&b, []byte(ext))
		b.WriteString("</string>\n\t\t\t</array>\n\t\t\t<key>CFBundleTypeName</key>\n\t\t\t<string>")
		_ = xml.EscapeText(&b, []byte(desc))
		b.WriteString("</string>\n\t\t\t<key>CFBundleTypeRole</key>\n\t\t\t<string>Editor</string>\n")
		b.WriteString("\t\t\t<key>LSHandlerRank</key>\n\t\t\t<string>Owner</string>\n")
		if icnsFileName != "" {
			b.WriteString("\t\t\t<key>CFBundleTypeIconFile</key>\n\t\t\t<string>")
			_ = xml.EscapeText(&b, []byte(icnsFileName))
			b.WriteString("</string>\n")
		}
		b.WriteString("\t\t</dict>\n")
	}
	b.WriteString("\t</array>\n")

	out := make([]byte, 0, len(plistContent)+b.Len())
	out = append(out, plistContent[:dictEnd]...)
	out = append(out, []byte(b.String())...)
	out = append(out, plistContent[dictEnd:]...)
	return out, true
}
