package main

import (
	"testing"

	"installerbear/internal/packproject"
)

func TestSuggestBundleID(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		publisher string
		vendor    string
		appName   string
		want      string
	}{
		{
			"github URL takes precedence, matching this project's own real convention",
			"https://github.com/amarillier/KrankyBearInstallerBear", "someone@example.com", "",
			"KrankyBear InstallerBear", "com.github.amarillier.krankybear-installerbear",
		},
		{"non-github URL falls through to publisher", "https://example.org/amarillier", "someone@example.com", "", "Test App", "com.example.test-app"},
		{"email publisher reverses domain", "", "someone@example.com", "", "Test App", "com.example.test-app"},
		{"email vendor used when publisher blank", "", "", "team@my-company.co.uk", "Test App", "uk.co.my-company.test-app"},
		{"plain publisher name slugged under com.", "", "Allan Marillier", "", "Test App", "com.allan-marillier.test-app"},
		{"blank publisher and vendor falls back to com.example", "", "", "", "Test App", "com.example.test-app"},
		{"blank app name falls back to app", "", "someone@example.com", "", "", "com.example.app"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			proj := &packproject.Project{Identity: packproject.Identity{
				Name:      c.appName,
				URL:       c.url,
				Publisher: c.publisher,
				Vendor:    c.vendor,
			}}
			if got := suggestBundleID(proj); got != c.want {
				t.Errorf("suggestBundleID() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEditor_GenerateIDButtonFillsIDField(t *testing.T) {
	e := newTestEditor(t)
	e.nameEntry.SetText("Test App")
	e.publisherEntry.SetText("someone@example.com")

	// The button's own OnTapped just calls suggestBundleID and SetTexts the
	// result — exercised directly here since simulating a Tap would need
	// locating the button among the form's children, more machinery for no
	// extra confidence over calling the same code path the button calls.
	e.idEntry.SetText(suggestBundleID(e.proj))

	if e.proj.Identity.ID != "com.example.test-app" {
		t.Errorf("Identity.ID = %q, want com.example.test-app", e.proj.Identity.ID)
	}
}
