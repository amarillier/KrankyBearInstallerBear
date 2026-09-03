package packager

import "fmt"

// Factory constructs one target's Packager. Registered by each backend
// package's own init-time call to Register — main.go/cli.go only ever
// import the backend packages for their side effect, never their types.
type Factory func() Packager

var registry = map[Target]Factory{}

// Register adds a backend factory under its target name. Backend packages
// (debrpm, macpkg, winexe, winmsi) call this from an init() func.
func Register(t Target, f Factory) { registry[t] = f }

// Resolve builds the Packager for each requested target, in the order given.
// An unregistered target (a backend not wired in yet, e.g. macpkg before
// step 2 of the build order) is reported as an error rather than silently
// skipped, since that's a build-time gap the caller should know about
// immediately — unlike HostSupported/Preflight, which are legitimate
// per-environment reasons to skip.
func Resolve(targets []Target) ([]Packager, error) {
	pkgrs := make([]Packager, 0, len(targets))
	for _, t := range targets {
		factory, ok := registry[t]
		if !ok {
			return nil, fmt.Errorf("target %q has no backend registered (not implemented yet)", t)
		}
		pkgrs = append(pkgrs, factory())
	}
	return pkgrs, nil
}

// Registered reports which targets currently have a backend wired in.
func Registered() []Target {
	out := make([]Target, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	return out
}
