// Package winmsi builds a real .msi via wixl (part of GNOME's msitools),
// not Microsoft's own WiX Toolset CLI. That's a deliberate substitution,
// not a preference: WiX v4/v5's own directory-name path validation is
// broken on non-Windows hosts (confirmed against a real install here —
// even a single plain, unremarkable directory name fails with WIX0389
// "not a relative path" — and the WiX team closed the upstream report as
// "not planned", i.e. Windows-only is their accepted position, not a bug
// they intend to fix). wixl is a from-scratch reimplementation that
// actually builds valid .msi files from macOS/Linux, verified against
// msiinfo here, at the cost of only supporting a WiX v3-era XML subset
// (no StandardDirectory sugar, no CustomAction/EXE-based custom actions —
// see wxs_template.go's comment on why that's an acceptable trade for
// this tool's deliberately basic scope).
package winmsi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"installerbear/internal/packager"
	"installerbear/internal/packproject"
)

const defaultArch = "amd64"

func init() {
	packager.Register(packager.TargetWinMSI, New)
}

// New returns the .msi backend.
func New() packager.Packager { return winmsiPackager{} }

type winmsiPackager struct{}

func (winmsiPackager) Target() packager.Target { return packager.TargetWinMSI }

// HostSupported is always nil: wixl genuinely builds real .msi files from
// any host OS (verified), unlike the Microsoft WiX CLI this backend
// deliberately avoids.
func (winmsiPackager) HostSupported() error { return nil }

func (winmsiPackager) Preflight() error {
	if _, err := exec.LookPath("wixl"); err != nil {
		return fmt.Errorf("wixl not found on PATH (install via brew/apt: brew install msitools, or apt install msitools)")
	}
	return nil
}

func (p winmsiPackager) Build(ctx context.Context, proj *packproject.Project, opts packager.BuildOptions, progress packager.ProgressFunc) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	arch := opts.Arch
	if arch == "" {
		arch = defaultArch
	}

	bin, ok := proj.BinaryFor("windows", arch)
	if !ok {
		return "", fmt.Errorf("winmsi: no windows/%s binary registered in project.binaries", arch)
	}

	root, allComponents, err := buildDirTree(proj, bin)
	if err != nil {
		return "", fmt.Errorf("winmsi: %w", err)
	}

	used := map[string]bool{"INSTALLDIR": true, "ProgramFilesFolder": true, "TARGETDIR": true, "ProgramMenuFolder": true}
	for _, c := range allComponents {
		used[c] = true
	}
	data := wxsData{
		AppName:             proj.Identity.Name,
		Manufacturer:        firstNonEmpty(proj.Identity.Publisher, proj.Identity.Vendor),
		Version:             proj.Identity.Version,
		UpgradeGUID:         trimBraces(proj.Windows.UpgradeGUID),
		ProductDescription:  proj.Identity.Description,
		Root:                root,
		AllComponentIDs:     allComponents,
		ProgramMenuDirID:    uniqueID(used, "dir_"+proj.Identity.Name),
		ShortcutComponentID: uniqueID(used, "cmp_shortcut"),
		ShortcutTargetName:  proj.Windows.ExeName,
	}

	stagingDir, err := os.MkdirTemp("", "packman-winmsi-*")
	if err != nil {
		return "", fmt.Errorf("winmsi: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	wxsPath := filepath.Join(stagingDir, "app.wxs")
	wxsFile, err := os.Create(wxsPath)
	if err != nil {
		return "", fmt.Errorf("winmsi: %w", err)
	}
	if err := wxsTemplate.Execute(wxsFile, data); err != nil {
		wxsFile.Close()
		return "", fmt.Errorf("winmsi: rendering .wxs: %w", err)
	}
	wxsFile.Close()

	outDir := proj.Output.Dir
	if opts.OutputDir != "" {
		outDir = opts.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("winmsi: creating output dir: %w", err)
	}
	// arch in the filename matters once more than one Windows arch is in
	// play (see packager.Run's multi-arch loop) — without it, a second
	// arch's build would silently overwrite the first's output file.
	outPath := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s.msi", packager.Slug(proj.Identity.Name), proj.Identity.Version, arch))

	p.emit(progress, "running wixl...")
	if err := packager.RunCommand(ctx, "", func(line string) { p.emit(progress, line) }, "wixl", wxsPath, "-o", outPath, "-a", "x64"); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("winmsi: wixl: %w", err)
	}

	return outPath, nil
}

func (winmsiPackager) emit(progress packager.ProgressFunc, line string) {
	if progress != nil {
		progress(packager.TargetWinMSI, line)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// trimBraces strips the {..} wrapper some GUID sources include (Inno's AppId
// convention among them) — wixl expects a bare GUID for UpgradeCode.
func trimBraces(guid string) string {
	if len(guid) >= 2 && guid[0] == '{' && guid[len(guid)-1] == '}' {
		return guid[1 : len(guid)-1]
	}
	return guid
}
