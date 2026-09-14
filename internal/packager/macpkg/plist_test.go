package macpkg

import (
	"strings"
	"testing"

	"installerbear/internal/packproject"
)

const testPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>TestApp</string>
</dict>
</plist>
`

func TestInjectFileAssociationDocTypes_NoAssociationsReturnsUnchanged(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "TestApp"}}

	got, injected := injectFileAssociationDocTypes([]byte(testPlist), proj, "")
	if injected {
		t.Error("expected injected=false with no FileAssociations configured")
	}
	if string(got) != testPlist {
		t.Errorf("content should be returned unchanged, got:\n%s", got)
	}
}

func TestInjectFileAssociationDocTypes_AddsEntries(t *testing.T) {
	proj := &packproject.Project{
		Identity: packproject.Identity{Name: "TestApp"},
		FileAssociations: []packproject.FileAssociation{
			{Extension: ".myp", Description: "My App Project"},
		},
	}

	got, injected := injectFileAssociationDocTypes([]byte(testPlist), proj, "TestApp.icns")
	if !injected {
		t.Fatal("expected injected=true")
	}
	s := string(got)
	for _, want := range []string{
		"<key>CFBundleDocumentTypes</key>",
		"<string>myp</string>",
		"<string>My App Project</string>",
		"<key>CFBundleTypeRole</key>",
		"<string>Editor</string>",
		"<key>LSHandlerRank</key>",
		"<string>Owner</string>",
		"<key>CFBundleTypeIconFile</key>",
		"<string>TestApp.icns</string>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, s)
		}
	}
	// The new block must land inside the root <dict>, before its closing
	// tag - not appended after </plist> or anywhere else that would break
	// the file's structure.
	if strings.Index(s, "<key>CFBundleDocumentTypes</key>") > strings.Index(s, "</dict>") {
		t.Error("expected CFBundleDocumentTypes to be inserted before the root dict's closing tag")
	}
}

func TestInjectFileAssociationDocTypes_NoIconFileOmitsIconKey(t *testing.T) {
	proj := &packproject.Project{
		Identity:         packproject.Identity{Name: "TestApp"},
		FileAssociations: []packproject.FileAssociation{{Extension: ".myp"}},
	}

	got, _ := injectFileAssociationDocTypes([]byte(testPlist), proj, "")
	if strings.Contains(string(got), "CFBundleTypeIconFile") {
		t.Error("expected no CFBundleTypeIconFile key when no ICNS icon is set")
	}
	// Falls back to "<AppName> File" when Description is blank, matching
	// every other backend's packager.FileAssociationDescription behavior.
	if !strings.Contains(string(got), "<string>TestApp File</string>") {
		t.Errorf("expected the default description fallback, got:\n%s", got)
	}
}

func TestInjectFileAssociationDocTypes_RespectsExistingDocTypes(t *testing.T) {
	const withOwnTypes = `<?xml version="1.0"?>
<plist version="1.0">
<dict>
	<key>CFBundleDocumentTypes</key>
	<array></array>
</dict>
</plist>
`
	proj := &packproject.Project{
		Identity:         packproject.Identity{Name: "TestApp"},
		FileAssociations: []packproject.FileAssociation{{Extension: ".myp"}},
	}

	got, injected := injectFileAssociationDocTypes([]byte(withOwnTypes), proj, "")
	if injected {
		t.Error("expected injected=false when the plist already defines CFBundleDocumentTypes")
	}
	if string(got) != withOwnTypes {
		t.Error("expected the author's own CFBundleDocumentTypes to be left untouched")
	}
}

func TestInjectFileAssociationDocTypes_MalformedPlistReturnsUnchanged(t *testing.T) {
	proj := &packproject.Project{
		Identity:         packproject.Identity{Name: "TestApp"},
		FileAssociations: []packproject.FileAssociation{{Extension: ".myp"}},
	}

	malformed := []byte("not a plist at all")
	got, injected := injectFileAssociationDocTypes(malformed, proj, "")
	if injected {
		t.Error("expected injected=false for content with no </plist>/</dict>")
	}
	if string(got) != string(malformed) {
		t.Error("expected malformed content to be returned unchanged rather than risking corruption")
	}
}
