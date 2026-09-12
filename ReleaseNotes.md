Release notes
KrankyBear InstallerBear: cross-platform GUI + CLI for packaging pre-built binaries
into native Windows, macOS, and Linux installers, from one shared project file.

Future ideas, roughly in the order we plan to tackle them (batched into
small, shippable, testable chunks rather than one big push - reassessed
2026-09-09):

Next up:

Then (bigger; tackle once the above is proven):
- Install-experience options, part 2 - Launch-after-install, a Windows-only
  Desktop shortcut toggle, a Release Notes viewer, and a Windows-only
  Run-at-startup (autostart) toggle all shipped in 0.3.0 (see below); still
  open:
  - Real Linux .desktop-file generation (autostart via an XDG autostart
    entry, and a proper desktop-icon equivalent to Windows' Desktop
    shortcut) - Linux's LinuxOptions.DesktopCategories/DesktopComment
    fields already exist in the schema but nothing currently generates a
    .desktop file from them at all
  - File associations and custom installer wizard pages
  - Once these exist: [Registry]/[Icons]/[Tasks]/[Run]/[UninstallRun]/
    [UninstallDelete] importing for "Import Existing Config" - the
    importer already counts and reports these lines as skipped rather than
    silently dropping them, ready to wire up once there's something to
    import them into

Lower priority for now (confirmed with Allan 2026-09-09 - no certs yet for
code signing, and choco/brew were just alternative-packaging thoughts he'd
consider later, not a pressing need):
- code signing for NSIS/MSI/pkg/deb/rpm outputs
- choco and brew, with notes on self hosting use and also notes on
  submission and approval for real choco and brew based hosting

Smaller, uncategorized:
- Improve "Show/Hide All Windows" to remember exactly which secondary windows
  were open, instead of the current blanket a.Driver().AllWindows() approach
  (which can re-show a window closed earlier in the session)
Maybe later considerations, defer for now, lower value
  - maybe some day i18n language support
  - maybe some day a macOS .dmg target, alongside .pkg (not instead of -
    Allan's fine with .pkg either way, this is a "someday maybe, maybe
    never" thought, not a real ask). Worth noting if it ever comes up:
    a .dmg is a different distribution model entirely, not a script-driven
    installer like .pkg - just a mounted disk image with the .app bundle
    and an /Applications symlink, drag-to-install, no pkgbuild/postinstall
    scripts involved at all (hdiutil create is the whole mechanism), so it
    could actually be a lighter lift than .pkg was, not a harder one

Version 0.3.0 - September 12, 2026
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- Payload tab: every cell is now directly editable in place - type into
  Source/Dest/OS filter or toggle Recursive right in the table, no dialog
  round-trip needed for a quick tweak. Add File/Add Folder/Edit.../Remove
  still go through a dialog (adding a row or Browse-picking a path both
  need one). widget.Table recycles a fixed pool of cell objects across
  every row as the user scrolls, so this needed care to avoid a real class
  of bug: each cell's OnChanged is reset before repopulating it for a
  (possibly different) row, so a recycled cell can never fire a stale
  callback bound to whatever row it used to represent - covered by a
  dedicated regression test
- Identity tab: the Linux (.png) icon field now shows a small live
  thumbnail next to it, updating as you type a path or use Browse. No
  preview for .ico/.icns - Go has no standard decoder for either format,
  and this project would rather stay lean than pull in a third-party
  dependency per format for a small benefit
- New install-experience options (Windows only for now), a new
  InstallExperience project section with two toggles on the Identity
  tab's Windows section:
  - Launch after install - a real, checked-by-default "Launch <App> now"
    checkbox on both Setup.exe's and .msi's finish page. The end user
    still makes the actual call at install time; the author only opts the
    feature in. NSIS gets this via MUI_FINISHPAGE_RUN; wixl via a
    CustomAction (FileKey+ExeCommand) wired to WixUI_Minimal's ExitDialog
    - verified empirically against a real compiled .msi's CustomAction/
    ControlEvent tables, since an old code comment claiming wixl doesn't
    support EXE-based CustomActions turned out to be wrong (now corrected
    - same story as the macpkg Info.plist assumption earlier this
    version). Opting this in without a real license set still pulls in
    WixUI_Minimal's Welcome/EULA page (a placeholder license is written
    automatically) - the two are one bundled stock UI in wixl's shipped
    extension, not separable
  - Desktop shortcut - unlike Launch after install, this is an
    author-time-only choice, not an end-user one: wixl's bundled UI
    extension only ships WixUI_Minimal, not the fuller WixUI_FeatureTree/
    Mondo variants a real interactive "create a desktop icon?" checkbox
    would need, so building that would mean hand-authoring a custom MSI
    dialog - out of proportion for this. Both Setup.exe and .msi also now
    always create a Start Menu shortcut unconditionally (winexe had none
    at all before this)
  - Both are no-ops on macpkg/debrpm, which log a clear progress note
    explaining why rather than silently ignoring either setting - no
    installer-time "launch it now" convention exists on macOS/Linux, and
    auto-launching a GUI app from a postinstall script could break a
    headless/CI install; macOS has no "desktop icon" concept distinct
    from /Applications, and real Linux .desktop-file generation remains
    its own separate, bigger, already-tracked future item
- Fixed the Launch-after-install checkbox on .msi appearing unchecked
  despite being documented as checked-by-default - found in real Windows
  testing the same day it shipped. Root cause: only
  WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT (the checkbox's label) was ever
  set; the separate property that actually drives its checked state,
  WIXUI_EXITDIALOGOPTIONALCHECKBOX, was never set to 1. Easy to conflate
  the two - fixed and verified against a real compiled .msi's Property
  table
- New Validate check: Windows.ExeName must match the filename of every
  registered "windows" binary - also found via real Windows testing, this
  time on winexe: Setup.exe offered to launch (checked by default, working
  as intended there), but silently did nothing on Finish. Root cause was a
  project config mistake, not a code bug: exe_name was set to the
  installer's own output filename (e.g. "AppSetup.exe") rather than the
  actual app binary's filename that NSIS/MSI install unchanged - an easy
  mix-up given how similar the two names look. The same wrong value also
  silently breaks the uninstaller's taskkill and every Start Menu/Desktop
  shortcut on both backends, not just launch-after-install, so this is
  now caught at validate/build time with a clear message instead of
  silently doing nothing at install time
- Setup.exe now answers -?/-help/-h/--help/(and Windows-native /?//help):
  prints a usage message (the /S silent switch, /D=<path>, and whether a
  license page will show) and exits, instead of silently launching the
  normal wizard the way any other unrecognized NSIS installer argument
  would. Allan's own stated motivation, worth keeping verbatim: "any time
  I need silent install strings for installations I try -? or -help
  first... then curse the packager and start hunting repo/home page
  docs" - msiexec already answers /? unprompted on the .msi side, this
  closes the same gap on the winexe side
- Setup.exe now registers with Windows' "Apps & Features"/Programs and
  Features - found in real Windows testing: unlike a real .msi (always
  tracked by the Windows Installer service itself), NSIS never does this
  on its own, so Setup.exe silently never showed up there, nor in
  Get-Package, regardless of any InstallExperience setting. Every
  properly-built NSIS installer has to write the standard Uninstall
  registry keys by hand (DisplayName, DisplayVersion, Publisher,
  UninstallString, DisplayIcon, InstallLocation, EstimatedSize, NoModify/
  NoRepair), removed again on uninstall - unconditional, not gated behind
  any toggle
- .msi now shows a real icon in "Apps & Features"/Programs and Features
  (via ARPPRODUCTICON) when Icons.ICO is set, instead of Windows' generic
  default - also found in real Windows testing. Confirmed wixl supports
  this by compiling a real test .msi and checking its Icon/Property
  tables came out correct before wiring it in for real
- Desktop shortcut is now a real, checked-by-default end-user checkbox on
  Setup.exe (a Components page + a separate optional Section, with the
  main Section marked SectionIn RO so it can't be unchecked) - Allan asked
  for this after testing, explicitly fine with leaving it author-only if
  not feasible. It's genuinely doable on winexe/NSIS; .msi/wixl still
  can't (no fuller WixUI_FeatureTree/Mondo-style dialog available), so
  Desktop shortcut is now a deliberate asymmetry between the two Windows
  backends rather than a shared toggle: a real end-user choice on
  Setup.exe, an author-time-only one on .msi
- New Release Notes viewer, in the app itself: Help menu and tray both gained
  a "Release Notes" item that opens a real window rendering the installed
  ReleaseNotes.md/.txt (also checking the all-lowercase releasenotes.md/.txt
  spellings, since Linux's filesystem is case-sensitive) beside the running
  executable's own directory - the same place every backend already installs
  it as a Payload entry. Deliberately reads from disk at runtime rather than
  embedding the text into the binary at compile time: Allan's own call, since
  a real project's release notes can grow large over its life (one of his
  other projects is already close to 400KB) and an embed would bake that size
  into every build permanently, whether or not the window's ever opened.
  Rendered through Fyne's own built-in widget.NewRichTextFromMarkdown - no
  new dependency - so a project that writes real Markdown (headers, bold,
  lists, links) gets it rendered properly, while a plain ReleaseNotes.txt
  (today's convention, and what every existing project config here still
  ships) still reads fine as plain paragraphs. A standalone/portable binary
  copied without its accompanying files is treated as a real, expected case,
  not a bug: the window reports "Release Notes could not be found" with a
  link to the project's GitHub page, rather than silently showing nothing or
  shelling out to the OS's default text viewer/editor (deliberately avoided -
  Allan's own words, "I just find that really ugly")
- Fixed the Release Notes viewer freezing the whole app (a spinning-wait-
  cursor on macOS) for several seconds when opened on a project with a large
  release notes file - found via real testing on a different project whose
  ReleaseNotes.md is already close to 400KB. Root cause: reading the file and
  building the Markdown widget both ran synchronously before the window ever
  appeared, blocking the main goroutine the whole time. Fixed by opening the
  window immediately with a small loading indicator, doing the read/parse in
  a background goroutine, and swapping the real content in via fyne.Do once
  ready - the window is responsive right away, and text appears as soon as
  it's ready rather than after a multi-second freeze. Imperceptible on most
  projects' much smaller release notes; only matters once one grows large
- New install-experience option: Run at startup (autostart), Windows only,
  following the exact same real-checkbox-on-Setup.exe/author-time-only-on-
  .msi split as Desktop shortcut (wixl's bundled UI still can't offer a real
  Components-page choice - same limitation, same reasoning). Unlike Desktop
  shortcut, unchecked by default on Setup.exe's Components page (a
  NSIS `Section /o`): opting a user into launching at every login is a
  bigger behavioral change to spring on them by surprise than an extra
  shortcut is, so this asks explicitly rather than defaulting on. Writes/
  removes a per-user HKCU "...\CurrentVersion\Run" value (never HKLM - only
  ever opts in the account that ran the install, not every account on the
  machine) on Setup.exe; on .msi it's a plain author-time registry-value
  Component (no File, a standard WiX/MSI pattern) whose mere presence in the
  one Feature is itself the opt-in, since there's no Components-page UI to
  gate it behind. No-op with a clear progress note on macpkg/debrpm, same as
  every other Windows-only InstallExperience setting.
- New "File > New Sample Project..." menu item (also in the tray menu):
  writes a real, fully-featured example installerbear.yaml to a location
  you pick, then opens it as the current project - Allan's own idea, for
  anyone who's never seen a project file before and would rather start
  from a real working example than a blank one. Prefers this app's own
  bundled sample (a genuine copy of the exact config that builds
  KrankyBear InstallerBear's own installers, shipped as a Payload entry
  next to the installed binary - see sample-installerbear.yaml); falls
  back to an in-code generated sample (a plain packproject.Project value,
  not a hand-written YAML string, so a future schema/field rename can't
  silently leave it stale) when the bundled file isn't found, e.g. running
  via `go run .` before packaging, or a portable build without its Payload
  files attached. The generated fallback mints a fresh Windows UpgradeGUID
  on every call rather than a fixed placeholder, so saving several sample
  projects in a row never leaves them with colliding UpgradeCodes.
- This project's own ReleaseNotes.txt is retired - Allan switched to
  ReleaseNotes.md ("much nicer to read"), which the in-app Release Notes
  viewer above already preferred first anyway. The "New Project" smart-scan
  (projectscan) now also checks .md before .txt when prefilling from an
  existing folder's release notes, matching the new convention; .txt-based
  projects are still found just as well, only the preference order changed.

Version 0.2.0 - September 11, 2026
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
- New Project smart defaults: pick a folder and best-effort-fill Publisher,
  URL, License file, and icons from its git remote/identity, LICENSE file,
  and assets/images icons
- Binaries and Payload tabs both gained a "Scan folder..." action to
  auto-detect binaries by filename convention and propose payload entries
  from a directory (with a review step before anything is added)
- Build tab's Start button is now a traffic light: green (ready), orange
  (building), green/red (last build's result)
- Toolbar buttons for New/Open/Save/Save As, above the tabs
- Tray menu now duplicates New/Open/Save/Save As from the File menu, has a
  hover tooltip, and gained "Show/Hide All Windows" (also added to the main
  View menu) to show or hide the whole window stack together
- Build log text is now fully legible in both Light and Dark theme
- CLI: `-help`/`-?`/`-h`/`--help` print full usage and exit
- Renamed the default project filename and CLI-facing text from
  packman.yaml/"packman" to installerbear.yaml/"installerbear" (Open still
  works with any filename via the file picker; nothing else changed)
- New "Import Existing Config..." File menu action: best-effort-imports an
  existing Inno .iss and/or KrankyBear-template build-config.sh/package.sh
  into the current project (Identity, Windows Upgrade GUID/exe name,
  bundle ID from an existing Info.plist/Info-plist.txt, Linux desktop
  metadata, and Payload entries with real dest remapping and Excludes
  carried over) - a review step shows current -> proposed for every field,
  and nothing is applied until confirmed. Preserves the original Inno AppId
  as the Windows Upgrade GUID exactly, which matters for real: reusing a
  fresh GUID instead would break in-place upgrades for anyone who already
  has the app from the old Inno-built installer. Payload content is only
  restricted to Windows when it's genuinely Windows-only (the Mesa3D
  fallback); everything else (ReleaseNotes.txt, assets, ...) imports
  unrestricted, matching how package.sh already bundles it into every
  platform
- New Project smart defaults extended: also best-effort-fills Name/Version/
  Description from ReleaseNotes.txt, falls back to a URL found in
  help.go/about.go/update.go source when there's no git remote, matches the
  Linux icon to the same basename as the Windows/macOS icon instead of
  picking the first PNG alphabetically, and auto-scans a conventional bin/
  folder to fill Binaries and derive the Windows exe name/macOS bundle
  executable
- PayloadEntry gained an Excludes field (glob patterns skipped on a
  recursive copy, honored by every packaging backend) to preserve Inno's
  own [Files] Excludes behavior
- Fixed an inconsistency across backends in what a non-recursive
  PayloadEntry's Dest meant (now consistently "a destination directory"
  everywhere); Validate now also catches a Recursive flag that doesn't
  match whether Source is actually a file or a directory, before it can
  reach a backend and fail with a confusing error (or, worse, silently
  build a wrong layout)
- Fixed Validate flagging several differently-named files that legitimately
  share one Dest directory (e.g. three separate mesa-fallback files) as
  duplicate-destination collisions - hit while migrating this very
  project. The collision key now also considers the installed filename
  (Dest + Source's basename) for a non-recursive entry, not Dest alone;
  a Recursive entry's whole tree is still the installed extent, so those
  still collide on Dest by itself
- Payload tab: columns are now resizable (drag a header boundary) and
  ellipsize long paths instead of overflowing into the next column; Add
  File/Add Folder no longer leaves Dest blank when Source is filled in
  (several blank-Dest entries used to silently collide as duplicate
  destinations, only surfacing as a confusing error at build time)
- Binaries tab "Scan folder..." review now distinguishes "already set,
  kept" and "multiple matches, skipped" from a genuine "no match" (all
  three used to be reported identically as "no match")
- Build tab: optional "Remove older-version installers from the output
  folder after a successful build" (off by default; always shows the exact
  file list and asks before deleting anything), plus Copy Log and Save
  Log... buttons
- New Project and Open Project now ask before discarding unsaved changes
  (compares the project's current YAML encoding against its last
  save/load, so this can't be fooled by a field's SetText firing its own
  OnChanged during a routine refresh)
- New Identity "License" field (a short identifier like "GPL v3"/"MIT",
  distinct from the existing License file path) - wired into Linux
  .deb/.rpm package metadata, and "Import Existing Config" now imports
  KB_LICENSE_DEFAULT into it instead of just reporting it as skipped
- New `installerbear import -source <dir> -p installerbear.yaml` CLI
  command: the scriptable equivalent of "Import Existing Config...", for
  migrating a whole set of sibling projects by script instead of clicking
  through the GUI once per project. Only fills already-set fields with
  -overwrite (default: blanks only, since there's no human to click a
  checkbox), skips a Payload entry whose Source is already present (safe to
  re-run against an already-migrated project), and supports -dry-run
- Fixed three real Windows-install bugs found via hands-on testing on
  actual Windows hardware, none of which any existing test had caught:
  (1) the default Windows install path used Inno Setup's "{autopf}"
  placeholder, which NSIS doesn't understand and wrote into the script as
  a literal, unresolved string; (2) the .msi never set ALLUSERS, so
  Windows Installer defaulted to a per-user context that silently failed
  to elevate against the Program-Files-rooted directory tree, leaving
  neither files nor a Programs-and-Features entry behind, with no error
  shown; (3) even after that fix, the .msi installed into "Program Files
  (x86)" instead of "Program Files" - ProgramFilesFolder always means the
  32-bit path on 64-bit Windows regardless of package bitness; getting the
  real 64-bit path needs the separate ProgramFiles64Folder standard
  directory id instead
- Windows Setup.exe now uses NSIS's Modern UI 2 (MUI2) instead of the old
  bare "classic" UI - a noticeably bigger wizard window and a properly
  readable license text box, matching what Inno Setup users are used to.
  Also fixes the installer-window icon going missing after that switch
  (MUI2 needs its own MUI_ICON/MUI_UNICON defines; the plain Icon/
  UninstallIcon directives alone aren't enough once MUI2 is in play)
- Windows .msi gained a real Welcome/License/Progress wizard (whenever
  Identity.LicenseFile is set), via wixl's bundled WixUI_Minimal-equivalent
  extension - verified empirically against a real compiled .msi, since
  GNOME's own docs only ever claimed installer UI support was missing.
  Silent/unattended installs (msiexec /qn or /qb) skip this dialog
  entirely as normal, so a new ACCEPTEULA=1 launch condition now blocks a
  silent install unless that's passed explicitly, matching how other
  vendors' MSI packages handle EULA acceptance for scripted deployment
- Documented the existing (no code changes needed) silent/unattended
  install switches for both Windows installers in the README: Setup.exe
  /S, msiexec /qn or /qb, and ALLUSERS="" for a current-user-only .msi
  install (ALLUSERS is a public property, so it's already overridable on
  the command line despite the package authoring it as 1 by default)
- Fixed macpkg always synthesizing a real Info.plist from Identity fields,
  which conflicted with this project's own historical package.sh/fpm
  convention of deliberately never having a real one (verified against a
  real installed sibling app - some IT security scanning apparently flags
  it, and macOS itself doesn't require it for an app to run or be found by
  Spotlight). No Info.plist is created by default now; one is only copied
  in, verbatim, if the project directory already has a real Info.plist
  file (e.g. a deliberately-renamed Info-plist.txt placeholder). This also
  needed switching pkgbuild from --component to --root mode - --component
  requires a valid bundle with Info.plist and hard-refuses otherwise,
  confirmed empirically, while --root just packages the directory as-is
- macpkg now also auto-copies Info-plist.txt/Readme-plist.txt into
  Contents/ (a sibling of MacOS/) verbatim, if present in the project
  directory, matching package.sh's own layout for these two placeholder/
  documentation files exactly - a Payload entry can't reach that location
  since Payload always lands under Contents/MacOS/
- Fixed a real rpm bug found via `rpm -e` on a real machine: every payload
  file installed correctly, but every directory it lived in was left
  behind empty forever on uninstall. Root cause: a Recursive Payload entry
  with Excludes set (this project's own assets/ entry, excluding
  mesa-win/*, among them) went through a path that emitted file entries
  only, silently dropping every directory - unlike an Excludes-free
  Recursive entry, which nfpm's own TypeTree expansion already handles
  correctly. rpm specifically needs an explicit directory entry (not just
  an implicit parent-of-file) to actually own and clean one up again.
  Confirmed fixed by rebuilding a real .rpm and checking `rpm -qlv` lists
  those paths with directory mode bits now. .deb was very likely
  unaffected by this one (its builder tars every directory, implicit or
  not) - confirmed via nfpm's own source, not yet independently retested.
  One level up from that fix, the install root itself (e.g. /opt/AppName)
  was still left behind empty after a real `rpm -e` - it's never the
  explicit destination of anything, only ever the parent of the
  binary/License.txt/Payload entries, so the same "implicit directories
  aren't owned" rule applied to it too. buildInfo now adds an explicit
  directory entry for the install root directly
- Fixed a raw GLFW panic ("NotInitialized", full Go stack trace) on Linux
  when launching the GUI with no DISPLAY/WAYLAND_DISPLAY set (a headless
  server, or an SSH session with no display forwarding) - confirmed on a
  real headless Ubuntu box. Now prints a clear message pointing at the CLI
  instead, and exits cleanly. Same check/message pattern as this project's
  sibling TaniumSensorExplorer (gui_display_linux.go/
  gui_display_nonlinux.go, checked before app.NewWithID is ever called)

Version 0.1.0 - September 03, 2026
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✨ NEW Cross-platform installer builder
▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔
- Project-based workflow: identity, per-OS/arch binaries, payload, and build
  targets all live in one packman.yaml project file (New/Open/Save/Save As)
- Identity tab: name, bundle ID, version, publisher, vendor, URL, description,
  license, icons, Windows/macOS/Linux-specific settings, output directory
- Generate ID (reverse-DNS bundle ID from GitHub URL or Publisher/Vendor) and
  Generate GUID (Windows Upgrade GUID) helpers
- Binaries tab: per-OS/arch binary paths (Windows/macOS/Linux x amd64/arm64),
  with automatic multi-arch output when more than one arch is filled in
- Payload tab: add extra files/folders with recursive copy and OS filtering
- Build tab: per-target checkboxes, live preflight status (tool installed? OS
  supported?), Re-check Tools, Start/Cancel, streaming build log
- Packaging backends: Windows Setup.exe (NSIS/makensis), Windows .msi (wixl),
  macOS .pkg (real .app bundle + pkgbuild), Linux .deb/.rpm (nfpm, buildable
  from any host OS) - each target builds independently of the others
- Headless CLI (packman): build, validate, doctor, list-targets subcommands
  for CI use, sharing the same preflight logic as the GUI
- System tray + main menu, Light/Dark/System theme, update checker with
  once-daily automatic check plus manual "Check for Updates", window size
  persistence, i18n scaffolding
- Windows VMs/locked-down hosts without hardware OpenGL automatically fall
  back to a bundled Mesa3D software renderer
- New Project now starts with a folder picker and best-effort-prefills
  Publisher, URL, License file, and icons from that directory's git
  remote/identity, LICENSE file, and assets/images icons
- Binaries tab "Scan folder..." guesses Windows/macOS/Linux binary paths
  from filename conventions, without ever overwriting a manually-set path
- Payload tab "Scan folder..." proposes payload entries from a directory
  (skipping anything already covered elsewhere), with a review step before
  anything is actually added
- Build tab's Start button is now a traffic light: green when ready, orange
  while building, back to green on success or red on failure
