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
    <!-- Without ALLUSERS=1, Windows Installer defaults to a per-user
         context, which mismatches the ProgramFiles64Folder-rooted directory
         tree below: the install silently fails to elevate, rolls back with
         no error dialog (there's no custom UI to show one in), and leaves
         nothing behind - no files, no Programs-and-Features entry. -->
    <Property Id='ALLUSERS' Value='1'/>
{{if .HasLicense}}
    <!-- Blocks a fully unattended (/qn or /qb) install unless the caller
         explicitly passes ACCEPTEULA=1 - without this, silent automation
         could install without ever agreeing to the license, since Windows
         Installer skips ALL dialogs (including WelcomeEulaDlg below) in
         that mode. Installed (already-installed product, e.g. a repair)
         and UILevel > 3 (a real interactive session, where the checkbox in
         WelcomeEulaDlg itself gates the Install button) both bypass it. -->
    <Condition Message='This installer requires accepting the license. Run it interactively, or pass ACCEPTEULA=1 for a silent/unattended install.'>Installed OR ACCEPTEULA="1" OR UILevel &gt; 3</Condition>
{{end}}
    <Directory Id='TARGETDIR' Name='SourceDir'>
      <!-- ProgramFilesFolder ALWAYS resolves to "Program Files (x86)" on
           64-bit Windows, regardless of this package's own bitness (wixl is
           always invoked with -a x64, see build.go) - confirmed the hard
           way via a real msiexec /l*v log showing INSTALLDIR resolve to
           (x86) despite every install action reporting success. Getting
           the real 64-bit "Program Files" requires the separate, dedicated
           ProgramFiles64Folder standard directory id instead. -->
      <Directory Id='ProgramFiles64Folder' Name='PFiles'>
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
{{if .HasLicense}}
    <!-- wixl's bundled "ui" extension (the "-ext ui" flag in build.go) reimplements
         WiX's stock WixUI_Minimal: Welcome/EULA, Progress, and Exit
         dialogs - verified empirically against a real compiled .msi
         (Dialog/Control/InstallUISequence tables all populated correctly)
         since GNOME's own docs only ever claimed UI support was missing.
         WelcomeEulaDlg.wxs (part of this extension) reads the license
         text from a file it hardcodes as "License.rtf", resolved relative
         to this .wxs file's own directory - build.go writes one there,
         converted from Identity.LicenseFile, whenever this block is used. -->
    <UIRef Id='WixUI_Minimal'/>
{{end}}  </Product>
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
	// HasLicense gates the WixUI_Minimal license/EULA dialog and its
	// matching ACCEPTEULA launch condition. build.go sets this alongside
	// writing the License.rtf file the dialog needs.
	HasLicense bool
}
