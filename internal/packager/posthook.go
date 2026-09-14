package packager

import (
	"context"
	"fmt"
	"os"
	"strings"

	"installerbear/internal/packproject"
)

// postBuildHookTarget labels the hook's own progress lines (e.g.
// "[post_build_hook] ...") — a synthetic value, never one of
// packproject's real target names, so it can't collide with or be
// mistaken for an actual build target's own output in a build log.
const postBuildHookTarget Target = "post_build_hook"

// runPostBuildHook runs proj.Output.PostBuildHook once, after every
// requested target/arch in results has finished building — see that
// field's own doc comment for the full design. A no-op when the hook is
// blank or ctx is already canceled.
func runPostBuildHook(ctx context.Context, proj *packproject.Project, results []BuildResult, onEvent ProgressFunc) {
	hook := strings.TrimSpace(proj.Output.PostBuildHook)
	if hook == "" {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	if _, failed := Summary(results); failed > 0 {
		emit(onEvent, postBuildHookTarget, fmt.Sprintf("skipped: %d target(s) failed or were skipped", failed))
		return
	}

	scriptPath, cleanup, err := writePostBuildHookScript(hook)
	if err != nil {
		emit(onEvent, postBuildHookTarget, fmt.Sprintf("failed to prepare script: %v", err))
		return
	}
	defer cleanup()

	env := postBuildHookEnv(proj, results)
	emit(onEvent, postBuildHookTarget, "running post_build_hook...")
	// Runs the script file directly (not "/bin/sh <path>") so a custom
	// shebang (#!/usr/bin/env python3, say) is honored, not just POSIX
	// sh - the same convention a real .deb/.rpm maintainer script already
	// gets from dpkg/rpm itself.
	if err := RunCommandEnv(ctx, "", env, func(line string) { emit(onEvent, postBuildHookTarget, line) }, scriptPath); err != nil {
		emit(onEvent, postBuildHookTarget, fmt.Sprintf("failed: %v", err))
		return
	}
	emit(onEvent, postBuildHookTarget, "done")
}

// postBuildHookEnv builds the INSTALLERBEAR_* environment variables handed
// to the hook script: identity basics, plus one variable per successfully
// built artifact. A target that only ever builds one arch (or none at
// all, e.g. macpkg with a single darwin binary) gets the bare
// INSTALLERBEAR_OUTPUT_<TARGET> name; a target built for more than one
// arch (two macOS arches, say) instead gets
// INSTALLERBEAR_OUTPUT_<TARGET>_<ARCH> per arch, since one bare name
// couldn't hold more than one path. Skipped/failed results contribute
// nothing — callers only ever reach this after confirming zero of those
// (see runPostBuildHook), but the guard stays here too since this is
// also independently unit-tested.
func postBuildHookEnv(proj *packproject.Project, results []BuildResult) []string {
	env := []string{
		"INSTALLERBEAR_APP_NAME=" + proj.Identity.Name,
		"INSTALLERBEAR_VERSION=" + proj.Identity.Version,
		"INSTALLERBEAR_OUTPUT_DIR=" + proj.Output.Dir,
	}

	archCount := make(map[Target]int)
	for _, r := range results {
		if r.Err == nil && !r.Skipped {
			archCount[r.Target]++
		}
	}

	var artifacts []string
	for _, r := range results {
		if r.Err != nil || r.Skipped {
			continue
		}
		key := strings.ToUpper(string(r.Target))
		if archCount[r.Target] > 1 && r.Arch != "" {
			key += "_" + strings.ToUpper(r.Arch)
		}
		env = append(env, "INSTALLERBEAR_OUTPUT_"+key+"="+r.OutputPath)
		artifacts = append(artifacts, r.OutputPath)
	}
	env = append(env, "INSTALLERBEAR_ARTIFACTS="+strings.Join(artifacts, " "))

	return env
}

// writePostBuildHookScript writes content to a new temp script file,
// prepending a POSIX shebang + "set -e" when content doesn't already
// start with its own "#!" line (matching a project author's freedom to
// pick a different interpreter). Deliberately a small local duplicate of
// internal/packager/debrpm's own writeHookScript rather than a shared
// export: debrpm already imports this package, so the reverse dependency
// isn't available, and the two are a handful of lines each.
func writePostBuildHookScript(content string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "installerbear-posthook-*.sh")
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	if !strings.HasPrefix(content, "#!") {
		content = "#!/bin/sh\nset -e\n" + content
	}
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Chmod(0o755); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}

	name := f.Name()
	return name, func() { os.Remove(name) }, nil
}
