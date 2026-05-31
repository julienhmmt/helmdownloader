package tui

import (
	"fmt"
	"strings"
)

// View renders the current screen.
func (m model) View() string {
	switch m.state {
	case stateSearch:
		return m.viewSearch()
	case stateSearching, statePreparing:
		return m.viewBusy()
	case stateResults:
		return m.viewList(m.results.View())
	case stateVersions:
		return m.viewList(m.versions.View())
	case stateReview:
		return m.viewReview()
	case stateAddImage:
		return m.viewAddImage()
	case stateDownloading:
		return m.viewDownloading()
	case stateDone:
		return m.viewDone()
	case stateError:
		return m.viewError()
	}
	return ""
}

// viewSearch renders the search prompt.
func (m model) viewSearch() string {
	var builder strings.Builder
	builder.WriteString(m.styles.title.Render("HelmDownloader — airgap chart bundler"))
	builder.WriteString("\n\n")
	builder.WriteString("Search ArtifactHub for a Helm chart:\n\n")
	builder.WriteString(m.search.View())
	builder.WriteString("\n")
	builder.WriteString(m.styles.help.Render("enter: search   esc: quit"))
	return builder.String()
}

// viewBusy renders a spinner with a contextual label.
func (m model) viewBusy() string {
	label := "Searching ArtifactHub…"
	if m.state == statePreparing {
		label = fmt.Sprintf("Pulling and rendering %s %s…", m.selectedPkg.Name, m.selectedVersion)
	}
	return fmt.Sprintf("\n  %s %s\n", m.spinner.View(), label)
}

// viewList renders a bubbles list with a footer.
func (m model) viewList(body string) string {
	return body + "\n" + m.styles.help.Render("enter: select   /: filter   esc: back   ctrl+c: quit")
}

// viewReview renders the image checklist.
func (m model) viewReview() string {
	var builder strings.Builder
	header := fmt.Sprintf("Images for %s %s — %d selected of %d",
		m.selectedPkg.Name, m.selectedVersion, m.countSelected(), len(m.reviewImages))
	builder.WriteString(m.styles.title.Render(header))
	builder.WriteString("\n\n")
	if len(m.reviewImages) == 0 {
		builder.WriteString(m.styles.subtle.Render("  No images discovered. Press 'a' to add one manually.\n"))
	}
	for index, img := range m.reviewImages {
		cursor := "  "
		if index == m.reviewCursor {
			cursor = m.styles.selected.Render("> ")
		}
		box := "[ ]"
		if img.Selected {
			box = m.styles.checked.Render("[x]")
		}
		fmt.Fprintf(&builder, "%s%s %s\n", cursor, box, img.Ref)
	}
	builder.WriteString("\n")
	builder.WriteString(m.styles.subtle.Render(
		fmt.Sprintf("prefix: %s   platform: %s   out: %s",
			m.cfg.RegistryPrefix, m.cfg.Platform, m.cfg.OutputDir)))
	builder.WriteString("\n")
	builder.WriteString(m.styles.help.Render(
		"space: toggle   a: add   d: delete   enter: download   esc: back"))
	return builder.String()
}

// viewAddImage renders the manual image entry prompt.
func (m model) viewAddImage() string {
	var builder strings.Builder
	builder.WriteString(m.styles.title.Render("Add an image reference"))
	builder.WriteString("\n\n")
	builder.WriteString(m.addInput.View())
	builder.WriteString("\n")
	builder.WriteString(m.styles.help.Render("enter: add   esc: cancel"))
	return builder.String()
}

// viewDownloading renders the download progress screen.
func (m model) viewDownloading() string {
	var builder strings.Builder
	builder.WriteString(m.styles.title.Render("Downloading images"))
	builder.WriteString("\n\n")

	percent := 0.0
	if m.downTotal > 0 {
		percent = float64(m.downCurrent) / float64(m.downTotal)
	}
	fmt.Fprintf(&builder, "  %s  %d/%d\n\n", m.progress.ViewAs(percent), m.downCurrent, m.downTotal)
	fmt.Fprintf(&builder, "  %s %s\n", m.spinner.View(), m.downRef)

	if m.downFailures > 0 {
		builder.WriteString(m.styles.errorMsg.Render(
			fmt.Sprintf("  %d failed\n", m.downFailures)))
	}
	builder.WriteString(m.styles.help.Render("downloading and saving image tarballs, please wait…"))
	return builder.String()
}

// viewDone renders the success summary.
func (m model) viewDone() string {
	var builder strings.Builder
	builder.WriteString(m.styles.success.Render("Bundle created"))
	builder.WriteString("\n\n")
	fmt.Fprintf(&builder, "  %s\n", m.bundlePath)
	if m.downFailures > 0 {
		builder.WriteString(m.styles.errorMsg.Render(
			fmt.Sprintf("  %d image(s) failed and were skipped\n", m.downFailures)))
	}
	builder.WriteString("\n")
	builder.WriteString(m.styles.help.Render("n: new bundle   q: quit"))
	return builder.String()
}

// viewError renders the error screen.
func (m model) viewError() string {
	var builder strings.Builder
	builder.WriteString(m.styles.errorMsg.Render("Error"))
	builder.WriteString("\n\n")
	if m.err != nil {
		fmt.Fprintf(&builder, "  %s\n", m.err.Error())
	}
	builder.WriteString("\n")
	builder.WriteString(m.styles.help.Render("n: new bundle   q: quit"))
	return builder.String()
}
