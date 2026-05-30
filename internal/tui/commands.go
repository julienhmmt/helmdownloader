package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/julienhmmt/helmdownloader/internal/artifacthub"
	"github.com/julienhmmt/helmdownloader/internal/pipeline"
)

// searchCmd queries ArtifactHub for charts matching query.
func searchCmd(client *artifacthub.Client, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		packages, err := client.Search(context.Background(), query, limit)
		if err != nil {
			return errMsg{err}
		}
		return searchResultMsg{packages}
	}
}

// versionsCmd fetches the available versions of the selected package.
func versionsCmd(client *artifacthub.Client, pkg artifacthub.Package) tea.Cmd {
	return func() tea.Msg {
		versions, err := client.Versions(context.Background(), pkg.RepoName, pkg.Name)
		if err != nil {
			return errMsg{err}
		}
		return versionsMsg{versions}
	}
}

// prepareCmd pulls and renders the chart, returning its discovered images.
func prepareCmd(pl *pipeline.Pipeline, pkg artifacthub.Package, version string) tea.Cmd {
	return func() tea.Msg {
		prepared, err := pl.Prepare(context.Background(), pkg, version)
		if err != nil {
			return errMsg{err}
		}
		return preparedMsg{prepared}
	}
}

// buildCmd starts the download+bundle in a goroutine, streaming progress over
// the model's channel. It returns immediately; progress is consumed by
// waitForActivity.
func buildCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		go func() {
			path, err := m.pipeline.Build(m.prepared, m.selectedPkg, m.selectedVersion,
				func(current, total int, ref string, perr error) {
					m.activity <- progressMsg{current: current, total: total, ref: ref, failed: perr != nil}
				})
			if err != nil {
				m.activity <- errMsg{err}
				return
			}
			m.activity <- doneMsg{bundlePath: path}
		}()
		return waitForActivity(m.activity)()
	}
}

// waitForActivity blocks on the activity channel and returns the next message.
func waitForActivity(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
