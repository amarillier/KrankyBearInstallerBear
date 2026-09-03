package packproject

import "path/filepath"

// ResolvePath joins a path from the project file (a Payload.Source,
// Identity.LicenseFile, an icon, ...) against BaseDir, unless it's already
// absolute. Every packager backend must go through this rather than using a
// config path as-is, so relative paths behave the same regardless of the
// process's own working directory.
func (p *Project) ResolvePath(rel string) string {
	if rel == "" || filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(p.BaseDir, rel)
}

// BinaryFor returns the configured binary for a given OS/arch pair, or false
// if none is registered.
func (p *Project) BinaryFor(os, arch string) (BinaryEntry, bool) {
	for _, b := range p.Binaries {
		if b.OS == os && b.Arch == arch {
			return b, true
		}
	}
	return BinaryEntry{}, false
}

// ArchesFor returns the distinct architectures with a registered binary for
// the given OS, in the order they first appear in Binaries. Drives
// multi-arch builds: a target that cares about this OS builds once per
// arch returned here — see internal/packager.Run.
func (p *Project) ArchesFor(os string) []string {
	var arches []string
	seen := make(map[string]bool)
	for _, b := range p.Binaries {
		if b.OS == os && !seen[b.Arch] {
			seen[b.Arch] = true
			arches = append(arches, b.Arch)
		}
	}
	return arches
}
