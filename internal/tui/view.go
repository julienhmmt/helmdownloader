package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the current screen, declaring the alt screen via the v2 tea.View.
func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// render returns the content string for the active screen.
func (m model) render() string {
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
	case stateDownloadReview:
		return m.viewDownloadReview()
	case stateBundling:
		return m.viewBundling()
	case stateDone:
		return m.viewDone()
	case stateError:
		return m.viewError()
	}
	return ""
}

// viewSearch renders the search prompt.
func (m model) viewSearch() string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		"Search ArtifactHub for a Helm chart:",
		"",
		m.search.View(),
	)
	return m.screen("HelmDownloader", "airgap chart bundler", body, "enter search · esc quit")
}

// viewBusy renders a centered spinner with a contextual label.
func (m model) viewBusy() string {
	label := "Searching ArtifactHub…"
	if m.state == statePreparing {
		label = fmt.Sprintf("Pulling and rendering %s %s…", m.selectedPkg.Name, m.selectedVersion)
	}
	return m.frame(fmt.Sprintf("%s %s", m.spinner.View(), label))
}

// viewList renders a bubbles list with a styled footer. List screens are left
// unframed: the widget manages its own sizing and a border would fight it.
func (m model) viewList(body string) string {
	return body + "\n" + m.styles.help.Render("enter select · / filter · esc back · ctrl+c quit")
}

// viewReview renders the image checklist inside the app frame.
func (m model) viewReview() string {
	title := fmt.Sprintf("Images · %s %s", m.selectedPkg.Name, m.selectedVersion)
	subtitle := fmt.Sprintf("%d selected of %d", m.countSelected(), len(m.reviewImages))

	var rows strings.Builder
	if len(m.reviewImages) == 0 {
		rows.WriteString(m.styles.subtle.Render("No images discovered. Press 'a' to add one manually."))
	}
	for index, img := range m.reviewImages {
		cursor := "  "
		if index == m.reviewCursor {
			cursor = m.styles.cursor.Render("▸ ")
		}
		box := "[ ]"
		if img.Selected {
			box = m.styles.checked.Render("[x]")
		}
		fmt.Fprintf(&rows, "%s%s %s", cursor, box, img.Ref)
		if index < len(m.reviewImages)-1 {
			rows.WriteString("\n")
		}
	}

	meta := m.styles.subtle.Render(fmt.Sprintf("prefix %s · platform %s · out %s",
		m.cfg.RegistryPrefix, m.cfg.Platform, m.cfg.OutputDir))
	body := lipgloss.JoinVertical(lipgloss.Left, rows.String(), "", meta)
	return m.screen(title, subtitle, body,
		"space toggle · a add · d delete · enter download · esc back")
}

// viewAddImage renders the manual image entry prompt.
func (m model) viewAddImage() string {
	return m.screen("Add an image reference", "", m.addInput.View(), "enter add · esc cancel")
}

// viewDownloading renders the download progress screen.
func (m model) viewDownloading() string {
	percent := 0.0
	if m.downTotal > 0 {
		percent = float64(m.downCurrent) / float64(m.downTotal)
	}

	lines := []string{
		fmt.Sprintf("%s  %d/%d", m.progress.ViewAs(percent), m.downCurrent, m.downTotal),
		"",
		fmt.Sprintf("%s %s", m.spinner.View(), m.downRef),
	}

	// Show byte-level progress for the in-flight image when available.
	if m.downWritten > 0 {
		if m.downSize > 0 {
			lines = append(lines, m.styles.subtle.Render(
				fmt.Sprintf("%s / %s", humanBytes(m.downWritten), humanBytes(m.downSize))))
		} else {
			lines = append(lines, m.styles.subtle.Render(humanBytes(m.downWritten)))
		}
	}
	if len(m.failures) > 0 {
		lines = append(lines, m.styles.errorMsg.Render(
			fmt.Sprintf("%d failed so far", len(m.failures))))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.screen("Downloading images", "", body, "saving image tarballs, please wait…")
}

// viewBundling renders the brief archive-assembly step.
func (m model) viewBundling() string {
	return m.frame(fmt.Sprintf("%s Assembling bundle…", m.spinner.View()))
}

// viewDownloadReview lists the images that failed to download and the reasons,
// letting the user retry, continue, or abort.
func (m model) viewDownloadReview() string {
	var rows strings.Builder
	for index, f := range m.failures {
		rows.WriteString(m.styles.selected.Render(f.Ref))
		rows.WriteString("\n")
		rows.WriteString(m.styles.subtle.Render("  " + errLine(f.Err)))
		if index < len(m.failures)-1 {
			rows.WriteString("\n")
		}
	}

	ok := m.styles.subtle.Render(fmt.Sprintf("%d downloaded successfully", len(m.entries)))
	footer := "r retry failed · q abort"
	if len(m.entries) > 0 {
		footer = fmt.Sprintf("r retry failed · c continue with %d · q abort", len(m.entries))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.errorMsg.Render(fmt.Sprintf("%d image(s) failed to download", len(m.failures))),
		"",
		rows.String(),
		"",
		ok,
		"",
		m.styles.help.Render(footer),
	)
	return m.frame(body)
}

// humanBytes formats a byte count with a binary unit suffix, e.g. 1536 -> "1.5 KiB".
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// errLine renders an error as a single trimmed line for compact display.
func errLine(err error) string {
	if err == nil {
		return ""
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

// viewDone renders the success summary.
func (m model) viewDone() string {
	lines := []string{
		m.styles.success.Render("✓ Bundle created"),
		"",
		m.bundlePath,
	}
	if len(m.failures) > 0 {
		lines = append(lines, m.styles.errorMsg.Render(
			fmt.Sprintf("%d image(s) failed and were skipped", len(m.failures))))
	}
	lines = append(lines, "", m.styles.help.Render("n new bundle · q quit"))
	return m.frame(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// viewError renders the error screen.
func (m model) viewError() string {
	lines := []string{m.styles.errorMsg.Render("Error"), ""}
	if m.err != nil {
		lines = append(lines, m.err.Error())
	}
	lines = append(lines, "", m.styles.help.Render("n new bundle · q quit"))
	return m.frame(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
