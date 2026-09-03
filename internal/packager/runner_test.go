package packager

import (
	"context"
	"errors"
	"testing"

	"installerbear/internal/packproject"
)

// fakePackager lets tests control exactly which of HostSupported/Preflight/
// Build fail, without needing any real backend or external tool.
type fakePackager struct {
	target         Target
	hostErr        error
	preflightErr   error
	buildErr       error
	buildOutput    string
	buildCallCount *int
	buildArches    *[]string // records opts.Arch for every Build call, in order
}

func (f fakePackager) Target() Target       { return f.target }
func (f fakePackager) HostSupported() error { return f.hostErr }
func (f fakePackager) Preflight() error     { return f.preflightErr }
func (f fakePackager) Build(_ context.Context, _ *packproject.Project, opts BuildOptions, _ ProgressFunc) (string, error) {
	if f.buildCallCount != nil {
		*f.buildCallCount++
	}
	if f.buildArches != nil {
		*f.buildArches = append(*f.buildArches, opts.Arch)
	}
	return f.buildOutput, f.buildErr
}

func TestRun_PartialFailureDoesNotStopOtherTargets(t *testing.T) {
	debCalls, macCalls := 0, 0
	pkgrs := []Packager{
		fakePackager{target: "deb", buildOutput: "out.deb", buildCallCount: &debCalls},
		fakePackager{target: "macpkg", hostErr: errors.New("not darwin"), buildCallCount: &macCalls},
	}

	results := Run(context.Background(), &packproject.Project{}, pkgrs, nil)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Err != nil || results[0].OutputPath != "out.deb" {
		t.Errorf("deb result = %+v, want success with out.deb", results[0])
	}
	if !results[1].Skipped || results[1].Err == nil {
		t.Errorf("macpkg result = %+v, want Skipped with an error", results[1])
	}
	if debCalls != 1 {
		t.Errorf("deb Build called %d times, want 1", debCalls)
	}
	if macCalls != 0 {
		t.Errorf("macpkg Build called %d times, want 0 (should be skipped before Build)", macCalls)
	}
}

func TestRun_PreflightFailureSkipsWithoutBuilding(t *testing.T) {
	buildCalls := 0
	pkgrs := []Packager{
		fakePackager{target: "winexe", preflightErr: errors.New("makensis not found"), buildCallCount: &buildCalls},
	}

	results := Run(context.Background(), &packproject.Project{}, pkgrs, nil)

	if !results[0].Skipped {
		t.Errorf("expected Skipped=true, got %+v", results[0])
	}
	if buildCalls != 0 {
		t.Errorf("Build should not be called after a Preflight failure, got %d calls", buildCalls)
	}
}

func TestRun_BuildFailureIsNotSkipped(t *testing.T) {
	pkgrs := []Packager{
		fakePackager{target: "rpm", buildErr: errors.New("boom")},
	}

	results := Run(context.Background(), &packproject.Project{}, pkgrs, nil)

	if results[0].Skipped {
		t.Errorf("a Build() failure is not a Skipped outcome, got %+v", results[0])
	}
	if results[0].Err == nil {
		t.Errorf("expected Build error to be recorded")
	}
}

func TestRun_BuildsOncePerRegisteredArch(t *testing.T) {
	var arches []string
	pkgrs := []Packager{
		fakePackager{target: TargetMacPkg, buildOutput: "out.pkg", buildArches: &arches},
	}
	proj := &packproject.Project{
		Binaries: []packproject.BinaryEntry{
			{OS: "darwin", Arch: "amd64", Path: "bin/app-amd64"},
			{OS: "darwin", Arch: "arm64", Path: "bin/app-arm64"},
		},
	}

	results := Run(context.Background(), proj, pkgrs, nil)

	if len(results) != 2 {
		t.Fatalf("expected 2 results (one per registered darwin arch), got %d: %+v", len(results), results)
	}
	if arches[0] != "amd64" || arches[1] != "arm64" {
		t.Errorf("Build called with arches %v, want [amd64 arm64] in registration order", arches)
	}
	for _, r := range results {
		if r.Err != nil || r.Skipped {
			t.Errorf("result %+v should have succeeded", r)
		}
	}
}

func TestRun_NoRegisteredArchStillAttemptsOneDefaultBuild(t *testing.T) {
	var arches []string
	pkgrs := []Packager{
		fakePackager{target: TargetDEB, buildErr: errors.New("no linux/amd64 binary registered"), buildArches: &arches},
	}
	proj := &packproject.Project{} // no binaries at all

	results := Run(context.Background(), proj, pkgrs, nil)

	if len(results) != 1 {
		t.Fatalf("expected exactly 1 attempt when no arch is registered, got %d", len(results))
	}
	if len(arches) != 1 || arches[0] != "" {
		t.Errorf("expected one Build call with an empty arch (letting the backend's own default-arch error surface), got %v", arches)
	}
}

func TestRun_MultiArchOneFailureDoesNotStopTheOther(t *testing.T) {
	callCount := 0
	pkgrs := []Packager{
		fakePackager{target: TargetMacPkg, buildErr: errors.New("boom"), buildCallCount: &callCount},
	}
	proj := &packproject.Project{
		Binaries: []packproject.BinaryEntry{
			{OS: "darwin", Arch: "amd64", Path: "bin/app-amd64"},
			{OS: "darwin", Arch: "arm64", Path: "bin/app-arm64"},
		},
	}

	results := Run(context.Background(), proj, pkgrs, nil)

	if callCount != 2 {
		t.Fatalf("expected both arches to be attempted regardless of the fake's shared error, got %d calls", callCount)
	}
	if len(results) != 2 || results[0].Err == nil || results[1].Err == nil {
		t.Errorf("both arch results should carry the Build error, got %+v", results)
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name    string
		results []BuildResult
		want    int
	}{
		{"all succeeded", []BuildResult{{Target: "deb"}, {Target: "rpm"}}, 0},
		{"all failed", []BuildResult{{Target: "deb", Err: errors.New("x")}}, 1},
		{"all skipped", []BuildResult{{Target: "macpkg", Skipped: true, Err: errors.New("x")}}, 1},
		{"partial", []BuildResult{{Target: "deb"}, {Target: "macpkg", Skipped: true, Err: errors.New("x")}}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExitCode(c.results); got != c.want {
				t.Errorf("ExitCode() = %d, want %d", got, c.want)
			}
		})
	}
}
