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
