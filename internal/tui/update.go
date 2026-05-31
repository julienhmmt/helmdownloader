package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/julienhmmt/helmdownloader/internal/images"
)

// Update is the Bubble Tea message dispatcher.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = typed.Width, typed.Height
		m.results.SetSize(typed.Width-2, typed.Height-6)
		m.versions.SetSize(typed.Width-2, typed.Height-6)
		return m, nil
	case tea.KeyMsg:
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
		return m, nil
	case progressMsg:
		m.downCurrent, m.downTotal, m.downRef = typed.current, typed.total, typed.ref
		if typed.failed {
			m.downFailures++
		}
		return m, waitForActivity(m.activity)
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
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir), tea.Quit)
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
	case stateDone, stateError:
		return m.handleEndKey(msg)
	}
	return m.updateComponents(msg)
}

// handleSearchKey processes input on the search screen.
func (m model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		query := m.search.Value()
		if query == "" {
			return m, nil
		}
		m.state = stateSearching
		return m, tea.Batch(m.spinner.Tick, searchCmd(m.client, query, m.cfg.SearchLimit))
	case tea.KeyEsc:
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir), tea.Quit)
	}
	return m.updateComponents(msg)
}

// handleResultsKey processes selection on the results screen.
func (m model) handleResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.results.FilterState() == 1 { // filtering: let the list consume keys
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
		return m, tea.Batch(m.spinner.Tick, versionsCmd(m.client, item.pkg))
	}
	return m.updateComponents(msg)
}

// handleVersionsKey processes selection on the versions screen.
func (m model) handleVersionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.versions.FilterState() == 1 {
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
			cleanup = cleanupCmd(m.prepared.WorkDir)
		}
		return m, tea.Batch(m.spinner.Tick, cleanup, prepareCmd(m.pipeline, m.selectedPkg, m.selectedVersion))
	}
	return m.updateComponents(msg)
}

// handleReviewKey processes the image review checklist.
func (m model) handleReviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case " ":
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
		if !m.hasSelection() {
			return m, nil
		}
		m.prepared.Images = m.reviewImages
		m.state = stateDownloading
		m.downCurrent, m.downTotal, m.downFailures = 0, m.countSelected(), 0
		return m, tea.Batch(m.spinner.Tick, buildCmd(m.pipeline, m.prepared, m.selectedPkg, m.selectedVersion, m.activity))
	}
	return m, nil
}

// handleAddImageKey processes the add-custom-image input.
func (m model) handleAddImageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		ref := m.addInput.Value()
		if ref != "" {
			m.reviewImages = append(m.reviewImages, images.Image{Ref: ref, Selected: true})
		}
		m.addInput.Blur()
		m.state = stateReview
		return m, nil
	case tea.KeyEsc:
		m.addInput.Blur()
		m.state = stateReview
		return m, nil
	}
	return m.updateComponents(msg)
}

// handleEndKey processes the terminal done/error screens.
func (m model) handleEndKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		return m, tea.Batch(cleanupCmd(m.prepared.WorkDir), tea.Quit)
	case "n":
		fresh, cmd := m.reset()
		return fresh, cmd
	}
	return m, nil
}

// reset returns the model to a fresh search state for another chart.
func (m model) reset() (model, tea.Cmd) {
	fresh := New(m.cfg, m.logger)
	fresh.width, fresh.height = m.width, m.height
	fresh.results.SetSize(m.width-2, m.height-6)
	fresh.versions.SetSize(m.width-2, m.height-6)
	return fresh, cleanupCmd(m.prepared.WorkDir)
}

// hasSelection reports whether at least one image is selected.
func (m model) hasSelection() bool {
	return m.countSelected() > 0
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
