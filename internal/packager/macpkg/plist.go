package macpkg

import (
	"os"
	"text/template"

	"installerbear/internal/packproject"
)

var plistTemplate = template.Must(template.New("Info.plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>{{.Executable}}</string>
	<key>CFBundleIdentifier</key>
	<string>{{.ID}}</string>
	<key>CFBundleName</key>
	<string>{{.Name}}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>{{.Version}}</string>
	<key>CFBundleVersion</key>
	<string>{{.Version}}</string>
	<key>NSHighResolutionCapable</key>
	<true/>
{{- if .MinSystemVersion}}
	<key>LSMinimumSystemVersion</key>
	<string>{{.MinSystemVersion}}</string>
{{- end}}
{{- if .Category}}
	<key>LSApplicationCategoryType</key>
	<string>{{.Category}}</string>
{{- end}}
{{- if .IconFile}}
	<key>CFBundleIconFile</key>
	<string>{{.IconFile}}</string>
{{- end}}
</dict>
</plist>
`))

type plistData struct {
	Executable, ID, Name, Version, MinSystemVersion, Category, IconFile string
}

// writeInfoPlist writes a real Info.plist. Earlier fpm-based packaging in
// this project's own history renamed this to Info-plist.txt to dodge fpm's
// bundle-detection heuristics during staging — pkgbuild has no such quirk
// and expects a normal Info.plist inside a real .app bundle, so that
// workaround doesn't carry forward here.
func writeInfoPlist(proj *packproject.Project, path string) error {
	data := plistData{
		Executable:       proj.MacOS.BundleExecutable,
		ID:               proj.Identity.ID,
		Name:             proj.Identity.Name,
		Version:          proj.Identity.Version,
		MinSystemVersion: proj.MacOS.MinSystemVersion,
		Category:         proj.MacOS.Category,
	}
	if proj.Identity.Icons.ICNS != "" {
		data.IconFile = proj.Identity.Name + ".icns"
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return plistTemplate.Execute(f, data)
}
