# Release Notes

KrankyBear InstallerBear: cross-platform GUI + CLI for packaging pre-built binaries into native Windows, macOS, and Linux installers, from one shared project file.

## Future Ideas


### Next up

*Nothing queued right now.*

### Maybe next, under consideration
- Windows equivalent of `hooks.pre_install`/`post_uninstall` (currently no-ops there - NSIS/MSI have no shell interpreter). Technically buildable, not just a dead end: NSIS already shells out to an external `.exe` for the uninstall-time taskkill call, so a Windows hook could write the user's script text to a temp `.ps1`/`.bat` and `ExecWait powershell.exe`/`cmd.exe` against it the same way; `.msi` could follow the already-proven Launch-after-install CustomAction pattern to invoke an external program too. Deliberately parked (2026-09-14) - Allan doesn't use pre/post hooks much and has other tools for that job, but we recognize other people will see value in this

### Bigger; deferred for now, maybe never

- Custom installer wizard pages - still just an idea with no concrete design (what a "page" would even configure isn't defined), and deliberately deprioritized as of 0.6.0: the real per-engine cost (wixl has no custom-dialog authoring precedent at all) isn't justified without a specific use case in view. Most "ask the user something at setup time" needs are better solved as first-run setup inside the app itself anyway - no per-installer-engine parity problem to solve at all
- Real Linux autostart (XDG autostart) is still deliberately unsolved - the only real mechanism there (/etc/xdg/autostart) is system-wide, unlike Windows' own per-user toggle, a genuine semantic mismatch worth a real decision rather than a quick wrong-shaped fix - for now we will defer, maybe skip this as never do, to be decided
- Windows ARM64 .msi is currently NOT achievable - confirmed empirically 2026-09-14 by testing wixl 0.106 directly: it only recognizes x86/x64/intel/intel64 as -a values, arm64/aarch64/amd64 all hard-error with "arch of type 'X' is not supported". Not a hardcoded-flag bug fixable here - a genuine limitation of the wixl build this project deliberately chose over Microsoft's own WiX (path-validation bug on non-Windows hosts, see winmsi's own package doc comment). Setup.exe/NSIS has no such limitation - winexe/build.go has no arch-specific packaging logic at all, since NSIS doesn't encode target CPU architecture into the installer format the way MSI does - so it should already work for a real windows/arm64 binary with zero code changes, though this is genuinely untested on real ARM64 Windows hardware. If real MSI-for-ARM64 is ever worth solving: a second, Windows-only backend using real Microsoft WiX (v4/v5) would be the honest path, since the original reason to avoid it (broken path validation on macOS/Linux build hosts) wouldn't apply on a build that genuinely runs on a native Windows GitHub Actions runner (windows-11-arm/windows-11-vs2026-arm, GA since 2026-08-19) - a bigger, separate piece of work, not a quick fix

### Lower priority for now

No certs yet for code signing, and choco/brew were just alternative-packaging thoughts to consider later, not a pressing need.

- code signing for NSIS/MSI/pkg/deb/rpm outputs
- choco and brew, with notes on self hosting use and also notes on submission and approval for real choco and brew based hosting

### Maybe later, lower value

- maybe some day i18n language support in the application itself (the `assets/i18n` scaffolding already exists - see CLAUDE.md - but wiring an actual language switcher through every window is a real, substantial investment, not a quick add). Deliberately left parked (2026-09-14) until an actual non-English user is driving it, not just "would be nice"
- maybe some day letting installer-generated dialogs themselves show in other languages (distinct from the app's own i18n above - this would be about `Setup.exe`/`.msi`'s own wizard text). Checked empirically (2026-09-14): genuinely cheap on `Setup.exe` (NSIS's MUI2 ships built-in language packs, just `!insertmacro MUI_LANGUAGE "German"` etc.), but no free lunch on `.msi` - `wixl`'s bundled UI extension ships zero translated string tables, so that half would mean hand-authoring every dialog string per language from scratch. Deliberately parked until someone actually asks for a non-English installer
- macOS `.dmg` alongside `.pkg` - settled as not worth pursuing (2026-09-14): We have built both before and found no real advantage either way for the end user, `.pkg` just easier for creating. Not revisiting unless something changes

## Version 0.7.0 - September 15, 2026

- Hand-typed `#` comments in `installerbear.yaml` now survive being opened
  and saved by this app - a real bug Allan found by hand: he'd edited
  `sample-installerbear.yaml`'s own header comment block, opened the
  result in the GUI, saved, and every comment silently vanished (plain
  `gopkg.in/yaml.v3` `Marshal`/`Unmarshal` doesn't track comments at all).
  Fixed by parsing a loaded project through a `yaml.Node` tree (not just
  straight into the struct) and, on save, merging that tree's comments
  onto a fresh encoding of the project's current values before writing -
  matched by key name for a mapping, and by a natural per-shape key for a
  list (a Payload entry's `source`, a Binary's `os`+`arch`, a File
  Association's `extension`, a plain scalar list like `targets` by its
  own value) rather than by position, so a comment survives even if the
  list around it was reordered or added to. Deliberately manual-editing
  only, per Allan's own framing - there's no GUI for adding or editing a
  comment, this only stops the app from silently destroying one typed
  directly into the file (ordinary `#`-to-end-of-line syntax, exactly
  like a shell script). A project built by hand (a brand-new project, the
  generated sample, every existing test) has nothing to preserve, so it
  marshals exactly as before - confirmed via Allan's own hands-on retest
  (add comments, Open, Save As..., reopen, verify) as well as the
  automated test suite
- Documented `hooks.pre_install`/`post_uninstall` for the first time
  beyond "runs on the target machine" (README, in-app Help, and the
  `Hooks` struct's own doc comment): both run under `/bin/sh` by default,
  but starting the hook's own text with a shebang (e.g.
  `#!/usr/bin/env python3`) switches the interpreter, same convention as
  a normal script file. `pre_install` runs on both macOS and Linux;
  `post_uninstall` is Linux-only - a `.pkg` install has no OS-level
  uninstall action to hook into at all. Windows has no effect on either
  (NSIS/MSI have no shell interpreter to run script text in) - a real
  Windows equivalent is technically buildable (NSIS could shell out to
  `powershell.exe`/`cmd.exe` the same way it already does for the
  uninstall-time taskkill call) but deliberately parked, see "Future
  Ideas". One real caveat documented too: InstallerBear's own
  desktop-database/icon-cache/mime-database refresh commands are appended
  after `post_uninstall`'s own text in the same script on Linux, so a
  custom shebang there should stay POSIX-shell-compatible
- New/New Sample Project/Open Project/Save Project As no longer default
  to whatever folder the OS's own file dialog happens to start in
  (usually the user's home directory) - Allan's own idea, since a fresh
  install otherwise drops a first-time user in an empty home folder with
  no clue `ReleaseNotes.md`/`sample-installerbear.yaml` exist right next
  to the installed binary. New Project/Open Project/New Sample Project
  (even though it's technically a Save dialog - its whole point is
  showing a first-time user a real example, the same idea as Open) now
  start beside the running executable the first time, before anything's
  been remembered; Save Project As starts in the user's home directory
  instead, since a personal project file belongs there, not inside the
  application's own install directory. From the first successful use of
  any of the four, a single shared "last used project directory"
  preference takes over and all four start there from then on - the
  common "remember the last folder I was in" convention most
  file-dialog-heavy apps already follow
- Payload `source:` can now be a glob pattern (`*`/`?`/`[...]` - the same
  dialect `excludes:` already uses, deliberately, rather than a second
  pattern syntax to learn; real regex was considered and dropped for that
  consistency) instead of a literal path, for a project with a larger
  number of files to bundle than makes sense to list by hand one at a
  time - e.g. `source: "*.yaml"` to bundle every YAML file in the project
  directory. Resolved once per build against whatever currently matches;
  matching nothing isn't an error, just nothing to bundle this run.
  `excludes:` now also filters *which* glob matches get included, even for
  a non-recursive entry (previously only meaningful inside a recursive
  copy) - "include broadly via `source`, exclude specifically via
  `excludes`," exactly the workflow Allan asked for. Every backend's own
  copy logic is completely unchanged - a single shared expansion step
  (`packproject.ExpandedPayload`) resolves patterns into concrete files
  once, before any backend ever runs, so none of the four needed to learn
  anything about wildcards at all. Deliberately never mutates the
  project's own stored Payload (what the GUI edits and saves back to
  installerbear.yaml) - only the in-memory copy actually used for
  validating and building sees the expansion, so a `source: "*.yaml"`
  pattern survives being built and saved again, rather than freezing into
  today's matches. Verified with a real build: a `.deb` built against a
  glob `source` correctly bundled the matching files and honored
  `excludes` on top
- Payload tab: the inline table's Dest column now shows greyed-out
  placeholder text ("(install root)") when blank, so a blank Dest reads as
  intentional rather than an accidental omission - Allan's own idea,
  prompted by worrying someone might type `dest: <same name as the file>`
  by hand not realizing that nests the file one level deeper (Dest is
  always a destination *directory*; the installed filename already comes
  from Source's own basename automatically)
- While looking into that, found the exact bug he was worried about
  already happening automatically: **Add File...** (and "Scan folder...")
  defaulted a plain file's Dest to its own basename (e.g. adding
  `branding.go` proposed `Dest: branding.go`), which silently installed it
  at `.../branding.go/branding.go` instead of `.../branding.go` - the
  installed filename already comes from Source's own basename, so
  defaulting Dest to that same basename doubles it up as an extra nested
  directory. A folder's own basename is still the correct default for a
  **Recursive** entry (its contents belong under a same-named
  subdirectory); only the plain-file case was wrong. Fixed in both
  `showPayloadDialog`'s auto-fill and `projectscan.ScanPayloadCandidates`
  ("Scan folder..."'s own proposals) - a plain file now defaults to a
  blank Dest (the install root), matching how this project's own
  `installerbear.yaml` already writes `ReleaseNotes.md`'s entry by hand
- Payload tab gained a fifth column, **Excludes** (comma-separated glob
  patterns, same as OS filter's own style), on both the inline table and
  the Add/Edit dialog - `excludes:` had no GUI exposure at all until now,
  only ever settable by hand-editing the `.yaml` file directly (or
  inherited from an imported `.iss`). Asked for right after adding the
  `source: "*.yaml"` example above, since narrowing down a glob match is
  the other half of that feature - now usable end to end without dropping
  to the file
- Payload tab's table now stretches its last column (Excludes) to fill
  whatever width is left over in the window, instead of leaving dead space
  and forcing a horizontal scrollbar even on a plenty-wide window - found
  right after adding the Excludes column above. `widget.Table` itself has
  no "stretch"/flexible-column concept at all (confirmed by reading Fyne's
  own source - `SetColumnWidth` only ever sets a fixed width), so this
  needed a small custom `fyne.Layout` wrapping the table, recalculating
  the last column's width on every resize
- New Validate check: `macos.bundle_executable` can no longer end in
  `.exe` - found via Allan's own hands-on macOS testing, where this
  project's own `installerbear.yaml`/`sample-installerbear.yaml` had
  `KrankyBear-InstallerBear.exe` (copy-pasted alongside `windows.exe_name`).
  Harmless functionally - macpkg just copies the binary verbatim under
  whatever name is configured, and it launched fine - but `.exe` is a pure
  Windows-ism that never belongs on a macOS executable and looks wrong
  inside the installed `.app` bundle. Fixed in both files; the new check
  catches the same mistake for anyone else, the same way
  `validateWindowsExeName` already catches a mismatched Windows one
- Fixed a real, significant bug found via Allan's own hands-on testing
  while migrating a second, much larger real project: the Payload tab's
  **Remove** button (and **Edit...**) silently did nothing. Root cause,
  confirmed by reading Fyne's own hit-testing code
  (`internal/driver.FindObjectAtPositionMatching`): every cell in this
  table is filled edge-to-edge by its own interactive `Entry`/`Check`
  widget, and Fyne always dispatches a click to the deepest matching
  object under the pointer - which is always that cell's own widget,
  never bubbling up to the Table's own row-selection
  (`OnSelected`/`OnUnselected`). So row selection had silently never
  worked at all since cells became directly editable in place (0.3.0) -
  not something a tweak to that mechanism could fix, since the same
  dispatch rule applies to any cell containing its own focusable widget.
  Fixed with a dedicated **Select** checkbox column instead (Allan's own
  suggested design) - completely sidesteps the problem, and as a bonus
  now supports removing several rows at once, not just one. The File
  Associations tab had the exact same bug (same table pattern, same root
  cause) and got the identical fix
- Payload `os:` filter can now scope by architecture, not just OS - an
  entry can be a bare OS name (`windows` - every arch of that OS,
  unchanged) or an `os/arch` pair (`windows/arm64` - that arch only), the
  same slash convention Docker's `--platform`/`go tool dist list` already
  use rather than a new syntax to invent. `mac`/`macos` (case-insensitive)
  are now also accepted as aliases for `darwin` on either side of the
  slash. Asked for while migrating `../TaniumMigrator` - some payload
  files there are only relevant to one architecture, and bundling both
  unconditionally wasted space (and could confuse users poking around the
  installed files). All four backends previously hand-rolled their own
  identical OS-only check; consolidated into one shared
  `PayloadEntry.AppliesToOS(os, arch)` method so they can't drift out of
  sync with each other again. Verified with a real build: two `.deb`s
  (amd64 and arm64) built from the same project, confirming an
  arch-scoped entry landed in only the matching one while a bare-OS entry
  landed in both
- Payload tab's Source/Dest/OS column headers are now clickable to sort by
  that column (click again to reverse direction) - asked for after Allan
  compared the new OS/Excludes columns against a real, larger project's
  config and wanted an easy way to visually check everything was covered.
  This is a real, persisted reorder of `Project.Payload`, not just an
  on-screen view - comment-preservation (0.7.0's own comment feature above)
  already matches entries by content rather than position, so a hand-typed
  `#` comment stays attached to the right entry across a sort, which made
  reordering the underlying data itself the simpler and safer choice over
  a separate row-index overlay. `widget.Table` has no built-in sortable-
  header concept, but its `CreateHeader`/`UpdateHeader` callbacks accept
  any `fyne.CanvasObject`, so the header row is now built from `Button`s
  instead of plain `Label`s (mirroring the same pattern already proven in
  `../KrankyBearProcessMiner`/`../KrankyBearCommander`) - Select/Recursive/
  Excludes get the same Button type (Table recycles one header object per
  column position, so every header must share a type) with no click
  handler wired, so they're inert
- The OS filter's placeholder/hint text (both the inline table cell and
  the Add/Edit dialog) now shows more concrete examples
  (`linux/amd64, darwin/amd64, mac/arm64`) instead of a single
  `windows/arm64,mac` example that Allan felt could read as unclear on its
  own
- The Add/Edit Payload dialog's OS filter is now a checkbox grid (one row
  per OS - Windows/macOS/Linux - each with an "Any arch" box plus amd64/
  arm64 boxes, "Any arch" mutually exclusive with the individual arch
  boxes) instead of free-text typing, removing all typo/ambiguity risk when
  one entry needs to target several specific OS/arch combos at once (e.g.
  mac/arm64 + linux/arm64 + linux/amd64 - three checkboxes, no need for
  separate Payload rows). The OS/arch space is small and fully known (the
  same set `AppliesToOS`/the Binaries tab already understand), which is
  what made checkboxes practical here. A pre-existing OS value the grid
  doesn't recognize (an exotic arch, a typo) is preserved as-is rather than
  silently dropped when the dialog saves. The inline table cell's OS
  column is unchanged - still plain comma-separated text, confirmed still
  wanted for a quick glance/tweak without opening the dialog

## Version 0.6.0 - September 14, 2026

- File Associations now also work on macOS: when the project directory has
  a real `Info.plist`, or its own `Info-plist.txt` placeholder (promoted
  to a real, functional one even without renaming - configuring a File
  Association at all is already an explicit opt-in to needing a real
  plist), macpkg merges a `CFBundleDocumentTypes` array into a copy of it
  automatically. An author's own hand-written `CFBundleDocumentTypes` is
  always left untouched, never fought or duplicated; with neither file
  present there's still nothing to merge into, so macpkg logs a clear
  progress note instead. No plist-parsing library added - hand-built XML
  injection in the same spirit as this project's shared-mime-info
  generator, finding the root `<dict>`'s own closing tag (guaranteed to be
  the last `</dict>` before `</plist>`, by XML nesting) and inserting the
  new block right before it
- `ReleaseNotes.md` converted to proper Markdown (real `#`/`##`/`###`
  headings, no more decorative `━` underlines) for better formatting and
  future readability. Two real regressions this could have introduced were
  caught and fixed in the same pass, since two tools parse this file's
  exact shape by regex/string-match, not just render it: `internal/
  projectscan`'s "New Project" smart-scan (tolerant of an optional `#`
  heading marker now) and `setver.sh`'s own version-bump header insertion
  (now matches/writes `## Version ...` instead of the old bare form)
- "Import Existing Config" (GUI + CLI) now also recognizes `[Tasks]`/
  `[Run]` conventions and maps them onto InstallerBear's own already-
  shipped `InstallExperience` toggles, rather than treating them as
  unsupported: Inno's own `"desktopicon"` task name -> Desktop shortcut; a
  task name containing "startup"/"autostart" -> Run at startup; a `[Run]`
  entry with the `postinstall` flag pointing at the app's own exe ->
  Launch after install. `[Icons]`/`[UninstallRun]` lines matching the
  Start Menu/Desktop shortcut and taskkill-on-uninstall conventions
  InstallerBear already handles unconditionally are now recognized as
  already-covered too, instead of being misreported as unsupported.
  `[UninstallDelete]` entries resolving inside the install directory get a
  clear, honest note: already cleaned up for free by `Setup.exe`'s
  uninstaller (which removes the whole install directory recursively) but
  **not** by `.msi`'s (Windows Installer only removes what it tracked, and
  the one real WiX mechanism for this - `RemoveFile`/`RemoveFolder` -
  crashes `wixl` outright when compiled, confirmed empirically: the same
  class of tool limitation as `wixl`'s missing ARM64 support). Nothing
  outside these specific conventions is guessed at - a Quick Launch icon
  task, a custom `[Run]` step, a path deleted from outside the install
  directory, and so on are still reported as not imported, same as before
- Custom installer wizard pages, and importing anything beyond the
  `[Tasks]`/`[Run]`/`[Icons]`/`[UninstallRun]`/`[UninstallDelete]`
  conventions above, remain explicitly out of scope (see "Future Ideas")
- `Setup.exe` now compiles with `SetCompressor /SOLID lzma` instead of
  NSIS's own weaker `zlib` default - found while investigating why this
  project's own InstallerBear-built installers came out noticeably bigger
  than its old Inno/fpm-built ones for the exact same payload (Inno Setup
  has always defaulted to lzma; NSIS doesn't unless told to). Benefits
  every project built with this tool, not just this one
- Fixed this project's own `installerbear.yaml`/`sample-installerbear.yaml`
  bundling ~18MB of `assets/images`/`assets/sounds` into every `.pkg`/
  `.deb`/`.rpm` as loose Payload files that the running app never actually
  reads from disk - `bundled.go` already `//go:embed`s the handful of
  images the app really needs straight into the binary at compile time.
  The old `package.sh` already knew this (it stages `assets/images`/
  `sounds` into scratch space but never actually lists them in any of its
  three `fpm` file-list arrays), InstallerBear's migrated Payload config
  just never carried that exclusion over. Fixed by replacing the single
  blanket `source: assets, dest: assets, recursive: true` entry (with an
  `excludes:` blocklist for `mesa-win/*`/`images/*`/`sounds/*`) with an
  explicit allowlist instead - just `source: assets/i18n, dest: assets/i18n,
  recursive: true` plus the existing per-file `mesa-win` entries - rather
  than keeping the blocklist approach: Allan's own call, since a Payload
  entry that bundles a whole folder *except* a hardcoded blocklist would
  silently and confusingly exclude a subfolder a project's own
  `installerbear.yaml` author might genuinely want bundled (there's no
  tool-level filtering either way - this is purely how these two specific
  config files list what they want - but an allowlist makes that obvious
  by inspection, a blocklist doesn't). Confirmed via a real rebuild that
  `.deb`/`.pkg` both dropped back down to roughly their old fpm-built
  sizes (`.deb`: 35.3MB -> 16.9MB; `.pkg`: 36.2MB -> 17.8MB), with the
  installed `assets/i18n/*` layout unchanged either way. A one-off fix
  to this project's own config, not a change to InstallerBear itself -
  worth checking for the same pattern (a Payload entry bundling a whole
  `assets/`-style folder whose images/sounds are already embedded via
  `go:embed`) when migrating any other KrankyBear-family project onto this
  tool. Both fixes above confirmed on real hardware via Allan's own
  `compile-all.sh` + side-by-side InstallerBear build: `Setup.exe` dropped
  from 52.0MB to 33.4MB (now smaller than the old Inno-built one), and
  `.pkg`/`.deb`/`.rpm` all landed within ~1-2MB of their old fpm-built
  sizes across amd64/arm64/x86_64/aarch64
- Fixed a real, reproducible bug in Show All Windows/Hide All Windows
  (View menu, tray, and each of them individually): explicitly closing a
  secondary window (About/Help/Update/Release Notes) and then using Hide
  All + Show All brought it back anyway - confirmed by Allan's own hands-on
  test (open all four, hide, show - all four return, correct; close two,
  hide, show - all four return again, wrong). Root cause: the previous
  implementation called Show()/Hide() on every window Fyne's driver
  happened to still hold a reference to, and this app's own
  `SetCloseIntercept` on every secondary window only ever calls `Hide()`
  (never a real `Close()`), so a "closed" window was indistinguishable
  from a merely-hidden one to that blanket approach. Fixed by porting
  `../KrankyBearClipboardSentinel`'s own `windowregistry.go` - the
  reference implementation of CLAUDE.md's "Hide all / show all windows"
  convention across the KrankyBear project family - which tracks each
  window's own explicitly-set "open" flag (set true on show, false only by
  that window's own close-intercept) instead of querying visibility from
  the driver. Show All Windows now restores exactly the set that was open,
  nothing more
- Fixed a real bug found via Allan's own hands-on testing: opening the
  bundled `sample-installerbear.yaml` (which has a real header comment
  block explaining it) in the GUI and saving silently dropped every
  comment - `gopkg.in/yaml.v3`'s normal `Marshal`/`Unmarshal` round-trip
  doesn't preserve comments at all. Fixed by parsing a loaded project
  through a `yaml.Node` tree (not just straight into the struct) and,
  when saving, merging that tree's comments onto a fresh encoding of the
  project's current values before writing - matching each entry by key
  name for a mapping, and by a natural per-shape key for a list
  (`PayloadEntry` by `Source`, `BinaryEntry` by `OS`+`Arch`,
  `FileAssociation` by `Extension`, a plain scalar list like `Targets` by
  its own value) rather than by position, so a comment survives even if
  the list around it was reordered or added to elsewhere. Deliberately a
  manual-editing-only capability, per Allan's own framing - there's no
  GUI for adding or editing a comment, this only stops the app from
  silently destroying one a human typed directly into the `.yaml` file
  (ordinary `#`-to-end-of-line syntax, exactly like a shell script). A
  project built by hand (a brand-new project, the in-code generated
  sample, every existing test) has nothing to preserve, so it marshals
  exactly as before - this only changes behavior for a project actually
  loaded from a real file
- Clarified in the README/in-app Help/schema doc comments exactly how
  `hooks.pre_install`/`post_uninstall` work, since neither had ever really
  been documented beyond "runs on the target machine": `pre_install` runs
  on both macOS (`.pkg`'s own preinstall script) and Linux (`.deb`/`.rpm`'s
  own preinst); `post_uninstall` runs on Linux only (a `.pkg` install has
  no OS-level uninstall action to hook into at all); Windows has no
  effect on either, since NSIS/MSI have no shell interpreter to run script
  text in. Both run under `/bin/sh` by default, but starting the hook's
  own text with a shebang line (e.g. `#!/usr/bin/env python3`) switches
  the interpreter, exactly like a normal script file. One real caveat
  worth knowing: InstallerBear's own desktop-database/icon-cache/
  shared-mime-info refresh commands are appended after `post_uninstall`'s
  own text in the same script on Linux, so a custom shebang there should
  stay POSIX-shell-compatible for those to keep working

## Version 0.5.0 - September 14, 2026

- New File Associations: a new Identity-adjacent "File Associations" tab
  lists Extension/Description pairs (e.g. `.myp` / "My App Project"), wired
  into every applicable backend, Windows/Linux/macOS all included
  - macOS (`.pkg`): conditional, not automatic - a real document-type
    association needs `CFBundleDocumentTypes` in a genuine `Info.plist`,
    and this project still never synthesizes one from scratch (see
    0.3.0's note below). But when the project directory has a real
    `Info.plist`, or its own `Info-plist.txt` placeholder (promoted to a
    real, functional one even without renaming - configuring a File
    Association at all is already an explicit opt-in to needing a real
    plist), macpkg merges a `CFBundleDocumentTypes` array into a copy of
    it automatically; an author's own hand-written entry there is always
    left untouched, never fought or duplicated. With neither file present
    there's still nothing to merge into, so macpkg logs a clear progress
    note instead of silently no-op'ing or synthesizing one out of thin
    air. A project with no File Associations configured keeps the exact
    same "no Info.plist unless deliberately provided" behavior as before
  - Setup.exe (NSIS): writes/removes plain `HKCR` registry entries
    (ProgID, description, DefaultIcon, `shell\open\command`) - both
    author-time only, no Components-page opt-out, since an association a
    user didn't ask to skip is harmless, unlike Desktop shortcut/autostart
  - .msi (wixl): a `<ProgId>` as a `<Component>` child, `<Extension>`/
    `<Verb>` nested inside it, plus a hand-written `DefaultIcon`
    `<RegistryValue>` - both found the hard way by testing minimal `.wxs`
    files directly against a real `wixl` compile before writing the real
    template: `<Extension>` nested directly under `<File>` crashes wixl
    outright (`unhandled child File node Extension`), and `ProgId`'s own
    `Icon`/`IconIndex` attributes are silently accepted by the XML parser
    but never actually written to the compiled `.msi`'s Registry table (a
    GObject property warning at compile time is the only clue). Verified
    against a real compiled `.msi`'s Registry/Component/FeatureComponents
    tables via `msiinfo export`, not just template-level testing
  - .deb/.rpm: installs a real shared-mime-info package
    (`/usr/share/mime/packages/<name>.xml`) and adds `MimeType=`/a
    trailing `%f` to the `.desktop` entry's `Exec=` line, so the app shows
    up in "Open With" and can receive a double-clicked file's path as an
    argument; `update-mime-database` is refreshed best-effort on install/
    removal alongside the existing icon-cache refresh from 0.4.0
  - Windows ARM64 caveat below still applies to `.msi` here too - no new
    limitation, just inherited
- "Import Existing Config" (both the GUI dialog and the CLI's `import`
  command) now also recognizes an existing Inno `[Registry]`-based file
  association and proposes it for import - both real-world `.iss` shapes
  are understood: the modern Inno-wizard-generated
  `Software\Classes\.ext\OpenWithProgids` indirection, and the older,
  simpler direct `HKCR\.ext` form some hand-written scripts use instead.
  A candidate is only proposed once a matching `<ProgID>\shell\open\command`
  line confirms it's a real, launchable association, not just a
  coincidentally-shaped registry entry; re-running import is safe and
  won't duplicate an extension already present. Verified against this
  project's own real, committed `Inno/KrankyBearInstallerBear.iss` as
  well as synthetic fixtures for both shapes
- Explicitly out of scope this round (see "Future Ideas" above): custom
  installer wizard pages (no concrete design yet), and importing the
  remaining `[Icons]/[Tasks]/[Run]/[UninstallRun]/[UninstallDelete]` Inno
  sections (only `[Registry]`-based file associations are wired up so far)

## Version 0.4.0 - September 12, 2026

- Real Linux `.desktop`-file generation for `.deb`/`.rpm`, closing the first
  item in the "Future Ideas" list. Both formats now always install a real
  app-menu entry at `/usr/share/applications/<name>.desktop` - unconditional,
  not gated behind `install_experience.desktop_shortcut` the way Windows'
  Desktop shortcut toggle is, since a `.desktop` file is how a Linux app
  appears in the launcher at all on every major desktop environment (most
  don't even have a separate literal "desktop icon" concept anymore), the
  same reasoning as Windows' always-created Start Menu shortcut. Driven by
  `Linux.DesktopCategories`/`DesktopComment` (both already existed in the
  schema since 0.2.0, unused by any backend until now); when
  `Identity.Icons.PNG` is set, it's also installed into the hicolor icon
  theme (`/usr/share/icons/hicolor/256x256/apps/`) and referenced by name
  from the `.desktop` entry's `Icon=` line. `update-desktop-database`/
  `gtk-update-icon-cache` are refreshed best-effort on install and removal
  (via nfpm's previously-unused PostInstall script slot, and appended after
  the project's own `Hooks.PostUninstall` on removal rather than replacing
  it) so the entry/icon show up without needing a logout - both silently
  skip on a minimal/headless box that doesn't have them. Categories are
  auto-normalized to the freedesktop-required semicolon-terminated form
  (`Utility` -> `Utility;`) so a value typed by hand in the Identity tab
  doesn't need to remember that convention. Deliberately left for later,
  not solved this round: `install_experience.autostart_at_login` still has
  no effect on Linux - the only real mechanism there (`/etc/xdg/autostart`)
  is system-wide, affecting every user on the machine, unlike Windows' own
  per-user HKCU Run key, a genuine semantic mismatch worth solving
  separately rather than papering over
- New Windows "Install for" choice: All users (the existing default -
  requires admin elevation, installs to `Program Files`/
  `ProgramFiles64Folder`, Programs & Features under `HKLM`) or Current user
  only (no elevation needed at all, installs to `%LOCALAPPDATA%`/
  `LocalAppDataFolder` instead, and Programs & Features moves to `HKCU`
  since a non-elevated process can't write `HKLM`). Author-time-only on
  both `Setup.exe` and `.msi` - confirmed neither backend can offer this as
  a real end-user runtime pick without extra cost: `wixl`'s bundled UI has
  no `WixUI_Advanced`/`InstallScopeDlg` equivalent (checked the installed
  msitools build's own bundled `ext/ui` directory - only `WixUI_Minimal` is
  there), and a real NSIS equivalent would need bundling the external UAC
  plugin to elevate only after the choice is made. On `Setup.exe`, current-
  user also switches `RequestExecutionLevel` from `admin` to `user` and
  drops the `SetShellVarContext all` call that redirects the Start
  Menu/Desktop shortcuts to the all-users ones (NSIS's own default context
  is already per-user, so nothing extra is needed there). Defaults to All
  users - every existing project's behavior is unchanged unless this is
  explicitly opted into. Confirmed on real Windows hardware: Setup.exe
  correctly prompts for elevation (UAC) when set to All users, and installs
  with no elevation prompt at all when set to Current user only
- New `output.post_build_hook`: inline shell script text that runs once, on
  this build machine (not the target machine - unlike `hooks.pre_install`/
  `post_uninstall`, which are baked into the installer package itself and
  run later, elsewhere), right after every requested target/arch has
  finished building. Only runs when the whole build succeeded - skipped
  with a clear progress note otherwise, since acting on "all the artifacts"
  rarely makes sense with some missing. Every successfully built artifact's
  path is handed to the script as an `INSTALLERBEAR_OUTPUT_<TARGET>`
  environment variable (or `INSTALLERBEAR_OUTPUT_<TARGET>_<ARCH>` when a
  target built more than one arch, since a single bare name couldn't hold
  more than one path), plus `INSTALLERBEAR_ARTIFACTS` (all of them,
  space-separated) and a few identity basics. A general escape hatch for
  things this tool doesn't implement natively - uploading to GitHub
  Releases, code signing, notarization, and so on - rather than growing a
  dedicated feature for each one; a new project idea, not previously on the
  "Future Ideas" list. Runs the script file directly (not `/bin/sh <path>`)
  so a custom shebang is honored, the same convention a real `.deb`/`.rpm`
  maintainer script already gets from `dpkg`/`rpm` itself. Verified with a
  real end-to-end test: `Run()` actually executes the hook as a subprocess
  and confirms it received the right environment variable for a real
  build's output path, not just a unit test of the env-var-building logic
  in isolation
- The Identity tab gained a new **Hooks** section (`pre_install`/
  `post_uninstall`, run on the target machine by the installed macOS
  `.pkg`/Linux `.deb`/`.rpm` package) and a **Post-build hook** field in the
  Output section (`output.post_build_hook`, run immediately on this build
  machine) - all three previously had to be typed directly into
  installerbear.yaml by hand, with no GUI exposure at all. Multi-line text
  entries, since real shell script text needs more than one line to be
  usable to read/type. Also updated the in-app Help text: mentioned the
  new Hooks fields, corrected two stale "Known Limitations" bullets
  (unsaved-changes confirmation and inline Payload editing were both
  already shipped, just never removed from that list) and a stale
  "Check Help → Check for Updates for release notes" line (the real
  Release Notes menu item didn't exist yet when that was written), and
  added a Windows ARM64 .msi limitation note
- Confirmed empirically (not assumed) that `wixl` 0.106 - the tool this
  project uses for `.msi` - has no ARM64 support at all: every arch string
  it recognizes (`x86`/`x64`/`intel`/`intel64`) is x86-family only;
  `arm64`/`aarch64`/`amd64` all hard-error with "arch of type 'X' is not
  supported". `Setup.exe`/NSIS has no such limitation (it doesn't encode
  target CPU architecture into the installer format the way MSI does), so
  it should already work for a real `windows/arm64` binary with zero code
  changes - untested on real ARM64 Windows hardware. Logged as a known
  limitation rather than "fixed", since this is a genuine tool constraint,
  not a bug in this project's own code
- Fixed a real bug in the Release Notes viewer's lazy-loading, found via
  Allan's own hands-on testing (a fresh binary, not `go run .`): the
  content would finish loading in the background correctly, but never
  actually appear on screen until something else happened - closing and
  reopening the window, or clicking the Release Notes menu item a second
  time while it was already open (both times the real content then showed
  up "almost instantly", since it had already finished loading, just never
  got painted). Root cause, confirmed by reading Fyne's own source rather
  than guessed: `scroll.Content = richText; scroll.Refresh()` only
  recomputes the Scroll widget's own layout - it never touches the
  canvas's dirty flag, which is what the 60Hz paint loop actually gates a
  repaint on. Fixed by also calling
  `releaseNotesWindow.Canvas().Refresh(scroll)` right after -
  `fyne.Canvas.Refresh(CanvasObject)` is the real, public API for "this
  object's content changed, please repaint it", confirmed against Fyne
  v2.8.1's own internal canvas/driver source. A real lesson for any future
  in-place content swap in this codebase: a widget's own `.Refresh()`
  doesn't guarantee a repaint when you've mutated its content out from
  under an already-rendered tree - the canvas's own `Refresh(obj)` does

## Version 0.3.0 - September 12, 2026

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
    - same story as the macpkg Info.plist assumption later this version).
    Opting this in without a real license set still pulls in
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

## Version 0.2.0 - September 11, 2026

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

## Version 0.1.0 - September 03, 2026

**✨ NEW Cross-platform installer builder**

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
