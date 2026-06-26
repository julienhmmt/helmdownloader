package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
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
	case tea.KeyPressMsg:
		return m.handleKey(typed)
	case searchResultMsg:
		m.state = stateResults
		m.results.SetItems(packagesToItems(typed.packages))
		return m, nil
	case versionsMsg:
		m.state = stateVersions
		m.versions.SetItems(versionsToItems(typed.versions))
		return m, nil
	case preparedMsg:
		m.prepared = typed.prepared
		m.reviewImages = typed.prepared.Images
		m.reviewCursor = 0
		m.state = stateReview
		if err := exportImages(m.cfg.ExportImages, m.reviewImages); err != nil {
			m.err = err
			m.state = stateError
			return m, nil
		}
		return m, nil
	case progressMsg:
		m.downCurrent, m.downTotal, m.downRef = typed.current, typed.total, typed.ref
		// A finished image clears any stale byte counter for the next one.
		m.downWritten, m.downSize = 0, 0
		return m, waitForActivity(m.activity)
	case byteProgressMsg:
		m.downWritten, m.downSize = typed.written, typed.total
		return m, waitForActivity(m.activity)
	case downloadDoneMsg:
		m.entries = append(m.entries, typed.entries...)
		m.failures = typed.failures
		if len(typed.failures) == 0 {
			m.state = stateBundling
			return m, tea.Batch(m.spinner.Tick,
				bundleCmd(m.pipeline, m.prepared, m.selectedPkg, m.selectedVersion, m.entries))
		}
		m.state = stateDownloadReview
		return m, nil
	case doneMsg:
		m.bundlePath = typed.bundlePath
		m.state = stateDone
		return m, nil
	case errMsg:
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
	switch m.state {
	case stateSearch:
		return m.handleSearchKey(msg)
	case stateResults:
		return m.handleResultsKey(msg)
	case stateVersions:
		return m.handleVersionsKey(msg)
	case stateReview:
		return m.handleReviewKey(msg)
	case stateAddImage:
		return m.handleAddImageKey(msg)
	case stateDownloadReview:
		return m.handleDownloadReviewKey(msg)
	case stateDone, stateError:
		return m.handleEndKey(msg)
	}
	return m.updateComponents(msg)
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
		return m, tea.Batch(m.spinner.Tick, searchCmd(m.ctx, m.client, query, m.cfg.SearchLimit))
	case "esc":
		m.cancel()
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir, m.prepared.TempWorkDir), tea.Quit)
	}
	return m.updateComponents(msg)
}

// handleResultsKey processes selection on the results screen.
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
		return m, tea.Batch(m.spinner.Tick, versionsCmd(m.ctx, m.client, item.pkg))
	}
	return m.updateComponents(msg)
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
	case "space":
		if len(m.reviewImages) > 0 {
			m.reviewImages[m.reviewCursor].Selected = !m.reviewImages[m.reviewCursor].Selected
		}
	case "a":
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
		if m.countSelected() == 0 {
			return m, nil
		}
		// If an approved image list was provided, it overrides the discovered
		// set: only refs present in the import (and marked Selected) are pulled.
		if m.cfg.ImportImages != "" {
			imported, err := importImages(m.cfg.ImportImages)
			if err != nil {
				m.err = err
				m.state = stateError
				return m, nil
			}
			if len(imported) > 0 {
				m.reviewImages = imported
			}
		}
		if m.countSelected() == 0 {
			return m, nil // import may have deselected everything
		}
		m.prepared.Images = m.reviewImages
		refs := selectedRefs(m.reviewImages)
		m.entries, m.failures = nil, nil
		m.state = stateDownloading
		m.downCurrent, m.downTotal = 0, len(refs)
		return m, tea.Batch(m.spinner.Tick, downloadCmd(m.ctx, m.pipeline, m.prepared, refs, m.activity))
	}
	return m, nil
}

// handleDownloadReviewKey processes the post-download failures screen, where the
// user can retry failed images, continue with what downloaded, or abort.
func (m model) handleDownloadReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		refs := failureRefs(m.failures)
		m.failures = nil
		m.state = stateDownloading
		m.downCurrent, m.downTotal = 0, len(refs)
		return m, tea.Batch(m.spinner.Tick, downloadCmd(m.ctx, m.pipeline, m.prepared, refs, m.activity))
	case "c":
		if len(m.entries) == 0 {
			return m, nil
		}
		m.state = stateBundling
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
		ref := m.addInput.Value()
		if ref != "" {
			m.reviewImages = append(m.reviewImages, images.Image{Ref: ref, Selected: true})
		}
		m.addInput.Blur()
		m.state = stateReview
		return m, nil
	case "esc":
		m.addInput.Blur()
		m.state = stateReview
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
	fresh := New(m.cfg, m.logger)
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
