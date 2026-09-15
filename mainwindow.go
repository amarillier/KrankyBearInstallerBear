package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"installerbear/internal/buildstate"
	"installerbear/internal/packproject"
)

// editor is InstallerBear's whole GUI: one project (identity, binaries, payload,
// targets) edited across tabs, plus the build state shared with the
// Build tab's Start/Cancel buttons. Every widget that needs repopulating
// after New/Open is tracked here so refreshAll can push proj's values into
// them in one place.
type editor struct {
	app fyne.App
	win fyne.Window

	proj *packproject.Project
	path string // "" if this project has never been saved

	// lastSavedSnapshot is proj's own YAML encoding as of the last
	// New/Open/Save — see projectio.go's isDirty/markSaved. Comparing a
	// fresh encoding against this is simpler and far less error-prone than
	// threading a "mark dirty" call through every OnChanged callback across
	// every tab (and SetText during a refresh already fires those same
	// callbacks harmlessly — see refreshAll's own comment — so a live dirty
	// flag would need to tell a real edit apart from that, which this
	// snapshot comparison sidesteps entirely).
	lastSavedSnapshot []byte

	build buildstate.State

	// Identity tab
	nameEntry, idEntry, versionEntry                                     *widget.Entry
	publisherEntry, vendorEntry                                          *widget.Entry
	urlEntry, descEntry                                                  *widget.Entry
	licenseNameEntry, licenseEntry                                       *widget.Entry
	icoEntry, icnsEntry, pngEntry                                        *widget.Entry
	pngIconThumbnail                                                     *canvas.Image
	guidEntry, exeNameEntry                                              *widget.Entry
	installScopeSelect                                                   *widget.Select
	launchAfterInstallCheck, desktopShortcutCheck, autostartAtLoginCheck *widget.Check
	macExecEntry, macMinOSEntry                                          *widget.Entry
	macCategoryEntry                                                     *widget.Entry
	linuxCategoriesEntry, linuxCommentEntry                              *widget.Entry
	preInstallHookEntry, postUninstallHookEntry                          *widget.Entry
	outputDirEntry, postBuildHookEntry                                   *widget.Entry

	// Binaries tab — one field per (OS, arch) pair; see binariesform.go.
	binaryFields []binaryField

	// Payload tab
	payloadTable *widget.Table
	// payloadSelected tracks the Select column's own checked state per row
	// (row index -> checked) - NOT part of packproject.Project, purely
	// which rows Remove/Edit... currently act on. widget.Table's own
	// row-selection (OnSelected/OnUnselected) never fires for a cell that
	// contains its own interactive widget (every cell here does - Fyne's
	// hit-testing dispatches a click to the deepest matching object under
	// the pointer, which is always the cell's Entry/Check, not the Table
	// itself), so a dedicated checkbox column is the real fix, not a
	// tweak to that mechanism - see payloadtable.go's own comment on this.
	payloadSelected map[int]bool
	// payloadSortCol/payloadSortAsc track which Payload column (if any) the
	// table is currently sorted by - clicking a sortable header (Source,
	// Dest, OS) reorders e.proj.Payload itself (not just the view), since
	// comment-preservation (see internal/packproject/comments.go) matches
	// entries by content rather than position, so a hand-typed # comment
	// stays attached to its entry across a sort. -1 means unsorted (rows
	// stay in whatever order they were added/loaded in).
	payloadSortCol int
	payloadSortAsc bool

	// File Associations tab
	fileAssocTable *widget.Table
	// fileAssocSelected mirrors payloadSelected above, same reason.
	fileAssocSelected map[int]bool

	// Build tab
	targetChecks          map[string]*widget.Check
	targetStatus          map[string]*widget.Label
	logEntry              *widget.Entry
	startBtn              *widget.Button
	cancelBtn             *widget.Button
	buildStatus           *widget.Label
	cleanOldVersionsCheck *widget.Check
	tabs                  *container.AppTabs
}

// newEditor constructs the editor around a fresh, blank project — see
// projectio.go's newProject for the menu-driven equivalent that also resets
// path and refreshes every widget.
func newEditor(a fyne.App, win fyne.Window) *editor {
	e := &editor{
		app:  a,
		win:  win,
		proj: &packproject.Project{},
	}
	e.targetChecks = make(map[string]*widget.Check)
	e.targetStatus = make(map[string]*widget.Label)
	e.payloadSelected = make(map[int]bool)
	e.fileAssocSelected = make(map[int]bool)
	e.payloadSortCol = -1
	return e
}

// content builds every tab's widgets (wiring their OnChanged callbacks to
// write straight into e.proj — see identityform.go's comment on why) and
// returns the assembled AppTabs. Called once, right after newEditor.
func (e *editor) content() fyne.CanvasObject {
	e.tabs = container.NewAppTabs(
		container.NewTabItem("Identity", e.buildIdentityTab()),
		container.NewTabItem("Binaries", e.buildBinariesTab()),
		container.NewTabItem("Payload", e.buildPayloadTab()),
		container.NewTabItem("File Associations", e.buildFileAssociationsTab()),
		container.NewTabItem("Build", e.buildBuildTab()),
	)
	e.refreshAll()
	e.markSaved()
	return container.NewBorder(e.buildToolbar(), nil, nil, nil, e.tabs)
}

// buildToolbar gives quick-access buttons for the File menu's project
// actions, so New/Open/Save/Save As don't require opening the menu every
// time. Plain buttons in an HBox, matching this codebase's existing
// "toolbar" idiom (see payloadtable.go/buildpanel.go) rather than
// widget.Toolbar's different icon-only look.
func (e *editor) buildToolbar() fyne.CanvasObject {
	newBtn := widget.NewButtonWithIcon("New", theme.DocumentCreateIcon(), func() { e.newProject() })
	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() { e.openProject() })
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() { e.saveProject() })
	saveAsBtn := widget.NewButton("Save As...", func() { e.saveProjectAs() })
	return container.NewHBox(newBtn, openBtn, saveBtn, saveAsBtn)
}

// refreshAll pushes e.proj's current values into every widget. Called after
// New/Open replace e.proj wholesale; SetText below also fires each widget's
// own OnChanged, which just writes the same value straight back into
// e.proj — harmless, and simpler than trying to suppress it.
func (e *editor) refreshAll() {
	// Row indices in these maps only ever mean anything relative to
	// whatever e.proj.Payload/FileAssociations looked like when they were
	// set - every caller of refreshAll has just replaced or bulk-merged
	// e.proj wholesale (New/Open/New Sample Project/Import), so any
	// leftover selection state from before is stale and must not survive.
	e.payloadSelected = make(map[int]bool)
	e.fileAssocSelected = make(map[int]bool)
	e.payloadSortCol = -1

	e.refreshIdentityTab()
	e.refreshBinariesTab()
	e.refreshPayloadTab()
	e.refreshFileAssociationsTab()
	e.refreshBuildTab()
}

// windowTitleForProject reflects the project name and unsaved-changes/path
// state in the title bar, mirroring how most desktop editors show "what am
// I looking at" without a dedicated status field.
func (e *editor) updateWindowTitle() {
	name := e.proj.Identity.Name
	if name == "" {
		name = "Untitled Project"
	}
	title := appName + " - " + name
	if e.path == "" {
		title += " (unsaved)"
	}
	e.win.SetTitle(title)
}
