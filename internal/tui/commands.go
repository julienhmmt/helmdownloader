package tui

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// searchCmd queries ArtifactHub for charts matching query.
func searchCmd(ctx context.Context, client *artifacthub.Client, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		packages, err := client.Search(ctx, query, limit)
		if err != nil {
			return errMsg{err}
		}
		return searchResultMsg{packages}
	}
}

// versionsCmd fetches the available versions of the selected package.
func versionsCmd(ctx context.Context, client *artifacthub.Client, pkg artifacthub.Package) tea.Cmd {
	return func() tea.Msg {
		versions, err := client.Versions(ctx, pkg.RepoName, pkg.Name)
		if err != nil {
			return errMsg{err}
		}
		return versionsMsg{versions}
	}
}

// prepareCmd pulls and renders the chart, returning its discovered images.
func prepareCmd(ctx context.Context, pl *pipeline.Pipeline, pkg artifacthub.Package, version string) tea.Cmd {
	return func() tea.Msg {
		prepared, err := pl.Prepare(ctx, pkg, version)
		if err != nil {
			return errMsg{err}
		}
		return preparedMsg{prepared}
	}
}

// downloadCmd saves the given image refs in a goroutine, streaming per-image
// progress over the model's channel. It returns immediately; progress is
// consumed by waitForActivity, and the pass ends with a downloadDoneMsg.
func downloadCmd(ctx context.Context, pl *pipeline.Pipeline, prepared pipeline.Prepared, refs []string, activity chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			entries, failures, err := pl.Download(ctx, prepared, refs,
				func(current, total int, ref string, perr error) {
					// Non-blocking: per-image progress is advisory. A slow terminal
					// could otherwise stall worker goroutines holding concurrency slots.
					select {
					case activity <- progressMsg{current: current, total: total, ref: ref, err: perr}:
					default:
					}
				},
				func(ref string, written, total int64) {
					// Non-blocking: byte updates are advisory, so drop them rather
					// than stall the puller if the UI is busy draining the channel.
					select {
					case activity <- byteProgressMsg{ref: ref, written: written, total: total}:
					default:
					}
				})
			// Terminal sends yield to context cancellation: on quit, ctx is
			// cancelled and nobody drains activity, so a blocking send would
			// leak this goroutine and hold open registry connections.
			if err != nil {
				sendOrCancel(ctx, activity, errMsg{err})
				return
			}
			sendOrCancel(ctx, activity, downloadDoneMsg{entries: entries, failures: failures})
		}()
		return waitForActivity(activity)()
	}
}

// sendOrCancel performs a terminal send that yields to context cancellation,
// so a quit (which cancels ctx and stops draining activity) cannot leak the
// download goroutine on its final send.
func sendOrCancel(ctx context.Context, activity chan tea.Msg, msg tea.Msg) {
	select {
	case activity <- msg:
	case <-ctx.Done():
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

// cleanupCmd removes the work directory when it is a temporary one created by
// Prepare. A user-configured persistent work dir (cfg.WorkDir) is preserved so
// -resume can reuse its tarballs across runs; pipeline.Bundle already prunes
// the per-bundle artifacts (images/ and the chart archive) from it.
func cleanupCmd(dir string, temp bool) tea.Cmd {
	return func() tea.Msg {
		if dir != "" && temp {
			_ = os.RemoveAll(dir)
		}
		return nil
	}
}
