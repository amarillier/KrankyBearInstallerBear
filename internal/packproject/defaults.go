package packproject

import "fmt"

// Defaults fills in every field that has a sensible OS-convention default when
// left blank in the loaded YAML. It mutates p in place.
func (p *Project) Defaults() {
	if p.Windows.InstallScope == "" {
		p.Windows.InstallScope = InstallScopeAllUsers
	}
	if p.Install.Windows == "" {
		// $PROGRAMFILES64/$LOCALAPPDATA are both NSIS runtime variables,
		// resolved by makensis itself at install time — NOT Inno Setup's
		// "{autopf}" constant, which NSIS doesn't understand and would
		// bake into the script as a literal, unresolved path. Which one is
		// the right default depends on InstallScope (defaulted just
		// above, so it's always set by this point): a current-user
		// install can't write to Program Files without elevation, so it
		// needs its own per-user-writable default instead.
		if p.Windows.InstallScope == InstallScopeCurrentUser {
			p.Install.Windows = fmt.Sprintf(`$LOCALAPPDATA\%s`, p.Identity.Name)
		} else {
			p.Install.Windows = fmt.Sprintf(`$PROGRAMFILES64\%s`, p.Identity.Name)
		}
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
