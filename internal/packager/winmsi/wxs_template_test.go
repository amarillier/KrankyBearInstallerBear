package winmsi

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files instead of comparing against them")

func sampleWxsData() wxsData {
	root := &dirNode{
		ID:   "INSTALLDIR",
		Name: "Test App",
		Files: []fileNode{
			{ID: "file_TestApp_exe", ComponentID: "cmp_TestApp_exe", Name: "TestApp.exe", Source: `C:\src\bin\TestApp.exe`},
			{ID: "file_License_txt", ComponentID: "cmp_License_txt", Name: "License.txt", Source: `C:\src\LICENSE`},
		},
		Dirs: []*dirNode{
			{
				ID:   "dir_assets",
				Name: "assets",
				Dirs: []*dirNode{
					{
						ID:   "dir_images",
						Name: "images",
						Files: []fileNode{
							{ID: "file_icon_png", ComponentID: "cmp_icon_png", Name: "icon.png", Source: `C:\src\assets\images\icon.png`},
						},
					},
				},
			},
		},
	}

	return wxsData{
		AppName:             "Test App",
		Manufacturer:        "Someone",
		Version:             "1.2.3",
		UpgradeGUID:         "4578B785-DB27-44FF-B3F9-2713B327BB90",
		ProductDescription:  "Test App Installer",
		Root:                root,
		AllComponentIDs:     []string{"cmp_TestApp_exe", "cmp_License_txt", "cmp_icon_png"},
		ProgramMenuDirID:    "dir_Test_App_menu",
		ShortcutComponentID: "cmp_shortcut",
		ShortcutTargetName:  "TestApp.exe",
	}
}

func TestWxsTemplate_License(t *testing.T) {
	data := sampleWxsData()
	data.HasLicense = true

	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"<UIRef Id='WixUI_Minimal'/>", "<Condition Message=", `ACCEPTEULA="1"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when HasLicense is true:\n%s", want, out)
		}
	}
}

func TestWxsTemplate_NoLicense(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{"WixUI_Minimal", "ACCEPTEULA", "<Condition"} {
		if bytes.Contains([]byte(out), []byte(unwanted)) {
			t.Errorf("expected no %q in output when HasLicense is false:\n%s", unwanted, out)
		}
	}
}

func TestWxsTemplate_LaunchAfterInstall(t *testing.T) {
	data := sampleWxsData()
	data.LaunchAfterInstall = true
	data.BinaryFileID = "file_TestApp_exe"

	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	// WixUI_Minimal is required even without a real license: its
	// ExitDialog is the only place the "Launch now" checkbox mechanism
	// lives (verified empirically - see build.go's own comment).
	for _, want := range []string{
		"<UIRef Id='WixUI_Minimal'/>",
		"<Property Id='WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT'",
		// Regression test for a real bug caught in live Windows testing:
		// the checkbox appeared but wasn't checked by default, because
		// only ...CHECKBOXTEXT (the label) was ever set - the *separate*
		// property that drives the checked state is this one.
		"<Property Id='WIXUI_EXITDIALOGOPTIONALCHECKBOX' Value='1'/>",
		"<CustomAction Id='LaunchApplication' FileKey='file_TestApp_exe'",
		"<Fragment>",
		"Event='DoAction' Value='LaunchApplication'",
		"WIXUI_EXITDIALOGOPTIONALCHECKBOX = 1 and NOT Installed",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when LaunchAfterInstall is true:\n%s", want, out)
		}
	}
	// No real license was set, so the ACCEPTEULA gate (which only makes
	// sense when there's something real to accept) must not appear.
	if bytes.Contains([]byte(out), []byte("ACCEPTEULA")) {
		t.Errorf("expected no ACCEPTEULA condition when HasLicense is false, even with LaunchAfterInstall true:\n%s", out)
	}
}

func TestWxsTemplate_NoLaunchAfterInstall(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, unwanted := range []string{"LaunchApplication", "WIXUI_EXITDIALOGOPTIONALCHECKBOX", "<Fragment>"} {
		if bytes.Contains([]byte(out), []byte(unwanted)) {
			t.Errorf("expected no %q in output when LaunchAfterInstall is false:\n%s", unwanted, out)
		}
	}
}

func TestWxsTemplate_DesktopShortcut(t *testing.T) {
	data := sampleWxsData()
	data.DesktopShortcut = true
	data.DesktopShortcutComponentID = "cmp_desktop_shortcut"

	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"<Directory Id='DesktopFolder'>",
		"<Component Id='cmp_desktop_shortcut' Guid='*'>",
		"<Shortcut Id='DesktopShortcut'",
		"<ComponentRef Id='cmp_desktop_shortcut'/>",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when DesktopShortcut is true:\n%s", want, out)
		}
	}
}

func TestWxsTemplate_NoDesktopShortcut(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("DesktopFolder")) {
		t.Errorf("expected no DesktopFolder reference when DesktopShortcut is false:\n%s", buf.String())
	}
}

// TestWxsTemplate_AutostartAtLogin covers winmsi's author-time-only variant
// (no end-user checkbox - wixl's bundled UI has no Components-page
// equivalent, unlike winexe's real Section /o checkbox): the Component's
// mere presence in MainFeature is itself the opt-in.
func TestWxsTemplate_AutostartAtLogin(t *testing.T) {
	data := sampleWxsData()
	data.AutostartAtLogin = true
	data.AutostartComponentID = "cmp_autostart"

	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"<Component Id='cmp_autostart' Guid='*'>",
		`Key='Software\Microsoft\Windows\CurrentVersion\Run'`,
		"<ComponentRef Id='cmp_autostart'/>",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when AutostartAtLogin is true:\n%s", want, out)
		}
	}
}

func TestWxsTemplate_NoAutostartAtLogin(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("CurrentVersion\\Run")) {
		t.Errorf("expected no autostart Run key reference when AutostartAtLogin is false:\n%s", buf.String())
	}
}

func TestWxsTemplate_Golden(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "sample.wxs.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if buf.String() != string(want) {
		t.Errorf("rendered .wxs does not match %s (run with -update to see/accept the diff)\n--- got ---\n%s\n--- want ---\n%s", goldenPath, buf.String(), want)
	}
}

// TestWxsTemplate_ARPProductIcon is a regression test for a real gap
// found in live Windows testing: without ARPPRODUCTICON, Windows shows a
// generic default icon for this product in "Apps & Features"/Programs
// and Features, unlike most other installed apps - confirmed feasible by
// compiling a real test .msi with an Icon table entry + ARPPRODUCTICON
// and checking both tables came out correct.
func TestWxsTemplate_ARPProductIcon(t *testing.T) {
	data := sampleWxsData()
	data.IconFile = `C:\src\icon.ico`

	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`<Icon Id='ProductIcon' SourceFile='C:\src\icon.ico'/>`,
		"<Property Id='ARPPRODUCTICON' Value='ProductIcon'/>",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected %q in output when IconFile is set:\n%s", want, out)
		}
	}
}

func TestWxsTemplate_NoARPProductIconWithoutIconFile(t *testing.T) {
	var buf bytes.Buffer
	if err := wxsTemplate.Execute(&buf, sampleWxsData()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("ARPPRODUCTICON")) {
		t.Errorf("expected no ARPPRODUCTICON when IconFile is empty:\n%s", buf.String())
	}
}
