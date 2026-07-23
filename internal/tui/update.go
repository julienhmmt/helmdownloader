package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// Update is the Bubble Tea message dispatcher.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = typed.Width, typed.Height
		m.results.SetSize(typed.Width-2, typed.Height-6)
		m.versions.SetSize(typed.Width-2, typed.Height-6)
		m.progress.SetWidth(max(0, min(typed.Width-4, 60)))
		return m, nil
	case tea.BackgroundColorMsg:
		// Always record host darkness; only auto applies it immediately.
		// (Named themes keep their fixed palette, but detection is stored so
		// switching back to auto does not use a stale preview value.)
		m.detectedIsDark = typed.IsDark()
		if config.ThemeIsForced(m.cfg.Theme) {
			return m, nil
		}
		m.applyTheme()
		m.bgKnown = true
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(typed)
	case searchResultMsg:
		// Stale if the user cancelled search and already moved on.
		if m.state != stateSearching {
			return m, nil
		}
		m.state = stateResults
		m.errStep = ""
		m.allPackages = typed.packages
		m.filterField = filterNone
		m.filterValue = ""
		m.refreshResults()
		if len(typed.packages) == 0 {
			m.setStatus("No charts found. Try a different query.")
		} else {
			m.clearStatus()
		}
		return m, nil
	case versionsMsg:
		if m.state != stateSearching {
			return m, nil
		}
		m.state = stateVersions
		m.errStep = ""
		m.versions.SetItems(versionsToItems(typed.versions))
		if len(typed.versions) == 0 {
			m.setStatus("No versions returned for this chart.")
		} else {
			m.clearStatus()
		}
		return m, nil
	case preparedMsg:
		if m.state != statePreparing {
			return m, nil
		}
		m.prepared = typed.prepared
		m.reviewImages = typed.prepared.Images
		m.reviewCursor = 0
		m.reviewOffset = 0
		m.reviewWarnAck = false
		m.clearStatus()
		m.state = stateReview
		m.errStep = ""
		// Export the discovered set first so -export-images always reflects
		// chart rendering, even when -import-images will replace the list.
		if err := exportImages(m.cfg.ExportImages, m.reviewImages); err != nil {
			m.err = err
			m.errStep = "prepare"
			m.state = stateError
			return m, nil
		}
		// Apply an approved list once when entering review so the operator
		// sees (and can edit) it before download. Re-importing on every Enter
		// would wipe space/d edits after a deprecated/prerelease ack.
		if m.cfg.ImportImages != "" {
			imported, err := importImages(m.cfg.ImportImages)
			if err != nil {
				m.err = err
				m.errStep = "prepare"
				m.state = stateError
				return m, nil
			}
			if len(imported) > 0 {
				m.reviewImages = imported
			}
		}
		return m, nil
	case progressMsg:
		if m.state != stateDownloading {
			return m, nil
		}
		m.downCurrent, m.downTotal = typed.current, typed.total
		delete(m.imageProgress, typed.ref)
		return m, waitForActivity(m.activity)
	case byteProgressMsg:
		if m.state != stateDownloading {
			return m, nil
		}
		m.imageProgress[typed.ref] = imageProgress{written: typed.written, total: typed.total}
		return m, waitForActivity(m.activity)
	case downloadDoneMsg:
		// Ignore completion after the user cancelled download and left the screen.
		if m.state != stateDownloading {
			return m, nil
		}
		m.entries = append(m.entries, typed.entries...)
		m.failures = typed.failures
		m.errStep = ""
		if len(typed.failures) == 0 {
			m.state = stateBundling
			m.errStep = "bundle"
			return m, tea.Batch(m.spinner.Tick,
				bundleCmd(m.pipeline, m.prepared, m.selectedPkg, m.selectedVersion, m.entries))
		}
		m.state = stateDownloadReview
		return m, nil
	case doneMsg:
		if m.state != stateBundling {
			return m, nil
		}
		m.bundlePath = typed.bundlePath
		m.state = stateDone
		m.errStep = ""
		return m, nil
	case errMsg:
		// Drop errors that arrive after cancel already left the busy state that
		// produced them (e.g. context.Canceled from a cancelled search/download).
		switch m.state {
		case stateSearching, statePreparing, stateDownloading, stateBundling:
			// Accept — still on the busy screen that issued the work.
		default:
			return m, nil
		}
		m.err = typed.err
		m.state = stateError
		return m, nil
	}
	return m.updateComponents(msg)
}

// updateComponents forwards a message to the focused sub-component and keeps
// the spinner animating.
func (m model) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0, 2)
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	switch m.state {
	case stateSearch:
		m.search, cmd = m.search.Update(msg)
	case stateAddImage:
		m.addInput, cmd = m.addInput.Update(msg)
	case stateFilterInput:
		m.filter, cmd = m.filter.Update(msg)
	case stateResults:
		m.results, cmd = m.results.Update(msg)
	case stateVersions:
		m.versions, cmd = m.versions.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// handleKey routes key presses based on the active screen.
func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.cancel()
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir), tea.Quit)
	}
	// Global theme menu — ctrl+t so it works even while typing in search /
	// filter / add-image fields and never collides with single-letter bindings.
	// Re-pressing while already in the menu is a no-op (use Esc to cancel).
	if msg.String() == "ctrl+t" {
		if m.state == stateThemeMenu {
			return m, nil
		}
		// Do not interrupt in-flight busy work with a palette picker.
		switch m.state {
		case stateSearching, statePreparing, stateDownloading, stateBundling:
			return m, nil
		}
		m.openThemeMenu()
		return m, nil
	}
	switch m.state {
	case stateSearch:
		return m.handleSearchKey(msg)
	case stateResults:
		return m.handleResultsKey(msg)
	case stateFilterInput:
		return m.handleFilterInputKey(msg)
	case stateVersions:
		return m.handleVersionsKey(msg)
	case stateReview:
		return m.handleReviewKey(msg)
	case stateAddImage:
		return m.handleAddImageKey(msg)
	case stateDownloadReview:
		return m.handleDownloadReviewKey(msg)
	case stateThemeMenu:
		return m.handleThemeMenuKey(msg)
	case stateSearching, statePreparing, stateDownloading, stateBundling:
		return m.handleBusyKey(msg)
	case stateDone, stateError:
		return m.handleEndKey(msg)
	}
	return m.updateComponents(msg)
}

// handleThemeMenuKey navigates the theme picker: j/k or arrows move (with live
// preview), Enter confirms, Esc restores the prior theme.
func (m model) handleThemeMenuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, m.cancelThemeMenu()
	case "enter":
		return m, m.confirmThemeMenu()
	case "up", "k":
		if m.themeMenuCursor > 0 {
			m.themeMenuCursor--
			m.previewThemeAtCursor()
		}
		return m, nil
	case "down", "j":
		if m.themeMenuCursor < len(config.ThemeMenu)-1 {
			m.themeMenuCursor++
			m.previewThemeAtCursor()
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6":
		// Digit jump: 1-based index into ThemeMenu.
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(config.ThemeMenu) {
			m.themeMenuCursor = idx
			m.previewThemeAtCursor()
		}
		return m, nil
	}
	return m, nil
}

// handleBusyKey cancels long-running ops without quitting the process.
// Bundle has no context, so Esc is a no-op during bundling; ctrl+c still quits.
func (m model) handleBusyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "esc" {
		return m.updateComponents(msg)
	}
	switch m.state {
	case stateSearching:
		m.cancel()
		m.ctx, m.cancel = context.WithCancel(context.Background())
		// Prefer results if packages are already loaded (versions fetch also uses
		// stateSearching); otherwise return to the search prompt.
		if len(m.allPackages) > 0 {
			m.state = stateResults
		} else {
			m.state = stateSearch
		}
		m.errStep = ""
		return m, nil
	case statePreparing:
		m.cancel()
		m.ctx, m.cancel = context.WithCancel(context.Background())
		cleanup := cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir)
		if m.selectedPkg.Name != "" {
			m.state = stateVersions
		} else {
			m.state = stateSearch
		}
		m.errStep = ""
		return m, cleanup
	case stateDownloading:
		m.cancel()
		m.ctx, m.cancel = context.WithCancel(context.Background())
		m.imageProgress = map[string]imageProgress{}
		m.errStep = ""
		if len(m.entries) > 0 {
			m.state = stateDownloadReview
		} else {
			m.state = stateReview
		}
		return m, nil
	case stateBundling:
		// Bundle ignores ctx; cancelling mid-write risks a partial archive.
		return m, nil
	}
	return m, nil
}

// handleSearchKey processes input on the search screen.
func (m model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		query := m.search.Value()
		if query == "" {
			return m, nil
		}
		m.state = stateSearching
		m.errStep = "search"
		return m, tea.Batch(m.spinner.Tick, searchCmd(m.ctx, m.client, query, m.cfg.SearchLimit))
	case "esc":
		m.cancel()
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir), tea.Quit)
	}
	return m.updateComponents(msg)
}

// handleResultsKey processes selection and sort/filter on the results screen.
func (m model) handleResultsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.results.FilterState() == list.Filtering { // let the list consume keys while filtering
		return m.updateComponents(msg)
	}
	switch msg.String() {
	case "esc":
		m.state = stateSearch
		return m, nil
	case "enter":
		item, ok := m.results.SelectedItem().(packageItem)
		if !ok {
			return m, nil
		}
		m.selectedPkg = item.pkg
		m.state = stateSearching
		m.errStep = "search"
		return m, tea.Batch(m.spinner.Tick, versionsCmd(m.ctx, m.client, item.pkg))
	case "s": // cycle sort field: stars → name → updated → stars
		m.sortField = cycleSortField(m.sortField)
		m.refreshResults()
		return m, nil
	case "o": // toggle sort direction asc/desc
		m.sortDir = toggleSortDir(m.sortDir)
		m.refreshResults()
		return m, nil
	case "f": // cycle filter field: off → author → company → off
		m.filterField = cycleFilterField(m.filterField)
		m.filterValue = ""
		m.refreshResults()
		return m, nil
	case "F": // open the filter substring input
		if m.filterField == filterNone {
			m.filterField = filterAuthor // default to author when opening
		}
		m.filter.SetValue(m.filterValue)
		m.filter.Focus()
		m.state = stateFilterInput
		return m, nil
	case "tab": // cycle through unique values for the active filter field
		m.cycleFilterValue()
		m.refreshResults()
		return m, nil
	}
	return m.updateComponents(msg)
}

// handleFilterInputKey processes the filter substring input screen.
func (m model) handleFilterInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filterValue = m.filter.Value()
		m.filter.Blur()
		m.state = stateResults
		m.refreshResults()
		return m, nil
	case "esc":
		m.filter.Blur()
		m.state = stateResults
		return m, nil
	case "tab":
		m.cycleFilterValue()
		m.filter.SetValue(m.filterValue)
		return m, nil
	}
	return m.updateComponents(msg)
}

// cycleFilterValue advances the current filter value to the next unique
// author or company present in the raw search results.
func (m *model) cycleFilterValue() {
	switch m.filterField {
	case filterAuthor:
		m.filterValue = nextUniqueValue(uniqueAuthors(m.allPackages), m.filterValue)
	case filterCompany:
		m.filterValue = nextUniqueValue(uniqueCompanies(m.allPackages), m.filterValue)
	default:
		m.filterValue = ""
	}
}

// refreshResults reprojects allPackages through the current sort/filter and
// updates the list items in place.
func (m *model) refreshResults() {
	items := packagesToItems(m.visiblePackages(), m.styles.palette)
	m.results.SetItems(items)
	m.results.Title = fmt.Sprintf("Charts · %d", len(items))
}

// visiblePackages returns allPackages sorted and filtered by the current
// sort/filter settings.
func (m model) visiblePackages() []artifacthub.Package {
	author, company := "", ""
	switch m.filterField {
	case filterAuthor:
		author = m.filterValue
	case filterCompany:
		company = m.filterValue
	}
	return applySortFilter(m.allPackages, m.sortField, m.sortDir, author, company)
}

// handleVersionsKey processes selection on the versions screen.
func (m model) handleVersionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.versions.FilterState() == list.Filtering {
		return m.updateComponents(msg)
	}
	switch msg.String() {
	case "esc":
		m.state = stateResults
		return m, nil
	case "enter":
		item, ok := m.versions.SelectedItem().(versionItem)
		if !ok {
			return m, nil
		}
		m.selectedVersion = item.version.Version
		m.state = statePreparing
		m.errStep = "prepare"
		var cleanup tea.Cmd
		if m.prepared.WorkDir != "" {
			cleanup = cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir)
		}
		// Starting a new prepare: abort any operation still running from a prior
		// selection, then install a fresh context so the new work is not born
		// cancelled.
		m.cancel()
		m.ctx, m.cancel = context.WithCancel(context.Background())
		return m, tea.Batch(m.spinner.Tick, cleanup, prepareCmd(m.ctx, m.pipeline, m.selectedPkg, m.selectedVersion))
	}
	return m.updateComponents(msg)
}

// handleReviewKey processes the image review checklist.
func (m model) handleReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearStatus()
		m.state = stateVersions
		return m, nil
	case "up", "k":
		if m.reviewCursor > 0 {
			m.reviewCursor--
		}
	case "down", "j":
		if m.reviewCursor < len(m.reviewImages)-1 {
			m.reviewCursor++
		}
	case "pgup", "ctrl+u":
		_, visible := m.reviewViewport()
		m.reviewCursor -= visible
		if m.reviewCursor < 0 {
			m.reviewCursor = 0
		}
	case "pgdown", "ctrl+d":
		_, visible := m.reviewViewport()
		m.reviewCursor += visible
		if n := len(m.reviewImages); n > 0 && m.reviewCursor >= n {
			m.reviewCursor = n - 1
		}
	case "g", "home":
		m.reviewCursor = 0
	case "G", "end":
		if n := len(m.reviewImages); n > 0 {
			m.reviewCursor = n - 1
		}
	case "space":
		if len(m.reviewImages) > 0 {
			m.reviewImages[m.reviewCursor].Selected = !m.reviewImages[m.reviewCursor].Selected
		}
	case "a":
		m.clearStatus()
		m.addInput.SetValue("")
		m.addInput.Focus()
		m.state = stateAddImage
		return m, nil
	case "d":
		if len(m.reviewImages) > 0 {
			m.reviewImages = append(m.reviewImages[:m.reviewCursor], m.reviewImages[m.reviewCursor+1:]...)
			if m.reviewCursor >= len(m.reviewImages) && m.reviewCursor > 0 {
				m.reviewCursor--
			}
		}
	case "enter":
		// Chart-only bundle (e.g. a CRD chart with no container images): there is
		// nothing to select or download, so skip straight to assembling the chart
		// alone. The deprecated/prerelease confirmation gate still applies.
		// -import-images is applied once in preparedMsg so empty discovery still
		// shows an approved list before this path runs.
		if len(m.reviewImages) == 0 {
			if warn := m.reviewSafetyWarning(); warn != "" && !m.reviewWarnAck {
				m.reviewWarnAck = true
				m.setStatus(warn)
				return m, nil
			}
			m.clearStatus()
			m.prepared.Images = nil
			m.entries, m.failures = nil, nil
			m.state = stateBundling
			m.errStep = "bundle"
			return m, tea.Batch(m.spinner.Tick,
				bundleCmd(m.pipeline, m.prepared, m.selectedPkg, m.selectedVersion, nil))
		}
		if m.countSelected() == 0 {
			m.setStatus("Select at least one image (space), or press a to add one.")
			return m, nil
		}
		if warn := m.reviewSafetyWarning(); warn != "" && !m.reviewWarnAck {
			m.reviewWarnAck = true
			m.setStatus(warn)
			return m, nil
		}
		m.clearStatus()
		m.prepared.Images = m.reviewImages
		refs := selectedRefs(m.reviewImages)
		m.entries, m.failures = nil, nil
		m.imageProgress = map[string]imageProgress{}
		m.state = stateDownloading
		m.downCurrent, m.downTotal = 0, len(refs)
		return m, tea.Batch(m.spinner.Tick, downloadCmd(m.ctx, m.pipeline, m.prepared, refs, m.activity))
	}
	m.ensureReviewCursorVisible()
	return m, nil
}

// handleDownloadReviewKey processes the post-download failures screen, where the
// user can retry failed images, continue with what downloaded, or abort.
func (m model) handleDownloadReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		refs := failureRefs(m.failures)
		m.failures = nil
		m.imageProgress = map[string]imageProgress{}
		m.state = stateDownloading
		m.errStep = "download"
		m.downCurrent, m.downTotal = 0, len(refs)
		return m, tea.Batch(m.spinner.Tick, downloadCmd(m.ctx, m.pipeline, m.prepared, refs, m.activity))
	case "c":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.state = stateBundling
		m.errStep = "bundle"
		return m, tea.Batch(m.spinner.Tick,
			bundleCmd(m.pipeline, m.prepared, m.selectedPkg, m.selectedVersion, m.entries))
	case "q", "esc":
		m.cancel()
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir), tea.Quit)
	}
	return m, nil
}

// handleAddImageKey processes the add-custom-image input.
func (m model) handleAddImageKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		ref := strings.TrimSpace(m.addInput.Value())
		if ref == "" {
			m.addInput.Blur()
			m.clearStatus()
			m.state = stateReview
			m.ensureReviewCursorVisible()
			return m, nil
		}
		if !images.ValidRef(ref) {
			// Stay on add screen so the user can edit; do not abort review.
			m.setStatus("Invalid image reference.")
			return m, nil
		}
		m.reviewImages = append(m.reviewImages, images.Image{Ref: ref, Selected: true})
		m.reviewCursor = len(m.reviewImages) - 1
		m.addInput.Blur()
		m.clearStatus()
		m.state = stateReview
		m.ensureReviewCursorVisible()
		return m, nil
	case "esc":
		m.addInput.Blur()
		m.clearStatus()
		m.state = stateReview
		m.ensureReviewCursorVisible()
		return m, nil
	}
	return m.updateComponents(msg)
}

// handleEndKey processes the terminal done/error screens.
func (m model) handleEndKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		m.cancel()
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir), tea.Quit)
	case "n":
		fresh, cmd := m.reset()
		return fresh, cmd
	}
	return m, nil
}

// reset returns the model to a fresh search state for another chart.
func (m model) reset() (model, tea.Cmd) {
	m.cancel()
	fresh := newModel(m.cfg, m.logger)
	fresh.width, fresh.height = m.width, m.height
	fresh.results.SetSize(m.width-2, m.height-6)
	fresh.versions.SetSize(m.width-2, m.height-6)
	fresh.progress.SetWidth(m.progress.Width())
	return fresh, cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir)
}

// selectedRefs returns the references of the images marked for inclusion.
func selectedRefs(imgs []images.Image) []string {
	refs := make([]string, 0, len(imgs))
	for _, img := range imgs {
		if img.Selected {
			refs = append(refs, img.Ref)
		}
	}
	return refs
}

// failureRefs returns the references of the given failures.
func failureRefs(failures []pipeline.ImageFailure) []string {
	refs := make([]string, 0, len(failures))
	for _, f := range failures {
		refs = append(refs, f.Ref)
	}
	return refs
}

// countSelected returns the number of selected images.
func (m model) countSelected() int {
	count := 0
	for _, img := range m.reviewImages {
		if img.Selected {
			count++
		}
	}
	return count
}

// reviewSafetyWarning returns a progressive safety message when the selected
// chart or version needs a second Enter confirmation. Empty means proceed.
func (m model) reviewSafetyWarning() string {
	if m.selectedPkg.Deprecated {
		return "Chart is deprecated on ArtifactHub. Enter again to download anyway."
	}
	if m.selectedIsPrerelease() {
		return "Selected version is a prerelease. Enter again to download anyway."
	}
	return ""
}

// selectedIsPrerelease reports whether the currently selected chart version was
// marked prerelease in the versions list items.
func (m model) selectedIsPrerelease() bool {
	for _, it := range m.versions.Items() {
		vi, ok := it.(versionItem)
		if !ok {
			continue
		}
		if vi.version.Version == m.selectedVersion {
			return vi.version.Prerelease
		}
	}
	return false
}
