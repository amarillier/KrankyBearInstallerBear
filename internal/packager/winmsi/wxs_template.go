package winmsi

import "text/template"

// wxsTemplate renders a WiX v3-schema source file covering the "basics": a
// stable UpgradeCode (Product/@Id='*' auto-generates a fresh ProductCode
// per build, which is exactly right — see this tool's plan notes on why
// conflating the two breaks major-upgrade detection), the binary plus every
// payload file/tree under INSTALLDIR, and a Start Menu shortcut (plus an
// optional Desktop one and an optional "launch it now" finish-page checkbox
// - see InstallExperience below). wixl genuinely does support an EXE-based
// CustomAction (FileKey+ExeCommand, verified empirically against a real
// compiled .msi's CustomAction/ControlEvent tables) — an earlier version of
// this comment claimed otherwise; that was wrong. taskkill-before-uninstall
// (which winexe has) still isn't replicated here, since it's less essential
// for MSI: the Windows Installer service already owns file/registry/
// shortcut removal on uninstall without needing to be told to.
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
{{if not .PerUser}}
    <!-- Without ALLUSERS=1, Windows Installer defaults to a per-user
         context, which mismatches the ProgramFiles64Folder-rooted directory
         tree below: the install silently fails to elevate, rolls back with
         no error dialog (there's no custom UI to show one in), and leaves
         nothing behind - no files, no Programs-and-Features entry. -->
    <Property Id='ALLUSERS' Value='1'/>
{{end}}
{{if .IconFile}}
    <!-- Without this, Windows Installer shows a generic default icon for
         this product in "Apps & Features"/Programs and Features, unlike
         most other installed apps - found in real Windows testing.
         Verified empirically that wixl supports both the Icon table
         entry and ARPPRODUCTICON (real compiled .msi's Icon/Property
         tables both came out correct). Unrelated to the Windows/macOS
         binary's own embedded icon (a property of how that binary itself
         was compiled) - this is purely what Programs and Features shows. -->
    <Icon Id='ProductIcon' SourceFile='{{.IconFile}}'/>
    <Property Id='ARPPRODUCTICON' Value='ProductIcon'/>
{{end}}
{{if .LaunchAfterInstall}}
    <!-- Adds a "Launch <App> now" checkbox to WixUI_Minimal's stock
         ExitDialog (finish page) - the checkbox control itself already
         exists in that dialog, gated on ...CHECKBOXTEXT being set (see
         ExitDialog's own ShowCondition). ...CHECKBOX itself (a *different*
         property - easy to conflate, and a real bug caught in testing:
         the checkbox appeared but wasn't checked by default despite this
         comment's own earlier claim that it would be) is the one that
         actually drives whether the control starts checked - a WiX
         CheckBox control reads its initial state from whether this
         property already equals CheckBoxValue ("1") when the dialog
         first shows, not from anything about ...CHECKBOXTEXT. The
         Fragment after </Product> below wires the actual launch to it. -->
    <Property Id='WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT' Value='Launch {{.AppName}} now'/>
    <Property Id='WIXUI_EXITDIALOGOPTIONALCHECKBOX' Value='1'/>
    <CustomAction Id='LaunchApplication' FileKey='{{.BinaryFileID}}' ExeCommand='' Return='asyncNoWait'/>
{{end -}}
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
{{if .PerUser -}}
      <!-- LocalAppDataFolder is the standard WiX/MSI directory id for
           %LOCALAPPDATA% - writable without elevation, unlike the
           all-users root the else branch below uses, which is why
           InstallScope's current-user variant roots here instead (paired
           with omitting the all-users elevation property above - see
           packproject.WindowsOptions.InstallScope's own doc comment on
           why the two always move together). -->
      <Directory Id='LocalAppDataFolder'>
{{template "dir" .Root}}
      </Directory>
{{else -}}
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
{{end -}}
      <Directory Id='ProgramMenuFolder'>
        <Directory Id='{{.ProgramMenuDirID}}' Name='{{.AppName}}'>
          <Component Id='{{.ShortcutComponentID}}' Guid='*'>
            <Shortcut Id='StartMenuShortcut' Name='{{.AppName}}' Target='[INSTALLDIR]{{.ShortcutTargetName}}' WorkingDirectory='INSTALLDIR'/>
            <RemoveFolder Id='{{.ProgramMenuDirID}}' On='uninstall'/>
            <RegistryValue Root='HKCU' Key='Software\{{.Manufacturer}}\{{.AppName}}' Name='installed' Type='integer' Value='1' KeyPath='yes'/>
          </Component>
{{if .AutostartAtLogin}}
          <!-- Author-time-only, unlike winexe's real end-user checkbox -
               wixl's bundled UI (WixUI_Minimal only) has no Components
               page to offer a real choice here, same reasoning as
               DesktopShortcut above (see InstallExperience's own doc
               comment). A registry-value-only Component (no File) is a
               standard WiX/MSI pattern - this Component's mere presence in
               MainFeature below is the opt-in itself, since there's no UI
               to conditionally select it through. HKCU (not HKLM): only
               ever autostarts for the account that ran the install, not
               every account on the machine. -->
          <Component Id='{{.AutostartComponentID}}' Guid='*'>
            <RegistryValue Root='HKCU' Key='Software\Microsoft\Windows\CurrentVersion\Run' Name='{{.AppName}}' Type='string' Value='[INSTALLDIR]{{.ShortcutTargetName}}' KeyPath='yes'/>
          </Component>
{{end}}
{{range .FileAssociations}}
          <!-- Always unconditional (no author toggle, no end-user
               checkbox) - registering a file type is the whole point of
               setting one. Root='HKCR' regardless of PerUser: Windows'
               own registry virtualization redirects an unprivileged
               process's HKCR writes to HKCU\Software\Classes, so this
               needs no manual scope switching (see
               packproject.FileAssociation's own doc comment). ProgId's
               own Icon/IconIndex attributes are NOT used here - confirmed
               empirically that wixl 0.106 silently ignores them (a
               GObject "no property named Icon" warning, not even a hard
               error) - so DefaultIcon is written by hand as a plain
               RegistryValue instead, the same proven mechanism every
               other registry entry in this template already uses. -->
          <Component Id='{{.ComponentID}}' Guid='*'>
            <ProgId Id='{{.ProgID}}' Description='{{.Description}}'>
              <Extension Id='{{.ExtensionNoDot}}' ContentType='{{.ContentType}}'>
                <Verb Id='open' Command='Open' TargetFile='{{$.BinaryFileID}}' Argument='"%1"'/>
              </Extension>
            </ProgId>
            <RegistryValue Root='HKCR' Key='{{.ProgID}}\DefaultIcon' Value='[INSTALLDIR]{{$.ShortcutTargetName}},0' Type='string' KeyPath='yes'/>
          </Component>
{{end}}
        </Directory>
      </Directory>
{{if .DesktopShortcut}}
      <!-- DesktopFolder is a standard WiX/MSI directory reference (like
           ProgramMenuFolder above) - an author-time choice, not an
           end-user one, unlike the Start Menu shortcut's own checkbox-free
           unconditional presence above (see InstallExperience's own doc
           comment on why this one isn't a real installer-time toggle). -->
      <Directory Id='DesktopFolder'>
        <Component Id='{{.DesktopShortcutComponentID}}' Guid='*'>
          <Shortcut Id='DesktopShortcut' Name='{{.AppName}}' Target='[INSTALLDIR]{{.ShortcutTargetName}}' WorkingDirectory='INSTALLDIR'/>
          <RegistryValue Root='HKCU' Key='Software\{{.Manufacturer}}\{{.AppName}}' Name='desktop_shortcut' Type='integer' Value='1' KeyPath='yes'/>
        </Component>
      </Directory>
{{end}}
    </Directory>

    <Feature Id='MainFeature' Title='{{.AppName}}' Level='1'>
{{range .AllComponentIDs}}      <ComponentRef Id='{{.}}'/>
{{end}}      <ComponentRef Id='{{.ShortcutComponentID}}'/>
{{if .DesktopShortcut}}      <ComponentRef Id='{{.DesktopShortcutComponentID}}'/>
{{end}}{{if .AutostartAtLogin}}      <ComponentRef Id='{{.AutostartComponentID}}'/>
{{end}}{{range .FileAssociations}}      <ComponentRef Id='{{.ComponentID}}'/>
{{end}}    </Feature>
{{if or .HasLicense .LaunchAfterInstall}}
    <!-- wixl's bundled "ui" extension (the "-ext ui" flag in build.go) reimplements
         WiX's stock WixUI_Minimal: Welcome/EULA, Progress, and Exit
         dialogs - verified empirically against a real compiled .msi
         (Dialog/Control/InstallUISequence tables all populated correctly)
         since GNOME's own docs only ever claimed UI support was missing.
         WelcomeEulaDlg.wxs (part of this extension) reads the license
         text from a file it hardcodes as "License.rtf", resolved relative
         to this .wxs file's own directory - build.go writes one there,
         converted from Identity.LicenseFile, whenever this block is used
         (a placeholder when LaunchAfterInstall is on but there's no real
         license: WixUI_Minimal's Welcome/EULA page and ExitDialog's
         Launch-now checkbox are one bundled stock UI, not separable, so
         opting into the checkbox means also getting the license page). -->
    <UIRef Id='WixUI_Minimal'/>
{{end}}  </Product>
{{if .LaunchAfterInstall}}
  <Fragment>
    <!-- Wires the ExitDialog's "Launch now" checkbox (surfaced by setting
         WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT above) to actually launch the
         app: WixUI_Minimal's own ExitDialog already publishes its default
         EndDialog/Return on Finish, so this adds a second, higher-priority
         (lower Order) event on the same Control rather than replacing
         anything - both fire, DoAction first, matching real WiX's own
         WixUI_Minimal + CustomAction idiom for this. Verified empirically:
         the resulting .msi's ControlEvent table has both events. -->
    <UI Id='InstallerBearLaunchUI'>
      <Publish Dialog='ExitDialog' Control='Finish' Event='DoAction' Value='LaunchApplication'
               Condition='WIXUI_EXITDIALOGOPTIONALCHECKBOX = 1 and NOT Installed'/>
    </UI>
  </Fragment>
{{end}}
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
	// LaunchAfterInstall adds a checked-by-default "Launch <App> now"
	// checkbox to the finish page (an end-user choice at install time,
	// opted into here by the project author) - see
	// packproject.InstallExperience's own doc comment on this split.
	// BinaryFileID is the main exe's own <File> Id (from buildDirTree),
	// needed as the LaunchApplication CustomAction's FileKey.
	LaunchAfterInstall bool
	BinaryFileID       string
	// DesktopShortcut adds a second shortcut on the Desktop alongside the
	// Start Menu one this template always creates - an author-time choice,
	// not something the person installing chooses (unlike
	// LaunchAfterInstall above). DesktopShortcutComponentID is only used
	// when this is true.
	DesktopShortcut            bool
	DesktopShortcutComponentID string
	// AutostartAtLogin writes a per-user HKCU Run value at install time -
	// always an author-time-only choice here (same reason as
	// DesktopShortcut: wixl's bundled UI has no Components-page
	// equivalent to offer a real end-user checkbox), unlike winexe's own
	// AutostartAtLogin handling, which is a real, unchecked-by-default
	// Components-page checkbox there. AutostartComponentID is only used
	// when this is true.
	AutostartAtLogin     bool
	AutostartComponentID string
	// IconFile is Identity.Icons.ICO's resolved path - shown in "Apps &
	// Features"/Programs and Features via ARPPRODUCTICON when set. Not
	// used for anything else here; the installed binary's own icon (if
	// any) comes from however that binary itself was compiled.
	IconFile string
	// PerUser mirrors packproject.WindowsOptions.InstallScope ==
	// InstallScopeCurrentUser - an author-time-only choice, same reasoning
	// as DesktopShortcut/AutostartAtLogin above (wixl's bundled UI has no
	// equivalent InstallScopeDlg/WixUI_Advanced to offer this as a real
	// end-user runtime pick - confirmed by checking the installed
	// msitools build's own bundled ext/ui directory). Omits ALLUSERS
	// entirely (not "0" - not a valid MSI value) and roots the install
	// tree at LocalAppDataFolder instead of ProgramFiles64Folder when
	// true - the two always move together, since a per-user (non-
	// elevated) context can't write to Program Files at all.
	PerUser bool
	// FileAssociations mirrors packproject.Project.FileAssociations -
	// always unconditional, matching winexe's own FileAssociations
	// handling (see nsiData's own doc comment on why this isn't gated
	// behind a toggle the way DesktopShortcut/AutostartAtLogin are).
	FileAssociations []wxsFileAssociation
}

// wxsFileAssociation is one packproject.FileAssociation translated into
// WiX/wixl terms. ProgID/ContentType/ComponentID are all computed once in
// build.go, not stored in the schema itself - implementation details, not
// something a project author needs to see or name.
type wxsFileAssociation struct {
	// ExtensionNoDot is the extension WITHOUT its leading dot - WiX's own
	// <Extension Id='...'> attribute is the bare extension (wixl reads it
	// that way; a leading dot there breaks the generated .myp registry
	// key path), unlike ProgID/DefaultIcon below which do want the dot
	// baked into the registry key text itself.
	ExtensionNoDot string
	ProgID         string
	Description    string
	ContentType    string
	ComponentID    string
}
