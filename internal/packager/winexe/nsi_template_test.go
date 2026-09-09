package winexe

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files instead of comparing against them")

func sampleNSIData() nsiData {
	return nsiData{
		AppName:       "Test App",
		AppVersion:    "1.2.3",
		Publisher:     "Someone",
		ExeName:       "TestApp.exe",
		OutFile:       `C:\out\TestAppSetup_1.2.3.exe`,
		InstallDir:    `$PROGRAMFILES64\Test App`,
		IconFile:      `C:\src\icon.ico`,
		BinarySource:  `C:\src\bin\TestApp.exe`,
		LicenseSource: `C:\src\LICENSE`,
		Files: []nsiFileEntry{
			{DestDir: `assets\images`, Source: `C:\src\assets\images`, Recursive: true},
			{DestDir: "", Source: `C:\src\ReleaseNotes.txt`},
		},
	}
}

func TestNSITemplate_Golden(t *testing.T) {
	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, sampleNSIData()); err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "sample.nsi.golden")
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
		t.Errorf("rendered .nsi does not match %s (run with -update to see/accept the diff)\n--- got ---\n%s\n--- want ---\n%s", goldenPath, buf.String(), want)
	}
}

func TestNSITemplate_NoLicenseNoIconNoPayload(t *testing.T) {
	data := sampleNSIData()
	data.LicenseSource = ""
	data.IconFile = ""
	data.Files = nil

	var buf bytes.Buffer
	if err := nsiTemplate.Execute(&buf, data); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{"MUI_PAGE_LICENSE", "MUI_ICON", "MUI_UNICON", "Icon ", "UninstallIcon", "/oname=License.txt"} {
		if bytes.Contains([]byte(out), []byte(unwanted)) {
			t.Errorf("expected no %q in output when license/icon/payload are all empty:\n%s", unwanted, out)
		}
	}
	if !bytes.Contains([]byte(out), []byte(`File "C:\src\bin\TestApp.exe"`)) {
		t.Error("expected the main binary File directive to still be present")
	}
}
