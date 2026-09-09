package innoimport

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseTemplatePackageConfig_RealFixture parses this repo's own real,
// committed build-config.sh and package.sh — see build-config.sh's own
// header comment: it's meant to be the single source of truth for these
// values, which is exactly the assumption this parser leans on.
func TestParseTemplatePackageConfig_RealFixture(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	res, err := ParseTemplatePackageConfig(dir)
	if err != nil {
		t.Fatalf("ParseTemplatePackageConfig: %v", err)
	}

	if res.Name != "KrankyBear InstallerBear" {
		t.Errorf("Name = %q", res.Name)
	}
	if res.URL != "https://github.com/amarillier/KrankyBearInstallerBear" {
		t.Errorf("URL = %q", res.URL)
	}
	if res.Publisher != "amarillier@gmail.com" {
		t.Errorf("Publisher = %q", res.Publisher)
	}
	if res.BundleID != "com.github.amarillier.KrankyBearInstallerBear" {
		t.Errorf("BundleID = %q, want the CFBundleIdentifier from Info-plist.txt", res.BundleID)
	}
	if res.Vendor != "Allan Marillier" {
		t.Errorf("Vendor = %q", res.Vendor)
	}
	if res.ISSPath != "Inno/KrankyBearInstallerBear.iss" {
		t.Errorf("ISSPath = %q", res.ISSPath)
	}
	if res.Description == "" {
		t.Error("expected Description to be picked up from package.sh's --description")
	}

	if res.License != "GPL v3" {
		t.Errorf("License = %q, want GPL v3 (from KB_LICENSE_DEFAULT)", res.License)
	}
}

// TestParseTemplatePackageConfig_BundleIDIndependentOfBuildConfig confirms
// a bare Info.plist (no build-config.sh at all — a project that isn't
// descended from this template, say) still yields a BundleID, since
// nothing else about Identity.ID scans for one otherwise (see
// suggestBundleID in identityform.go — it can only ever guess).
func TestParseTemplatePackageConfig_BundleIDIndependentOfBuildConfig(t *testing.T) {
	dir := t.TempDir()
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.testapp</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(dir, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ParseTemplatePackageConfig(dir)
	if err != nil {
		t.Fatalf("ParseTemplatePackageConfig: %v", err)
	}
	if res.BundleID != "com.example.testapp" {
		t.Errorf("BundleID = %q, want com.example.testapp", res.BundleID)
	}
	if res.Name != "" {
		t.Errorf("Name = %q, want empty (no build-config.sh present)", res.Name)
	}
}

func TestParseTemplatePackageConfig_NoBuildConfig(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseTemplatePackageConfig(dir)
	if err != nil {
		t.Fatalf("expected no error for a directory with no build-config.sh, got %v", err)
	}
	if res.Name != "" || res.Version != "" || res.URL != "" || len(res.Skipped) != 0 {
		t.Errorf("expected an empty result, got %+v", res)
	}
}

func TestParseTemplatePackageConfig_TolerantOfCommentsAndUnquoted(t *testing.T) {
	dir := t.TempDir()
	content := `#!/usr/bin/env bash
# a comment line

export KB_PROJECT_TITLE="Test App"
export KB_VERSION_DEFAULT=1.2.3 # trailing comment
`
	if err := os.WriteFile(filepath.Join(dir, "build-config.sh"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ParseTemplatePackageConfig(dir)
	if err != nil {
		t.Fatalf("ParseTemplatePackageConfig: %v", err)
	}
	if res.Name != "Test App" {
		t.Errorf("Name = %q, want Test App", res.Name)
	}
	if res.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", res.Version)
	}
}
