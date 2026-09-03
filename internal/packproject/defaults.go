package packproject

import "fmt"

// Defaults fills in every field that has a sensible OS-convention default when
// left blank in the loaded YAML. It mutates p in place.
func (p *Project) Defaults() {
	if p.Install.Windows == "" {
		p.Install.Windows = fmt.Sprintf(`{autopf}\%s`, p.Identity.Name)
	}
	if p.Install.MacOS == "" {
		p.Install.MacOS = fmt.Sprintf("/Applications/%s.app", p.Identity.Name)
	}
	if p.Install.Linux == "" {
		p.Install.Linux = fmt.Sprintf("/opt/%s", p.Identity.Name)
	}
	if p.Output.Dir == "" {
		p.Output.Dir = "./installers"
	}
	if p.Output.FilenameTemplate == "" {
		p.Output.FilenameTemplate = "{{.Identity.Name}}_{{.Identity.Version}}_{{.Arch}}"
	}
	if p.Windows.ExeName == "" {
		p.Windows.ExeName = p.Identity.Name + ".exe"
	}
	if p.MacOS.BundleExecutable == "" {
		p.MacOS.BundleExecutable = p.Identity.Name
	}
	if len(p.Targets) == 0 {
		p.Targets = append([]string(nil), AllTargets...)
	}
}
