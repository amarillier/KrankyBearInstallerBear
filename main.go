package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/systray"

	"installerbear/internal/startup"
)

const (
	// appName    = "KrankyBear InstallerBear"
	appVersion = "0.3.0" // see FyneApp.toml
	appAuthor  = "Allan Marillier"
	appID      = "com.github.amarillier.KrankyBearInstallerBear"
)

var appName = "KrankyBear InstallerBear"
var appCopyright = buildCopyrightNotice()

// mainEditor is the app's single project-editing session. A package-level
// var (matching this file's existing appName/appCopyright convention)
// rather than threading it through every menu/tray callback's signature —
// buildMenu, setupSystemTray, and quitApp all need it.
var mainEditor *editor

func buildCopyrightNotice() string {
	const startYear = 2026
	currentYear := time.Now().Year()
	if currentYear <= startYear {
		return "Copyright (c) Allan Marillier, 2026"
	}
	return fmt.Sprintf("Copyright (c) Allan Marillier, 2026-%d", currentYear)
}

func main() {
	// Headless subcommands (build/validate/doctor/list-targets) dispatch
	// BEFORE any Fyne/GLFW call — including the flag.Parse below, since
	// os.Args[1] here is a subcommand name, not a GUI flag. GLFW allows only
	// one Init/Terminate cycle per process (shared with Fyne's own driver;
	// see CLAUDE.md's Mesa3D section), and a display-less CI runner has no
	// hardware OpenGL to probe in the first place, so the CLI path must
	// never reach app.NewWithID or the Mesa fallback probe at all.
	if len(os.Args) > 1 && isHelpFlag(os.Args[1]) {
		printCLIUsage()
		os.Exit(0)
	}
	if isCLICommand(os.Args[1:]) {
		os.Exit(runCLI(os.Args[1:]))
	}

	// On Linux, a headless server or an SSH session with no display
	// forwarding has neither DISPLAY nor WAYLAND_DISPLAY set — GLFW can't
	// do anything useful there, and previously failed with a raw
	// "NotInitialized" panic and a stack trace (confirmed on a real
	// headless Ubuntu box) instead of an actionable message. Same check,
	// same message pattern as this project's sibling TaniumSensorExplorer.
	if !guiDisplayAvailable() {
		fmt.Fprintf(os.Stderr, "Cannot start the desktop interface: no graphical display detected.\n"+
			"On Linux, set DISPLAY (X11) or WAYLAND_DISPLAY (Wayland), or reconnect with SSH display forwarding (ssh -X/-Y).\n"+
			"Use the command-line interface instead:\n\n")
		printCLIUsage()
		os.Exit(0)
	}

	langFlag := flag.String("lang", "", "UI language code (e.g. en, de); overrides the saved preference for this run")
	mesaFallbackFlag := flag.Bool(startup.MesaFallbackFlagName, false, "internal: relaunch flag for the Mesa3D OpenGL fallback (Windows only)")
	flag.Parse()

	// Windows only (no-op elsewhere): prefer real hardware OpenGL, falling
	// back to the bundled Mesa3D software renderer (relaunching once) only
	// if the hardware probe fails. Must run before app.NewWithID — GLFW
	// only supports one Init/Terminate cycle per process, shared with
	// Fyne's own driver (see internal/startup/opengl.go, and CLAUDE.md's
	// "Mesa3D OpenGL fallback" section).
	startup.EnsureWindowsOpenGLReady(*mesaFallbackFlag)

	a := app.NewWithID(appID)
	a.SetIcon(resourceKrankyBearInstallerBearPng)
	setupI18n(a, *langFlag) // load message catalog + resolve UI language before building any UI
	loadTheme(a)

	win := a.NewWindow(appName)
	win.SetIcon(resourceKrankyBearInstallerBearPng)
	win.Resize(mainWindowLaunchSize(a)) // restore previous size (size only - Fyne can't restore position)

	mainEditor = newEditor(a, win)
	win.SetContent(mainEditor.content())

	win.SetMainMenu(buildMenu(a, win))
	setupSystemTray(a, win)

	// Closing the window quits the app. Deferred via fyne.Do so quit() runs on a
	// clean loop iteration outside whatever callback triggered it — quitting
	// directly from inside a menu-item click or close-intercept callback can hang
	// on Windows (see CLAUDE.md "Quitting cleanly").
	win.SetCloseIntercept(func() { fyne.Do(func() { quitApp(a, win) }) })

	checkForUpdatesAuto(a) // quiet, once-per-day check; dialog only if an update exists

	win.ShowAndRun()
}

// ── Window geometry ──────────────────────────────────────────────────────────
// Fyne has no cross-platform window position/display restore, so only size is
// persisted (see CLAUDE.md "Window size persistence").

const (
	prefWinWidth  = "mainWindowWidth"
	prefWinHeight = "mainWindowHeight"
	minWinWidth   = 400
	minWinHeight  = 300
	maxWinDim     = 8000
	defaultWinW   = 900
	defaultWinH   = 650
)

func mainWindowLaunchSize(a fyne.App) fyne.Size {
	w := a.Preferences().FloatWithFallback(prefWinWidth, defaultWinW)
	h := a.Preferences().FloatWithFallback(prefWinHeight, defaultWinH)
	if w < minWinWidth || w > maxWinDim {
		w = defaultWinW
	}
	if h < minWinHeight || h > maxWinDim {
		h = defaultWinH
	}
	return fyne.NewSize(float32(w), float32(h))
}

func saveMainWindowGeometry(a fyne.App, win fyne.Window) {
	size := win.Canvas().Size()
	a.Preferences().SetFloat(prefWinWidth, float64(size.Width))
	a.Preferences().SetFloat(prefWinHeight, float64(size.Height))
}

// quitApp does teardown in the order CLAUDE.md calls out: stop background
// work first, then persist geometry, then quit. A running build is exactly
// the "background work" this repo's CLAUDE.md warns must be stopped before
// Quit() — an in-progress packager.Run holds live *exec.Cmd subprocesses
// (makensis/wixl/pkgbuild) that would otherwise be orphaned.
func quitApp(a fyne.App, win fyne.Window) {
	if mainEditor != nil && mainEditor.build.IsRunning() {
		dialog.ShowConfirm("Build in progress", "Cancel the running build and quit?", func(confirmed bool) {
			if !confirmed {
				return
			}
			mainEditor.build.Cancel()
			saveMainWindowGeometry(a, win)
			a.Quit()
		}, win)
		return
	}
	saveMainWindowGeometry(a, win)
	a.Quit()
}

// ── Menu + tray (mirror each other; see CLAUDE.md "System tray + main menu") ──

func buildMenu(a fyne.App, win fyne.Window) *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Project", func() { mainEditor.newProject() }),
		fyne.NewMenuItem("New Sample Project...", func() { mainEditor.newSampleProject() }),
		fyne.NewMenuItem("Open Project...", func() { mainEditor.openProject() }),
		fyne.NewMenuItem("Save Project", func() { mainEditor.saveProject() }),
		fyne.NewMenuItem("Save Project As...", func() { mainEditor.saveProjectAs() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import Existing Config...", func() { mainEditor.importExistingConfig() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() { fyne.Do(func() { quitApp(a, win) }) }),
	)
	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItem("Show All Windows", func() { bringAllAppWindowsToFront(a, win) }),
		fyne.NewMenuItem("Hide All Windows", func() { hideAllAppWindows(a) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Light Theme", func() { setLightTheme(a) }),
		fyne.NewMenuItem("Dark Theme", func() { setDarkTheme(a) }),
		fyne.NewMenuItem("System Theme", func() { setSystemTheme(a) }),
	)
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("Help", func() { showHelp(a) }),
		fyne.NewMenuItem("Release Notes", func() { showReleaseNotes(a) }),
		fyne.NewMenuItem("Check for Updates", func() { checkForUpdatesManual(a) }),
		fyne.NewMenuItem("About", func() { showAbout(a) }),
	)
	return fyne.NewMainMenu(fileMenu, viewMenu, helpMenu)
}

// bringAllAppWindowsToFront shows every window Fyne's driver currently holds
// for this app (main window plus any About/Help/Update window that's been
// created, even if hidden) and focuses the main window last so the whole
// stack rises together. Mirrors ../TaniumMigrator's menu.go — note the same
// trade-off it accepts: Fyne doesn't destroy a window object on Hide, only
// on Close, so this can also re-show a secondary window the user explicitly
// closed earlier in the session, not just the ones open when Hide-all ran.
func bringAllAppWindowsToFront(a fyne.App, mainWin fyne.Window) {
	for _, win := range a.Driver().AllWindows() {
		if win != nil {
			win.Show()
		}
	}
	if mainWin != nil {
		mainWin.RequestFocus()
	}
}

// hideAllAppWindows hides every window Fyne's driver currently holds for
// this app, mirroring bringAllAppWindowsToFront so Hide/Show-all act on the
// same set.
func hideAllAppWindows(a fyne.App) {
	for _, win := range a.Driver().AllWindows() {
		if win != nil {
			win.Hide()
		}
	}
}

// setupSystemTray mirrors the main menu. Tray callbacks fire off the main
// goroutine, so every body is wrapped in fyne.Do (CLAUDE.md "fyne.Do is
// mandatory").
func setupSystemTray(a fyne.App, win fyne.Window) {
	desk, ok := a.(desktop.App)
	if !ok {
		return // not a desktop driver
	}
	menu := fyne.NewMenu(appName,
		fyne.NewMenuItem("Show All Windows", func() { fyne.Do(func() { bringAllAppWindowsToFront(a, win) }) }),
		fyne.NewMenuItem("Hide All Windows", func() { fyne.Do(func() { hideAllAppWindows(a) }) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("New Project", func() { fyne.Do(func() { mainEditor.newProject() }) }),
		fyne.NewMenuItem("New Sample Project...", func() { fyne.Do(func() { mainEditor.newSampleProject() }) }),
		fyne.NewMenuItem("Open Project...", func() { fyne.Do(func() { mainEditor.openProject() }) }),
		fyne.NewMenuItem("Save Project", func() { fyne.Do(func() { mainEditor.saveProject() }) }),
		fyne.NewMenuItem("Save Project As...", func() { fyne.Do(func() { mainEditor.saveProjectAs() }) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import Existing Config...", func() { fyne.Do(func() { mainEditor.importExistingConfig() }) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Light Theme", func() { fyne.Do(func() { setLightTheme(a) }) }),
		fyne.NewMenuItem("Dark Theme", func() { fyne.Do(func() { setDarkTheme(a) }) }),
		fyne.NewMenuItem("System Theme", func() { fyne.Do(func() { setSystemTheme(a) }) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Help", func() { fyne.Do(func() { showHelp(a) }) }),
		fyne.NewMenuItem("Release Notes", func() { fyne.Do(func() { showReleaseNotes(a) }) }),
		fyne.NewMenuItem("Check for Updates", func() { checkForUpdatesManual(a) }),
		fyne.NewMenuItem("About", func() { fyne.Do(func() { showAbout(a) }) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() { fyne.Do(func() { quitApp(a, win) }) }),
	)
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(resourceKrankyBearInstallerBearPng)

	// Hover tooltip on the tray icon (Windows/macOS; no-op on Linux) --
	// desktop.App has no tooltip setter, but fyne.io/systray (what Fyne's
	// own driver already uses internally for the tray icon) does. Deferred
	// slightly since, unlike SetSystemTrayIcon above, a raw systray.SetTooltip
	// call has no built-in retry/caching if the tray isn't fully ready yet.
	// Same pattern as ../TaniumMigrator and ../KrankyBearCommander.
	time.AfterFunc(300*time.Millisecond, func() {
		systray.SetTooltip(appName)
	})
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
