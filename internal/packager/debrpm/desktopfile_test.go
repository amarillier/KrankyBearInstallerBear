package debrpm

import (
	"strings"
	"testing"

	"installerbear/internal/packproject"
)

func TestDesktopEntry_IncludesCoreFields(t *testing.T) {
	proj := &packproject.Project{
		Identity: packproject.Identity{
			Name:  "Test App",
			Icons: packproject.IconSet{PNG: "assets/images/icon.png"},
		},
		Linux: packproject.LinuxOptions{
			DesktopComment:    "A test application",
			DesktopCategories: "Utility;Development",
		},
	}

	out := desktopEntry(proj, "/opt/Test App", "test-app")

	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Test App",
		"Comment=A test application",
		`Exec="/opt/Test App/test-app"`,
		"Icon=test-app",
		"Categories=Utility;Development;",
		"Terminal=false",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in .desktop entry:\n%s", want, out)
		}
	}
}

func TestDesktopEntry_OmitsIconWhenNoPNGSet(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App"}}

	out := desktopEntry(proj, "/opt/TestApp", "test-app")

	if strings.Contains(out, "Icon=") {
		t.Errorf("expected no Icon= line when Icons.PNG is unset:\n%s", out)
	}
}

func TestDesktopEntry_OmitsCommentAndCategoriesWhenUnset(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App"}}

	out := desktopEntry(proj, "/opt/TestApp", "test-app")

	if strings.Contains(out, "Comment=") {
		t.Errorf("expected no Comment= line when DesktopComment is unset:\n%s", out)
	}
	if strings.Contains(out, "Categories=") {
		t.Errorf("expected no Categories= line when DesktopCategories is unset:\n%s", out)
	}
}

func TestNormalizeDesktopCategories(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"Utility;Development;", "Utility;Development;"},
		{"Utility;Development", "Utility;Development;"},
		{"Utility", "Utility;"},
	}
	for _, c := range cases {
		if got := normalizeDesktopCategories(c.in); got != c.want {
			t.Errorf("normalizeDesktopCategories(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDesktopIntegrationRefreshScript_IconCacheOnlyWhenRequested(t *testing.T) {
	withIcon := desktopIntegrationRefreshScript(true, false)
	withoutIcon := desktopIntegrationRefreshScript(false, false)

	if !strings.Contains(withIcon, "update-desktop-database") || !strings.Contains(withIcon, "gtk-update-icon-cache") {
		t.Errorf("expected both refresh commands when includeIconCache=true, got:\n%s", withIcon)
	}
	if !strings.Contains(withoutIcon, "update-desktop-database") {
		t.Errorf("expected the desktop-database refresh even without an icon, got:\n%s", withoutIcon)
	}
	if strings.Contains(withoutIcon, "gtk-update-icon-cache") {
		t.Errorf("expected no icon-cache refresh when includeIconCache=false, got:\n%s", withoutIcon)
	}
}

func TestDesktopEntry_IncludesMimeTypeAndFileArgWhenAssociationsSet(t *testing.T) {
	proj := &packproject.Project{
		Identity: packproject.Identity{Name: "Test App"},
		FileAssociations: []packproject.FileAssociation{
			{Extension: ".myp", Description: "Test App Project"},
		},
	}

	out := desktopEntry(proj, "/opt/Test App", "test-app")

	if !strings.Contains(out, `Exec="/opt/Test App/test-app" %f`) {
		t.Errorf("expected Exec=...%%f when FileAssociations is set:\n%s", out)
	}
	if !strings.Contains(out, "MimeType=application/x-test-app-myp;") {
		t.Errorf("expected a semicolon-terminated MimeType= line:\n%s", out)
	}
}

func TestDesktopEntry_NoMimeTypeOrFileArgWithoutAssociations(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App"}}

	out := desktopEntry(proj, "/opt/TestApp", "test-app")

	if strings.Contains(out, "MimeType=") {
		t.Errorf("expected no MimeType= line without any FileAssociations:\n%s", out)
	}
	if strings.Contains(out, "%f") {
		t.Errorf("expected no %%f Exec suffix without any FileAssociations:\n%s", out)
	}
}

func TestMimeInfoXML_GeneratesValidPackage(t *testing.T) {
	proj := &packproject.Project{
		Identity: packproject.Identity{Name: "Test App"},
		FileAssociations: []packproject.FileAssociation{
			{Extension: ".myp", Description: "Test App Project"},
		},
	}

	out := mimeInfoXML(proj)

	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">`,
		`<mime-type type="application/x-test-app-myp">`,
		`<comment>Test App Project</comment>`,
		`<glob pattern="*.myp"/>`,
		`</mime-info>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in mime-info XML:\n%s", want, out)
		}
	}
}

func TestMimeInfoXML_EmptyWithoutAssociations(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App"}}
	if got := mimeInfoXML(proj); got != "" {
		t.Errorf("expected an empty string without any FileAssociations, got:\n%s", got)
	}
}

// TestXMLEscape_EscapesSpecialCharacters guards mimeInfoXML against
// malformed output when an author's free-form Description contains XML
// metacharacters (a real possibility - Validate never checks Description
// for XML-safety, since it's meant to be plain human-readable text).
func TestXMLEscape_EscapesSpecialCharacters(t *testing.T) {
	got := xmlEscape(`Foo & Bar <Format>`)
	if !strings.Contains(got, "&amp;") || !strings.Contains(got, "&lt;") || !strings.Contains(got, "&gt;") {
		t.Errorf("expected &, <, > to be escaped, got %q", got)
	}
}

func TestDesktopIntegrationRefreshScript_MimeDatabaseOnlyWhenRequested(t *testing.T) {
	withMime := desktopIntegrationRefreshScript(false, true)
	withoutMime := desktopIntegrationRefreshScript(false, false)

	if !strings.Contains(withMime, "update-mime-database") {
		t.Errorf("expected the mime-database refresh when includeMimeDatabase=true, got:\n%s", withMime)
	}
	if strings.Contains(withoutMime, "update-mime-database") {
		t.Errorf("expected no mime-database refresh when includeMimeDatabase=false, got:\n%s", withoutMime)
	}
}
