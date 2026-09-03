package main

// cli.go implements PackMan's headless mode (`packman build`/`validate`/
// `doctor`/`list-targets`). It must never import fyne.io/fyne, directly or
// transitively — main.go dispatches here BEFORE touching Fyne/GLFW at all,
// specifically so this works on a display-less CI runner.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"installerbear/internal/packager"

	_ "installerbear/internal/packager/debrpm" // registers the deb/rpm backends
	_ "installerbear/internal/packager/macpkg" // registers the macpkg backend
	_ "installerbear/internal/packager/winexe" // registers the winexe backend
	_ "installerbear/internal/packager/winmsi" // registers the winmsi backend
	"installerbear/internal/packproject"
)

// cliCommands lists the recognized subcommands; main.go checks os.Args[1]
// against this before deciding whether to launch the GUI at all.
var cliCommands = map[string]func([]string) int{
	"build":        cmdBuild,
	"validate":     cmdValidate,
	"doctor":       cmdDoctor,
	"list-targets": cmdListTargets,
}

// runCLI dispatches a recognized subcommand. Callers must only invoke this
// after confirming args[0] is a known command (see isCLICommand in main.go).
func runCLI(args []string) int {
	cmd, rest := args[0], args[1:]
	fn, ok := cliCommands[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "packman: unknown command %q\n\n", cmd)
		printCLIUsage()
		return 2
	}
	return fn(rest)
}

// isCLICommand reports whether args[0] (if present) names a CLI subcommand,
// so main() can decide to skip Fyne/GLFW entirely.
func isCLICommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	_, ok := cliCommands[args[0]]
	return ok
}

func printCLIUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  packman build    -p packman.yaml [-t deb,rpm,macpkg,winexe,winmsi|linux|mac|windows|all] [-o outdir] [--set KEY=VALUE ...] [--dry-run] [--verbose]
  packman validate -p packman.yaml
  packman doctor    [-t ...]
  packman list-targets
`)
}

// setValues collects repeated --set KEY=VALUE flags.
type setValues []string

func (s *setValues) String() string     { return strings.Join(*s, ",") }
func (s *setValues) Set(v string) error { *s = append(*s, v); return nil }

func cmdBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	projectPath := fs.String("project", "", "path to the project YAML file")
	fs.StringVar(projectPath, "p", "", "path to the project YAML file (shorthand)")
	targetArg := fs.String("target", "all", "comma-separated targets or group alias (linux|mac|windows|all)")
	fs.StringVar(targetArg, "t", "all", "comma-separated targets or group alias (shorthand)")
	outDir := fs.String("outdir", "", "override the project's output.dir")
	fs.StringVar(outDir, "o", "", "override the project's output.dir (shorthand)")
	var sets setValues
	fs.Var(&sets, "set", "override a config field, KEY=VALUE (repeatable)")
	dryRun := fs.Bool("dry-run", false, "resolve and validate everything, but don't build")
	verbose := fs.Bool("verbose", false, "print every backend's progress lines")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *projectPath == "" {
		fmt.Fprintln(os.Stderr, "packman build: -p/--project is required")
		return 2
	}

	proj, err := packproject.Load(*projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "packman build: %v\n", err)
		return 1
	}
	applyOverrides(proj, sets)
	if *outDir != "" {
		proj.Output.Dir = *outDir
	}

	targets, err := expandTargets(*targetArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "packman build: %v\n", err)
		return 2
	}

	pkgrs, err := packager.Resolve(targets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "packman build: %v\n", err)
		return 1
	}

	if *dryRun {
		fmt.Println("dry run: would build", joinTargets(targets), "from", *projectPath)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	results := packager.Run(ctx, proj, pkgrs, func(target packager.Target, line string) {
		if *verbose || strings.HasPrefix(line, "failed") || strings.HasPrefix(line, "skipped") || strings.HasPrefix(line, "done") {
			fmt.Printf("[%s] %s\n", target, line)
		}
	})

	succeeded, failed := packager.Summary(results)
	fmt.Printf("%d succeeded, %d failed/skipped\n", succeeded, failed)
	return packager.ExitCode(results)
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	projectPath := fs.String("project", "", "path to the project YAML file")
	fs.StringVar(projectPath, "p", "", "path to the project YAML file (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *projectPath == "" {
		fmt.Fprintln(os.Stderr, "packman validate: -p/--project is required")
		return 2
	}

	if _, err := packproject.Load(*projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "invalid: %v\n", err)
		return 1
	}
	fmt.Println("ok")
	return 0
}

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	targetArg := fs.String("target", "all", "comma-separated targets or group alias to check")
	fs.StringVar(targetArg, "t", "all", "comma-separated targets or group alias to check (shorthand)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	targets, err := expandTargets(*targetArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "packman doctor: %v\n", err)
		return 2
	}

	anyProblem := false
	for _, t := range targets {
		pkgrs, err := packager.Resolve([]packager.Target{t})
		if err != nil {
			fmt.Printf("%-8s not implemented yet\n", t)
			anyProblem = true
			continue
		}
		p := pkgrs[0]
		if err := p.HostSupported(); err != nil {
			fmt.Printf("%-8s unsupported on this host: %v\n", t, err)
			anyProblem = true
			continue
		}
		if err := p.Preflight(); err != nil {
			fmt.Printf("%-8s missing tool: %v\n", t, err)
			anyProblem = true
			continue
		}
		fmt.Printf("%-8s ready\n", t)
	}
	if anyProblem {
		return 1
	}
	return 0
}

func cmdListTargets(args []string) int {
	registered := make(map[packager.Target]bool)
	for _, t := range packager.Registered() {
		registered[t] = true
	}
	names := append([]string(nil), packproject.AllTargets...)
	sort.Strings(names)
	for _, name := range names {
		status := "not implemented yet"
		if registered[packager.Target(name)] {
			status = "implemented"
		}
		fmt.Printf("%-8s %s\n", name, status)
	}
	return 0
}

// expandTargets turns a comma-separated --target value (concrete names
// and/or a linux|mac|windows|all group alias) into a deduplicated, ordered
// list of packager.Target, mirroring package.sh's existing linux|mac|all
// ergonomics.
func expandTargets(arg string) ([]packager.Target, error) {
	seen := make(map[string]bool)
	var out []packager.Target

	add := func(name string) error {
		if seen[name] {
			return nil
		}
		found := false
		for _, known := range packproject.AllTargets {
			if name == known {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown target %q", name)
		}
		seen[name] = true
		out = append(out, packager.Target(name))
		return nil
	}

	for _, part := range strings.Split(arg, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if group, ok := packproject.TargetGroups[part]; ok {
			for _, name := range group {
				if err := add(name); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := add(part); err != nil {
			return nil, err
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no targets resolved from %q", arg)
	}
	return out, nil
}

func joinTargets(targets []packager.Target) string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = string(t)
	}
	return strings.Join(names, ",")
}

// applyOverrides covers the small set of fields today's package.sh already
// lets you override via shell env vars (VERSION=..., ARCH=...), so scripted
// callers don't need to hand-edit the project YAML for a one-off release.
func applyOverrides(proj *packproject.Project, sets setValues) {
	for _, kv := range sets {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "version":
			proj.Identity.Version = value
		case "publisher":
			proj.Identity.Publisher = value
		case "vendor":
			proj.Identity.Vendor = value
		case "url":
			proj.Identity.URL = value
		case "outputdir":
			proj.Output.Dir = value
		}
	}
}
