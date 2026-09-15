# KrankyBear InstallerBear

A cross-platform desktop application (Go + [Fyne](https://fyne.io/)) for turning a
set of already-built binaries into native installers/packages for Windows, macOS,
and Linux — one shared project file, several output formats, no hand-maintained
NSIS/WiX/Inno/fpm scripts. A headless CLI is built into the same binary for
CI use (run with `-help` or `-?` for usage).

Design philosophy aligns with Fyne: ease of use, solid functionality, steady bug
fixing and performance work.

## Features

### Project-based workflow

- Everything lives in one `installerbear.yaml` project file: identity,
  per-OS/arch binaries, extra payload, install locations, hooks, and which
  targets to build.
- New / Open / Save / Save As, available from the File menu, a toolbar row
  above the tabs, and (New/Open/Save/Save As) the system tray menu — with
  the window title reflecting the project name and unsaved-changes state.
- Open is lenient (a work-in-progress project can be reopened even if incomplete);
  Save-before-build always validates first.
- New/New Sample Project/Open all start browsing beside the running
  executable the first time (where `ReleaseNotes.md`/
  `sample-installerbear.yaml` already live), rather than the OS's usual
  default (often your home directory) — a first-time user actually sees
  them instead of needing to already know to go looking. Save As starts
  in your home directory the first time instead, since a personal project
  file belongs there, not inside the app's own install directory. From
  the first successful use of any of the four, a single remembered
  "last used" directory takes over for all of them.
- Hand-typed `#` comments in `installerbear.yaml` survive an Open/Save
  round-trip — useful for notes, or temporarily commenting out a
  payload/binary/file-association entry. Manual-editing only: there's no
  GUI for adding or editing a comment, this just stops the app from
  silently deleting one you typed directly into the file. Best-effort —
  each entry's comment is matched back by a natural key (a Payload
  entry's `source`, a Binary's `os`+`arch`, a File Association's
  `extension`) rather than by position, so it survives reordering, but a
  comment attached to something since deleted has nothing left to attach
  to and is dropped.
- **New Sample Project...** (File menu + tray) writes a real, fully-featured
  example `installerbear.yaml` to a location you pick and opens it — a
  working starting point for anyone who'd rather adapt a real config than
  fill in a blank one. Prefers this app's own bundled sample (a genuine
  copy of the exact config used to build KrankyBear InstallerBear's own
  installers); falls back to a generated example (with a freshly minted
  Windows UpgradeGUID) if that bundled file isn't present, e.g. running
  from source.
- **Import Existing Config...** (File menu, plus `installerbear import
  -source <dir> -p installerbear.yaml` on the CLI) best-effort-imports an
  existing Inno Setup `.iss` and/or KrankyBear-template
  `build-config.sh`/`package.sh` into the current project — Identity,
  Windows Upgrade GUID/exe name, bundle ID, Linux desktop metadata,
  Payload entries, File Associations (from `[Registry]`), and Install
  Experience toggles (Desktop shortcut/Run at startup/Launch after install,
  recognized from `[Tasks]`/`[Run]`). A review step (GUI) or `-dry-run`
  (CLI) shows current → proposed for every field before anything is
  applied; re-running is safe and won't duplicate what's already there.
  `[Icons]`/`[UninstallRun]`/`[UninstallDelete]` lines that are already
  covered by InstallerBear's own default behavior are recognized as such
  rather than reported as unsupported; custom installer wizard pages and
  anything else outside these conventions aren't imported.

### Identity tab

- Name, bundle ID, version, publisher, vendor, URL, description, license file.
- Icons (`.ico` / `.icns` / `.png`), with a small live thumbnail next to the
  `.png` field (typing a path or using Browse both update it). `.ico`/`.icns`
  have no preview — Go has no standard decoder for either format, and this
  project would rather stay lean than add a dependency per format for a
  small benefit.
- Windows: Upgrade GUID (with a **Generate GUID** button), exe name,
  **Install for** (All users, the default — requires admin elevation and
  installs to `Program Files`/`ProgramFiles64Folder`; or Current user only —
  no elevation, installs to `%LOCALAPPDATA%`/`LocalAppDataFolder`, and moves
  Programs & Features registration from `HKLM` to `HKCU` since a non-elevated
  install can't write there. Author-time-only on both backends — neither
  `wixl`'s bundled UI nor a dependency-free NSIS script can offer this as a
  real end-user runtime pick), and three install-experience toggles —
  **Launch after install** (a real,
  checked-by-default "Launch \<App\> now" checkbox on both `Setup.exe`'s and
  `.msi`'s finish page, opted into here by the project author; the actual
  choice at install time is the end user's), **Desktop shortcut** (both
  installers already create a Start Menu shortcut unconditionally; this
  adds a second one on the Desktop — a real, checked-by-default Components-
  page checkbox on `Setup.exe`, but an author-time-only choice on `.msi`,
  since `wixl`'s bundled UI has no equivalent interactive mechanism), and
  **Run at startup (autostart)** (writes/removes a per-user `HKCU` Run
  value — same real-checkbox-on-`Setup.exe`/author-time-only-on-`.msi`
  split as Desktop shortcut, but *unchecked* by default even on
  `Setup.exe`'s Components page, since autostarting is a bigger surprise to
  spring on someone than an extra shortcut). All three toggles are
  Windows-only: macOS/Linux backends log a clear note rather than silently
  ignoring any of them.
- macOS: bundle executable, minimum OS version, category.
- Linux: desktop categories and comment.
- **Hooks**: `pre_install`/`post_uninstall` — inline shell script text run on
  the *target* machine, baked into the macOS `.pkg`/Linux `.deb`/`.rpm`
  package itself and executed later by its own install/uninstall action.
  Windows has no equivalent (NSIS/MSI have no shell interpreter to run
  script text in at all) — these two fields are silently ignored by
  `Setup.exe`/`.msi`, no note logged, since it's plain unavailable rather
  than a setting that could apply but doesn't.
  - `pre_install` runs before files are installed, on both macOS (`.pkg`'s
    own `preinstall` script) and Linux (`.deb`/`.rpm`'s own `preinst`).
  - `post_uninstall` runs after removal, but **Linux only** — a `.pkg`
    install has no OS-level uninstall action at all for anything to hook
    into, so this field has no effect on macOS (a clear progress note is
    logged during a macOS build when it's set). On Linux, InstallerBear's
    own desktop-database/icon-cache/mime-database refresh commands (see
    the `.desktop`-file and File Associations sections above) run
    *after* your own hook content, in the same script — keep this one to
    plain POSIX shell (see below) so those still work.
  - By default the text runs under `/bin/sh` (a `#!/bin/sh` + `set -e`
    header is added automatically). Start your own text with a shebang
    line (e.g. `#!/usr/bin/env python3`, `#!/bin/bash`) to use a different
    interpreter instead, exactly like a normal script file — InstallerBear
    only prepends the default header when your text doesn't already start
    with `#!`.
- Output directory, filename template, and **Post-build hook**
  (`output.post_build_hook`) — inline shell script text run once, immediately,
  on *this* build machine right after a successful build, with each built
  artifact's path passed in as an environment variable — see "Build tab"
  below for the full details.
- **Generate ID** drafts a reverse-DNS bundle ID automatically — prefers
  `com.github.<owner>` when the project URL is a GitHub repo, otherwise derives
  one from the Publisher/Vendor's email domain or name.

### Binaries tab

- One file-picker per OS × architecture (Windows/macOS/Linux × amd64/arm64).
- Leave an architecture blank to skip building it; fill in more than one
  architecture per OS to produce multi-arch output automatically.

### Payload tab

- Table of extra files/folders to bundle alongside the binary (source, destination,
  recursive copy, OS filter, excludes) — the GUI equivalent of Inno's `[Files]`
  section or `fpm`'s `src=dest` arguments.
- Every cell is directly editable in place — type into Source/Dest/OS filter/
  Excludes or toggle Recursive right in the table, no dialog round-trip needed
  for a quick tweak. Add File / Add Folder / Edit... (Browse-assisted) / Remove
  still go through a dialog, since adding a row or picking a new path via a
  file/folder browser both need one. Excludes takes a comma-separated list of
  glob patterns, same as the field on the Add/Edit dialog.
- The leftmost **Select** column's checkboxes (not a click-to-select row —
  `widget.Table` can't offer that once every cell is its own editable
  widget) drive **Remove** (deletes every checked row, any number at once)
  and **Edit...** (needs exactly one checked).
- **Click the Source, Dest, or OS column header to sort** by it (click again
  to reverse). This reorders `Project.Payload` itself, not just the on-screen
  view — comment-preservation matches entries by content, not position, so a
  hand-typed `#` comment stays attached to its own entry across a sort — so
  the saved YAML ends up in the new order too.
- **Source may be a glob pattern** (`*`/`?`/`[...]` — the same dialect
  `excludes:` already uses) instead of a literal path, e.g. `*.yaml` to
  bundle every YAML file in the project directory without listing each one
  by hand. Resolved once per build against whatever currently matches — a
  pattern matching nothing isn't an error, just nothing to bundle this
  time. `excludes:` also filters *which* matches are included, even for a
  non-recursive entry (its normal role is filtering inside a recursive
  copy) — "include broadly via Source, exclude specifically via Excludes."
- **OS filter can scope by architecture too**, not just OS: an entry in `os:`
  can be a bare OS name (`windows` — every arch of that OS) or an `os/arch`
  pair (`windows/arm64` — that arch only), the same slash convention
  Docker's `--platform`/`go tool dist list` already use. `mac`/`macos`
  (case-insensitive) are accepted as aliases for `darwin` on either side of
  the slash. E.g. `os: [windows/arm64, mac]`.
- **The Add/Edit dialog offers OS/arch as a checkbox grid**, not a free-text
  field: one row per OS (Windows/macOS/Linux), each with an "Any arch" box
  plus amd64/arm64 boxes ("Any arch" and the individual arch boxes are
  mutually exclusive per OS). This is the known, finite OS/arch matrix this
  project actually builds for, so checkboxes remove all typo/ambiguity risk
  versus typing `windows/arm64,mac` by hand — a single entry can target
  several specific OS/arch combos this way with no need for separate rows.
  Any pre-existing OS value the grid doesn't recognize (an exotic arch, a
  typo) is kept as-is rather than silently dropped. The inline table cell's
  OS column is unchanged — still a plain comma-separated text field for a
  quick glance or tweak without opening the dialog.

### File Associations tab

- Table of Extension/Description pairs (e.g. `.myp` / "My App Project") the
  installed app should be registered to open.
  - **Windows `Setup.exe`**: plain `HKCR` registry entries (ProgID,
    description, default icon, `shell\open\command`), author-time only.
  - **Windows `.msi`**: a `ProgId`/`Extension`/`Verb` component per
    association, verified against a real compiled `.msi`'s Registry table.
  - **Linux `.deb`/`.rpm`**: installs a shared-mime-info package
    (`/usr/share/mime/packages/<name>.xml`) and adds `MimeType=`/a
    trailing `%f` to the `.desktop` entry, so the app appears in "Open
    With" and receives the file path as an argument.
  - **macOS `.pkg`**: conditional — a real document-type association needs
    a `CFBundleDocumentTypes` entry in `Info.plist`, and this project never
    synthesizes a whole `Info.plist` from scratch (see the macOS `.pkg`
    note below). But when the project directory has a real `Info.plist`,
    or its own `Info-plist.txt` placeholder (even unrenamed — configuring
    a File Association at all is already an explicit opt-in to needing a
    real plist), the entries are merged into a copy of it automatically.
    With neither file present, there's nothing to merge into, so macpkg
    logs a clear progress note instead.

### Build tab

- One checkbox and live status per target: Linux `.deb`, Linux `.rpm`, macOS
  `.pkg`, Windows `Setup.exe`, Windows `.msi`.
- **Re-check Tools** re-runs a preflight check (is the target supported on this
  host, and is the required external tool installed) and colors each status
  green/red.
- Start/Cancel with a streaming, scrollable build log. A running build is
  cancelled cleanly if the app is asked to quit mid-build.
- **`output.post_build_hook`** (Identity tab's Output section, and
  `hooks.pre_install`/`post_uninstall` in a new Identity tab **Hooks**
  section): inline shell script text run once, on
  this build machine, right after every requested target/arch has finished —
  only when the whole build succeeded, skipped with a clear note otherwise.
  Every successfully built artifact's path is passed in as an
  `INSTALLERBEAR_OUTPUT_<TARGET>` environment variable (or
  `INSTALLERBEAR_OUTPUT_<TARGET>_<ARCH>` when a target built more than one
  arch), plus `INSTALLERBEAR_ARTIFACTS` (all of them, space-separated) and
  `INSTALLERBEAR_APP_NAME`/`INSTALLERBEAR_VERSION`/`INSTALLERBEAR_OUTPUT_DIR`.
  A general escape hatch for things this tool doesn't implement natively —
  uploading to GitHub Releases, code signing, notarization — rather than a
  dedicated feature per case.

### Packaging backends

- **Windows `Setup.exe`** — NSIS via `makensis` (buildable from any host OS):
  app metadata, optional license page, binary + payload copy, registry
  install-dir marker, an unconditional Start Menu shortcut (plus, when
  opted in, real Components-page checkboxes letting the person installing
  choose a Desktop shortcut and/or Run-at-startup too — the latter
  unchecked by default — and/or an optional "launch it now" finish-page
  checkbox — see Install Experience below), a real "Apps &
  Features"/Programs and Features registration (NSIS doesn't do this on
  its own the way MSI always does — every properly-built installer writes
  it by hand), an uninstaller that stops the running exe first, and
  `-?`/`-help`/`-h`/`--help`/`/?`/`/help` support — prints a usage message
  (the `/S` silent switch, `/D=`) and exits instead of silently launching
  the wizard like an unhandled installer argument normally would.
- **Windows `.msi`** — a real MSI via `wixl` (GNOME msitools): stable
  UpgradeCode with a per-build ProductCode, binary + payload tree, Start Menu
  shortcut (plus the same optional launch-after-install checkbox as
  `Setup.exe` — Desktop shortcut and Run-at-startup both stay author-time-only
  choices here, since `wixl`'s bundled UI has no equivalent Components-page
  mechanism), a proper icon in "Apps & Features"/Programs and
  Features when `Icons.ICO` is set (via `ARPPRODUCTICON`), and (when a
  license file is set, or Launch-after-install is on) a full
  Welcome/License/Progress wizard via wixl's bundled WixUI_Minimal-equivalent
  extension, gated by an
  `ACCEPTEULA=1` launch condition for silent/unattended installs.
- **macOS `.pkg`** — assembles a genuine `.app` bundle (`.icns`, CLI
  symlink) then calls Apple's `pkgbuild` in `--root` mode (not
  `--component`, which requires a real `Info.plist` and refuses to build
  without one). No `Info.plist` is created by default — only copied in,
  verbatim, if the project directory already has a real one (e.g. a
  renamed `Info-plist.txt` placeholder); some IT security scanning flags
  a real `Info.plist`, and macOS itself doesn't require one for an app to
  run or be found by Spotlight. `Info-plist.txt`/`Readme-plist.txt`, if
  present in the project directory, are auto-copied verbatim into
  `Contents/` (a sibling of `MacOS/`) as harmless documentation — a
  Payload entry can't reach that location, since Payload always lands
  under `Contents/MacOS/`. Exception: when File Associations are
  configured, `Info-plist.txt` is promoted to a real, functional
  `Info.plist` too (with the association entries merged in), even without
  being renamed — needing a genuine `Info.plist` at all is an inherent
  requirement of that feature, not a policy this project relaxes lightly;
  a project with no File Associations configured keeps the exact same
  copied-verbatim-only-if-real, no-Info.plist-by-default behavior as
  before. macOS-only backend.
- **Linux `.deb` / `.rpm`** — via `nfpm` (pure Go, no `fpm`/Ruby, no
  `rpmbuild`), so both formats build even from macOS or Windows. Both
  formats always install a real app-menu entry
  (`/usr/share/applications/<name>.desktop`, using `Linux.DesktopCategories`/
  `DesktopComment`) plus, when `Icons.PNG` is set, the icon it references
  into the hicolor icon theme — unconditional, the same way both Windows
  installers always create a Start Menu shortcut, since a `.desktop` file is
  how a Linux app appears in the launcher at all on every major desktop
  environment, not an optional extra. `update-desktop-database`/
  `gtk-update-icon-cache` are refreshed best-effort on install and removal so
  the entry/icon show up without a logout.
- Backends run independently — one target failing doesn't stop the others — and
  the same preflight logic is shared by the GUI and the CLI's `doctor` command,
  so they never disagree about whether a tool is available.

### Silent / unattended installs (for scripted deployment)

Both Windows installers already support the standard silent-install switches
their underlying engines provide — nothing to configure in InstallerBear
itself:

- **`.exe`**: `Setup.exe /S` (NSIS's standard silent flag).
- **`.msi`**: `msiexec /i pkg.msi /qn` (fully silent) or `/qb` (progress bar
  only). Add `ALLUSERS=""` to install for the current user instead of the
  per-machine default (`ALLUSERS` is a public MSI property, so it's
  overridable on the command line even though the package authors it as
  `1`). If a license file is set, also pass `ACCEPTEULA=1` — otherwise the
  install refuses to proceed unattended (see above), since a silent run
  never shows the license dialog to accept in the first place.

### Headless CLI

Reachable via the same binary, for CI or scripting:

```
installerbear build -p installerbear.yaml -t deb,rpm,macpkg,winexe,winmsi -o installers
installerbear build -p installerbear.yaml -t linux --set version=1.2.3 --dry-run
installerbear validate -p installerbear.yaml
installerbear doctor
installerbear list-targets
installerbear -help   # or -?
```

- `build` accepts target group aliases (`linux`, `mac`, `windows`, `all`),
  `--set KEY=VALUE` overrides (version/publisher/vendor/url/outputdir), `-o`,
  `--dry-run`, and `--verbose`; its exit code is 0 (all targets ok), 1 (all
  failed), or 2 (partial).
- `doctor` and `list-targets` report per-target host support and tool
  availability without touching Fyne/GLFW at all, so they run fine headless.
- `-help`/`-?`/`-h`/`--help` print full CLI usage and exit, without touching
  Fyne/GLFW either.

### General application features

- System tray and main menu mirror each other: New/Open/Save/Save As,
  Show/Hide All Windows, Light/Dark/System theme, Help/Release Notes/Check
  for Updates/About, Quit. The tray icon also has a hover tooltip.
- "Show/Hide All Windows" (tray and main View menu) shows or hides the main
  window plus any open About/Help/Update/Release Notes window together in
  one click.
- Release Notes viewer: opens the installed `ReleaseNotes.md`/`.txt` (checked
  in that order) from beside the running executable, rendered through Fyne's
  built-in Markdown support — no new dependency, and a plain `.txt` still
  reads fine as plain paragraphs. Reads from disk at runtime rather than
  embedding, since release notes can grow large over a project's life and an
  embed would bake that size into every build permanently. If no release
  notes file is found beside the executable (e.g. a standalone binary copied
  without it), the window says so plainly with a link to the project's
  GitHub page, rather than silently showing nothing or shelling out to the
  OS's default text viewer.
- Light/Dark/System theme, remembered across launches.
- Update checker: a quiet automatic check once per day on launch, plus an
  unthrottled manual "Check for Updates", with a HardHat badge on About/Update
  when the local build is ahead of the latest published release.
- Main window size is remembered across launches.
- i18n scaffolding (English plus bundled locale packs) ready for translation.

## Known limitations

- No code signing for any output (NSIS, MSI, `.pkg`, `.deb`/`.rpm` are all
  produced unsigned).
- No file associations or custom installer wizard pages.
- "Show/Hide All Windows" doesn't remember exactly which secondary windows
  were open — it can re-show one you'd already closed earlier in the session.

## Cross-platform support

- **Linux**: GNOME, KDE, XFCE, Cinnamon, MATE, etc. on X11 or Wayland.
  Launching with no `DISPLAY`/`WAYLAND_DISPLAY` set (a headless server, or
  an SSH session without display forwarding) prints a clear message
  pointing at the CLI instead of a raw GLFW crash/panic.
- **macOS**: 10.13 (High Sierra) or later.
- **Windows**: Windows 10 or later. Some VMs and locked-down hosts have no
  usable hardware OpenGL, which most Fyne apps otherwise crash or hang on
  with no explanation — InstallerBear automatically probes for it at launch and,
  only if that fails, falls back to a bundled Mesa3D software renderer and
  relaunches itself, with no user action needed. Real hardware OpenGL is
  always preferred when available (it's faster); nothing changes on a
  normal machine with a working GPU. The installer bundles this fallback
  automatically. The **portable (zip) Windows build does not** — if you're
  running the portable version on a machine without hardware OpenGL, grab
  `mesa-fallback.zip` from the same release, extract it into a
  `mesa-fallback` folder next to `KrankyBearInstallerBear.exe`, and the same
  automatic fallback applies. Most users on a normal machine will never
  need this file at all.

## Building & running

Requires Go and a Fyne-capable toolchain (CGo + OpenGL on desktop):

```
go run .
go build -o installerbear .
```

Platform helpers: `compile-mac.sh`, `compile-win.sh`, `compile-linux.sh`, and
`package.sh` (`.deb`/`.rpm`, macOS `.pkg`).

External tools used by the packaging backends (only needed for the targets you
actually build): `makensis` (NSIS), `wixl` (msitools), `pkgbuild` (macOS,
built-in), `nfpm` (bundled as a Go dependency, no external install needed).

### Building on Windows

Today's workflow builds InstallerBear itself on the Mac (`compile-win.sh` syncs
source to a Windows box over SSH, compiles there, syncs the `.exe` back), then
packages everything from the Mac. If the Mac is ever replaced by a Windows
machine, here's what that Windows host needs installed to keep doing both
jobs itself:

- **Go + a C compiler** — Fyne requires cgo. MSYS2/MinGW-w64
  (`mingw-w64-x86_64-gcc`) or TDM-GCC, whichever `go env CC` resolves to.
- **`makensis` (NSIS)**, for the `.exe` — `choco install nsis` or
  `scoop install nsis`, or the official installer from nsis.sourceforge.io.
  Make sure its `Bin` folder is on `PATH`.
- **`wixl` (msitools)**, for the `.msi` — not native to Windows; install via
  MSYS2: `pacman -S mingw-w64-x86_64-msitools`, then put that MSYS2 mingw64
  `bin` directory on `PATH`. This gives you `wixl.exe` directly, no WSL
  needed.
- **`nfpm`** for `.deb`/`.rpm` — nothing to install; it's a pure-Go
  dependency built into `installerbear.exe` itself, so these two already
  build fine on Windows today.
- **`go-winres`** (`go install github.com/tc-hib/go-winres@latest`) to
  regenerate `rsrc_windows_amd64.syso` (InstallerBear's own exe icon/version)
  locally instead of relying on the Mac→Windows sync in `compile-win.sh`.
- **macOS `.pkg` — the one tool you can't get back.** `pkgbuild` is Apple's
  own tool with no Windows or Linux equivalent, and `macpkg.HostSupported()`
  hard-gates this backend to `darwin`. Losing the Mac means losing the
  ability to produce `.pkg` unless you keep *some* access to macOS — a spare
  Mac, or a CI runner (e.g. GitHub Actions' `macos-latest`). A local macOS VM
  isn't a practical substitute here: Apple's license terms restrict
  virtualizing macOS to genuine Apple hardware.

Run `installerbear doctor` on whatever host you're on — it reports per-target
readiness (host support + tool availability) without needing anything beyond
InstallerBear itself.

## License

Free for personal, educational and commercial use, under the GNU GPL-3.0.

## Author

Allan Marillier

## Acknowledgments

- Built with [Fyne](https://fyne.io/) — an easy-to-use GUI toolkit for Go.
