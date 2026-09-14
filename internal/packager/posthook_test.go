package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"installerbear/internal/packproject"
)

func envContains(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func TestPostBuildHookEnv_BareKeyWhenOnlyOneArchPerTarget(t *testing.T) {
	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App", Version: "1.0.0"}}
	proj.Output.Dir = "/out"
	results := []BuildResult{
		{Target: TargetDEB, OutputPath: "/out/app.deb"},
		{Target: TargetMacPkg, OutputPath: "/out/app.pkg"},
	}

	env := postBuildHookEnv(proj, results)

	for _, want := range []string{
		"INSTALLERBEAR_APP_NAME=Test App",
		"INSTALLERBEAR_VERSION=1.0.0",
		"INSTALLERBEAR_OUTPUT_DIR=/out",
		"INSTALLERBEAR_OUTPUT_DEB=/out/app.deb",
		"INSTALLERBEAR_OUTPUT_MACPKG=/out/app.pkg",
		"INSTALLERBEAR_ARTIFACTS=/out/app.deb /out/app.pkg",
	} {
		if !envContains(env, want) {
			t.Errorf("expected %q in env, got %v", want, env)
		}
	}
}

func TestPostBuildHookEnv_ArchSuffixWhenMultipleArchesForOneTarget(t *testing.T) {
	proj := &packproject.Project{}
	results := []BuildResult{
		{Target: TargetMacPkg, Arch: "amd64", OutputPath: "/out/app-amd64.pkg"},
		{Target: TargetMacPkg, Arch: "arm64", OutputPath: "/out/app-arm64.pkg"},
	}

	env := postBuildHookEnv(proj, results)

	for _, want := range []string{
		"INSTALLERBEAR_OUTPUT_MACPKG_AMD64=/out/app-amd64.pkg",
		"INSTALLERBEAR_OUTPUT_MACPKG_ARM64=/out/app-arm64.pkg",
	} {
		if !envContains(env, want) {
			t.Errorf("expected %q in env, got %v", want, env)
		}
	}
	for _, e := range env {
		if strings.HasPrefix(e, "INSTALLERBEAR_OUTPUT_MACPKG=") {
			t.Errorf("expected no bare INSTALLERBEAR_OUTPUT_MACPKG key when ambiguous between two arches, got %q", e)
		}
	}
}

func TestPostBuildHookEnv_SkipsFailedAndSkippedResults(t *testing.T) {
	proj := &packproject.Project{}
	results := []BuildResult{
		{Target: TargetDEB, OutputPath: "/out/app.deb"},
		{Target: TargetRPM, Err: errors.New("boom")},
		{Target: TargetWinExe, Skipped: true, Err: errors.New("no makensis")},
	}

	env := postBuildHookEnv(proj, results)

	if !envContains(env, "INSTALLERBEAR_OUTPUT_DEB=/out/app.deb") {
		t.Errorf("expected the successful deb result's env var, got %v", env)
	}
	for _, e := range env {
		if strings.Contains(e, "RPM") || strings.Contains(e, "WINEXE") {
			t.Errorf("expected no env var for a failed/skipped result, got %q", e)
		}
	}
	if !envContains(env, "INSTALLERBEAR_ARTIFACTS=/out/app.deb") {
		t.Errorf("expected INSTALLERBEAR_ARTIFACTS to only list the successful artifact, got %v", env)
	}
}

func TestWritePostBuildHookScript_PrependsShebangWhenMissing(t *testing.T) {
	path, cleanup, err := writePostBuildHookScript("echo hi")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh\nset -e\necho hi") {
		t.Errorf("expected a prepended POSIX shebang, got:\n%s", data)
	}
}

func TestWritePostBuildHookScript_PreservesCustomShebang(t *testing.T) {
	path, cleanup, err := writePostBuildHookScript("#!/usr/bin/env bash\necho hi")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "#!/usr/bin/env bash\necho hi") {
		t.Errorf("expected the custom shebang preserved verbatim, got:\n%s", data)
	}
}

// TestRun_PostBuildHookRunsAfterAllTargetsSucceed is a real end-to-end
// check (not just testing postBuildHookEnv in isolation): actually
// executes the hook as a subprocess via Run() and confirms it received
// the right environment variable for a real (fake) build's output path.
func TestRun_PostBuildHookRunsAfterAllTargetsSucceed(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hook-ran.txt")

	proj := &packproject.Project{Identity: packproject.Identity{Name: "Test App", Version: "1.0.0"}}
	proj.Output.PostBuildHook = fmt.Sprintf(`echo "$INSTALLERBEAR_OUTPUT_DEB" > %q`, marker)

	pkgrs := []Packager{
		fakePackager{target: TargetDEB, buildOutput: "/out/app.deb"},
	}

	Run(context.Background(), proj, pkgrs, nil)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("expected the hook to have run and written the marker file: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "/out/app.deb" {
		t.Errorf("marker content = %q, want the deb output path", got)
	}
}

func TestRun_PostBuildHookSkippedWhenATargetFails(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hook-ran.txt")

	proj := &packproject.Project{}
	proj.Output.PostBuildHook = fmt.Sprintf("touch %q", marker)

	pkgrs := []Packager{
		fakePackager{target: TargetDEB, buildOutput: "/out/app.deb"},
		fakePackager{target: TargetRPM, buildErr: errors.New("boom")},
	}

	var lines []string
	Run(context.Background(), proj, pkgrs, func(_ Target, line string) { lines = append(lines, line) })

	if _, err := os.Stat(marker); err == nil {
		t.Error("expected the hook NOT to run when a target failed")
	}
	if !strings.Contains(strings.Join(lines, "\n"), "skipped") {
		t.Errorf("expected a 'skipped' progress note explaining why, got:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRun_NoPostBuildHookConfiguredIsANoOp(t *testing.T) {
	proj := &packproject.Project{}
	pkgrs := []Packager{fakePackager{target: TargetDEB, buildOutput: "/out/app.deb"}}

	var lines []string
	Run(context.Background(), proj, pkgrs, func(_ Target, line string) { lines = append(lines, line) })

	for _, l := range lines {
		if strings.Contains(l, "post_build_hook") {
			t.Errorf("expected no post_build_hook activity when Output.PostBuildHook is blank, got: %s", l)
		}
	}
}
