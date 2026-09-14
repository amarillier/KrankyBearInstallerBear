package packager

import "strings"

// FileAssociationProgID computes a stable per-app, per-extension ProgID
// for Windows file-type registration (HKCR\<ProgID>) - shared by winexe
// and winmsi so both backends register the exact same identifier for a
// given (app, extension) pair, e.g. "myapp.myp" for app "My App" and
// extension ".myp". Not itself user-facing or stored in the schema -
// packproject.FileAssociation only ever asks for Extension/Description,
// this is purely an internal registry-key naming detail.
func FileAssociationProgID(appName, extension string) string {
	return Slug(appName) + strings.ToLower(extension)
}

// FileAssociationDescription returns desc if set, otherwise a generic
// "<AppName> File" fallback so Explorer/a file manager never shows a
// blank file-type name for an association whose author didn't bother
// setting Description.
func FileAssociationDescription(desc, appName string) string {
	if desc != "" {
		return desc
	}
	return appName + " File"
}

// FileAssociationContentType computes a plausible, collision-avoiding
// vendor MIME type for a custom extension this app owns - shared by
// winmsi (WiX's <Extension ContentType='...'> attribute, harmless
// metadata there, not load-bearing for the association itself) and
// debrpm's shared-mime-info package (where it's the actual real MIME
// type Linux registers and matches files against). "application/x-" is
// the conventional prefix for a non-standardized, vendor-specific type.
func FileAssociationContentType(appName, extension string) string {
	return "application/x-" + Slug(appName) + "-" + strings.TrimPrefix(strings.ToLower(extension), ".")
}
