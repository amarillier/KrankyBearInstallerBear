package winmsi

import "text/template"

// wxsTemplate renders a WiX v3-schema source file covering the "basics": a
// stable UpgradeCode (Product/@Id='*' auto-generates a fresh ProductCode
// per build, which is exactly right — see this tool's plan notes on why
// conflating the two breaks major-upgrade detection), the binary plus every
// payload file/tree under INSTALLDIR, and a Start Menu shortcut. No custom
// actions (e.g. taskkill-before-uninstall, as winexe has): wixl — the
// backend this template targets — doesn't support the EXE-based CustomAction
// pattern that would need, and it's less essential for MSI anyway, since the
// Windows Installer service (unlike a raw NSIS script) already owns
// file/registry/shortcut removal on uninstall without needing to be told to.
//
// v3 schema (xmlns .../2006/wi), not v4/v5's newer StandardDirectory sugar:
// this targets wixl (GNOME msitools), not Microsoft's own wix.exe — see
// build.go's comment for why.
var wxsTemplate = template.Must(template.New("app.wxs").Parse(`<?xml version='1.0'?>
<Wix xmlns='http://schemas.microsoft.com/wix/2006/wi'>
  <Product Name='{{.AppName}}' Manufacturer='{{.Manufacturer}}' Id='*' UpgradeCode='{{.UpgradeGUID}}' Language='1033' Codepage='1252' Version='{{.Version}}'>
    <Package Id='*' Keywords='Installer' Description='{{.ProductDescription}}' Manufacturer='{{.Manufacturer}}' InstallerVersion='200' Languages='1033' Compressed='yes' SummaryCodepage='1252'/>
    <Media Id='1' Cabinet='product.cab' EmbedCab='yes'/>
    <MajorUpgrade DowngradeErrorMessage='A newer version of [ProductName] is already installed.'/>

    <Directory Id='TARGETDIR' Name='SourceDir'>
      <Directory Id='ProgramFilesFolder' Name='PFiles'>
{{template "dir" .Root}}
      </Directory>
      <Directory Id='ProgramMenuFolder'>
        <Directory Id='{{.ProgramMenuDirID}}' Name='{{.AppName}}'>
          <Component Id='{{.ShortcutComponentID}}' Guid='*'>
            <Shortcut Id='StartMenuShortcut' Name='{{.AppName}}' Target='[INSTALLDIR]{{.ShortcutTargetName}}' WorkingDirectory='INSTALLDIR'/>
            <RemoveFolder Id='{{.ProgramMenuDirID}}' On='uninstall'/>
            <RegistryValue Root='HKCU' Key='Software\{{.Manufacturer}}\{{.AppName}}' Name='installed' Type='integer' Value='1' KeyPath='yes'/>
          </Component>
        </Directory>
      </Directory>
    </Directory>

    <Feature Id='MainFeature' Title='{{.AppName}}' Level='1'>
{{range .AllComponentIDs}}      <ComponentRef Id='{{.}}'/>
{{end}}      <ComponentRef Id='{{.ShortcutComponentID}}'/>
    </Feature>
  </Product>
</Wix>
{{define "dir"}}<Directory Id='{{.ID}}' Name='{{.Name}}'>
{{range .Files}}  <Component Id='{{.ComponentID}}' Guid='*'>
    <File Id='{{.ID}}' Name='{{.Name}}' Source='{{.Source}}' KeyPath='yes'/>
  </Component>
{{end}}{{range .Dirs}}{{template "dir" .}}
{{end}}</Directory>
{{end}}`))

// wxsData is the template's render input.
type wxsData struct {
	AppName, Manufacturer, Version, UpgradeGUID, ProductDescription string
	Root                                                            *dirNode
	AllComponentIDs                                                 []string
	ProgramMenuDirID, ShortcutComponentID, ShortcutTargetName       string
}
