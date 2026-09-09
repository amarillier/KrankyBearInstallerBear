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
	"strings"

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

	used := map[string]bool{"INSTALLDIR": true, "ProgramFiles64Folder": true, "TARGETDIR": true, "ProgramMenuFolder": true}
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

	if proj.Identity.LicenseFile != "" {
		licenseText, err := os.ReadFile(proj.ResolvePath(proj.Identity.LicenseFile))
		if err != nil {
			return "", fmt.Errorf("winmsi: reading license file: %w", err)
		}
		// WelcomeEulaDlg.wxs (part of wixl's bundled "ui" extension, see
		// below) hardcodes reading its license text from a file literally
		// named License.rtf, resolved relative to app.wxs's own directory
		// (confirmed empirically - not documented anywhere) - so it must
		// live in stagingDir alongside app.wxs, under exactly this name.
		rtfPath := filepath.Join(stagingDir, "License.rtf")
		if err := os.WriteFile(rtfPath, []byte(licenseToRTF(string(licenseText))), 0o644); err != nil {
			return "", fmt.Errorf("winmsi: writing License.rtf: %w", err)
		}
		data.HasLicense = true
	}

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
	outPath := filepath.Join(outDir, fmt.Sprintf("%sSetup_%s_%s.msi", strings.ReplaceAll(proj.Identity.Name, " ", ""), proj.Identity.Version, arch))

	args := []string{wxsPath, "-o", outPath, "-a", "x64"}
	if data.HasLicense {
		// Pulls in wixl's bundled WixUI_Minimal-equivalent (Welcome/EULA,
		// Progress, Exit dialogs) that <UIRef Id='WixUI_Minimal'/> in
		// wxs_template.go references.
		args = append(args, "--ext", "ui")
	}
	p.emit(progress, "running wixl...")
	if err := packager.RunCommand(ctx, "", func(line string) { p.emit(progress, line) }, "wixl", args...); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("winmsi: wixl: %w", err)
	}

	return outPath, nil
}

// licenseToRTF wraps plain text in the minimal valid RTF that
// WelcomeEulaDlg's ScrollableText control (part of wixl's "ui" extension)
// expects - verified against a real compiled .msi that this renders
// correctly. Escapes RTF's control characters and re-encodes anything
// outside ASCII as a \uN? sequence, RTF's own unicode-escape form, since
// license text is otherwise usually just an author's name or similar.
func licenseToRTF(text string) string {
	var b strings.Builder
	b.WriteString(`{\rtf1\ansi\deff0`)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		b.WriteByte('\n')
		for _, r := range line {
			switch {
			case r == '\\' || r == '{' || r == '}':
				b.WriteByte('\\')
				b.WriteRune(r)
			case r < 128:
				b.WriteRune(r)
			default:
				fmt.Fprintf(&b, `\u%d?`, r)
			}
		}
		b.WriteString(`\par`)
	}
	b.WriteString("\n}")
	return b.String()
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
