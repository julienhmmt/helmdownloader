package tui

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
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

// downloadCmd saves the given image refs in a goroutine, streaming per-image
// progress over the model's channel. It returns immediately; progress is
// consumed by waitForActivity, and the pass ends with a downloadDoneMsg.
func downloadCmd(pl *pipeline.Pipeline, prepared pipeline.Prepared, refs []string, activity chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		go func() {
			entries, failures, err := pl.Download(ctx, prepared, refs,
				func(current, total int, ref string, perr error) {
					activity <- progressMsg{current: current, total: total, ref: ref, err: perr}
				},
				func(ref string, written, total int64) {
					// Non-blocking: byte updates are advisory, so drop them rather
					// than stall the puller if the UI is busy draining the channel.
					select {
					case activity <- byteProgressMsg{ref: ref, written: written, total: total}:
					default:
					}
				})
			if err != nil {
				activity <- errMsg{err}
				return
			}
			activity <- downloadDoneMsg{entries: entries, failures: failures}
		}()
		return waitForActivity(activity)()
	}
}

// bundleCmd assembles the downloaded entries into the final archive.
func bundleCmd(pl *pipeline.Pipeline, prepared pipeline.Prepared, pkg artifacthub.Package, version string, entries []bundle.ImageEntry) tea.Cmd {
	return func() tea.Msg {
		path, err := pl.Bundle(prepared, pkg, version, entries)
		if err != nil {
			return errMsg{err}
		}
		return doneMsg{bundlePath: path}
	}
}

// waitForActivity blocks on the activity channel and returns the next message.
func waitForActivity(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// cleanupCmd removes the given temporary directory in the background.
func cleanupCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		if dir != "" {
			_ = os.RemoveAll(dir)
		}
		return nil
	}
}
