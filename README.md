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

### Identity tab

- Name, bundle ID, version, publisher, vendor, URL, description, license file.
- Icons (`.ico` / `.icns` / `.png`).
- Windows: Upgrade GUID (with a **Generate GUID** button) and exe name.
- macOS: bundle executable, minimum OS version, category.
- Linux: desktop categories and comment.
- Output directory and filename template.
- **Generate ID** drafts a reverse-DNS bundle ID automatically — prefers
  `com.github.<owner>` when the project URL is a GitHub repo, otherwise derives
  one from the Publisher/Vendor's email domain or name.

### Binaries tab

- One file-picker per OS × architecture (Windows/macOS/Linux × amd64/arm64).
- Leave an architecture blank to skip building it; fill in more than one
  architecture per OS to produce multi-arch output automatically.

### Payload tab

- Table of extra files/folders to bundle alongside the binary (source, destination,
  recursive copy, OS filter) — the GUI equivalent of Inno's `[Files]` section or
  `fpm`'s `src=dest` arguments.
- Add File / Add Folder / Edit / Remove via a dialog.

### Build tab

- One checkbox and live status per target: Linux `.deb`, Linux `.rpm`, macOS
  `.pkg`, Windows `Setup.exe`, Windows `.msi`.
- **Re-check Tools** re-runs a preflight check (is the target supported on this
  host, and is the required external tool installed) and colors each status
  green/red.
- Start/Cancel with a streaming, scrollable build log. A running build is
  cancelled cleanly if the app is asked to quit mid-build.

### Packaging backends

- **Windows `Setup.exe`** — NSIS via `makensis` (buildable from any host OS):
  app metadata, optional license page, binary + payload copy, registry
  install-dir marker, uninstaller that stops the running exe first.
- **Windows `.msi`** — a real MSI via `wixl` (GNOME msitools): stable
  UpgradeCode with a per-build ProductCode, binary + payload tree, Start Menu
  shortcut.
- **macOS `.pkg`** — assembles a genuine `.app` bundle (Info.plist, `.icns`,
  CLI symlink) then calls Apple's `pkgbuild`. macOS-only backend.
- **Linux `.deb` / `.rpm`** — via `nfpm` (pure Go, no `fpm`/Ruby, no
  `rpmbuild`), so both formats build even from macOS or Windows.
- Backends run independently — one target failing doesn't stop the others — and
  the same preflight logic is shared by the GUI and the CLI's `doctor` command,
  so they never disagree about whether a tool is available.

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
  Show/Hide All Windows, Light/Dark/System theme, Help/Check for
  Updates/About, Quit. The tray icon also has a hover tooltip.
- "Show/Hide All Windows" (tray and main View menu) shows or hides the main
  window plus any open About/Help/Update window together in one click.
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
- No unsaved-changes confirmation on New/Open Project yet.
- Payload entries are edited via a dialog, not inline in the table.
- "Show/Hide All Windows" doesn't remember exactly which secondary windows
  were open — it can re-show one you'd already closed earlier in the session.

## Cross-platform support

- **Linux**: GNOME, KDE, XFCE, Cinnamon, MATE, etc. on X11 or Wayland.
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

## License

Free for personal, educational and commercial use, under the GNU GPL-3.0.

## Author

Allan Marillier

## Acknowledgments

- Built with [Fyne](https://fyne.io/) — an easy-to-use GUI toolkit for Go.
