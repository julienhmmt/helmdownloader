package tui

import (
	"fmt"
	"os"
	"sort"
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
	case stateFilterInput:
		return m.viewFilterInput()
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

// viewBusy renders a centered spinner with a contextual label and cancel help.
func (m model) viewBusy() string {
	label := "Searching ArtifactHub…"
	if m.state == statePreparing {
		label = fmt.Sprintf("Pulling and rendering %s %s…", m.selectedPkg.Name, m.selectedVersion)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("%s %s", m.spinner.View(), label),
		"",
		m.renderHelp("esc cancel · ctrl+c quit"),
	)
	return m.frame(body)
}

// viewList renders a bubbles list with a styled footer. List screens are left
// unframed: the widget manages its own sizing and a border would fight it.
// On the results screen, a sort/filter status line is shown above the footer.
// Soft m.status feedback is rendered between body and help when set.
func (m model) viewList(body string) string {
	lines := []string{body}
	if m.state == stateResults {
		lines = append(lines, "", m.sortFilterStatus())
	}
	if m.status != "" {
		lines = append(lines, "", m.styles.errorMsg.Render(m.status))
	}
	lines = append(lines, m.renderHelp(m.listHelp()))
	return strings.Join(lines, "\n")
}

// listHelp returns the footer help text for the active list screen. Results
// uses two lines so the sort/filter controls are grouped separately from the
// navigation controls, keeping the footer readable on narrow terminals.
func (m model) listHelp() string {
	if m.state == stateResults {
		return "enter select · esc back · ctrl+c quit\n/ filter · s sort · o dir · f filter · F type · tab cycle"
	}
	return "enter select · / filter · esc back · ctrl+c quit"
}

// renderHelp splits a help string on newlines, then on " · ", and renders each
// segment's key (first token) in the accent color and its label in the muted
// color. This keeps keybindings discoverable while making the footer recede.
func (m model) renderHelp(help string) string {
	lines := strings.Split(help, "\n")
	rendered := make([]string, len(lines))
	for i, line := range lines {
		segments := strings.Split(line, " · ")
		parts := make([]string, 0, len(segments))
		for _, seg := range segments {
			key, label, found := strings.Cut(seg, " ")
			if !found {
				parts = append(parts, m.styles.selected.Render(seg))
				continue
			}
			parts = append(parts, m.styles.selected.Render(key)+m.styles.help.Render(" "+label))
		}
		rendered[i] = strings.Join(parts, m.styles.faint.Render(" · "))
	}
	return strings.Join(rendered, "\n")
}

// sortFilterStatus renders the current sort and filter settings as one line.
func (m model) sortFilterStatus() string {
	sortPart := fmt.Sprintf("sort: %s%s", sortFieldLabel(m.sortField), sortDirSymbol(m.sortDir))
	filterPart := "filter: off"
	if m.filterField != filterNone {
		value := m.filterValue
		if value == "" {
			value = "(any)"
		}
		filterPart = fmt.Sprintf("filter: %s=%q", filterFieldLabel(m.filterField), value)
	}
	count := fmt.Sprintf("%s %s", m.styles.selected.Render(fmt.Sprintf("%d", len(m.visiblePackages()))), m.styles.muted.Render("shown"))
	sep := m.styles.faint.Render(" · ")
	return strings.Join([]string{m.styles.muted.Render(sortPart), m.styles.muted.Render(filterPart), count}, sep)
}

// viewFilterInput renders the filter substring entry prompt.
func (m model) viewFilterInput() string {
	title := fmt.Sprintf("Filter by %s", filterFieldLabel(m.filterField))
	return m.screen(title, "type a substring, tab to cycle values", m.filter.View(),
		"enter apply · tab cycle · esc cancel")
}

// viewReview renders the image checklist inside the app frame. Only a window
// of rows is drawn so large charts remain navigable on short terminals.
func (m model) viewReview() string {
	title := fmt.Sprintf("Images · %s %s", m.selectedPkg.Name, m.selectedVersion)
	subtitle := fmt.Sprintf("%d selected of %d", m.countSelected(), len(m.reviewImages))

	var rows strings.Builder
	if len(m.reviewImages) == 0 {
		rows.WriteString(m.styles.muted.Render("No images discovered. Press 'a' to add one manually."))
	} else {
		start, visible := m.reviewViewport()
		end := start + visible
		if end > len(m.reviewImages) {
			end = len(m.reviewImages)
		}
		refWidth := m.reviewInnerWidth()
		if start > 0 {
			rows.WriteString(m.styles.faint.Render(fmt.Sprintf("↑ %d more", start)))
			rows.WriteString("\n")
		}
		for index := start; index < end; index++ {
			img := m.reviewImages[index]
			cursor := "  "
			if index == m.reviewCursor {
				cursor = m.styles.cursor.Render("▸ ")
			}
			box := "[ ]"
			if img.Selected {
				box = m.styles.checked.Render("[x]")
			}
			ref := truncateMiddle(img.Ref, refWidth)
			fmt.Fprintf(&rows, "%s%s %s", cursor, box, ref)
			if index < end-1 {
				rows.WriteString("\n")
			}
		}
		if end < len(m.reviewImages) {
			rows.WriteString("\n")
			rows.WriteString(m.styles.faint.Render(fmt.Sprintf("↓ %d more", len(m.reviewImages)-end)))
		}
	}

	meta := m.styles.muted.Render(fmt.Sprintf("prefix %s · platform %s · out %s",
		m.cfg.RegistryPrefix, m.cfg.Platform, m.cfg.OutputDir))
	body := lipgloss.JoinVertical(lipgloss.Left, rows.String(), "", meta)
	return m.screen(title, subtitle, body,
		"space toggle · a add · d delete · j/k move · pgup/pgdn page · g/G jump · enter download · esc back")
}

// viewAddImage renders the manual image entry prompt.
func (m model) viewAddImage() string {
	return m.screen("Add an image reference", "", m.addInput.View(), "enter add · esc cancel")
}

// viewDownloading renders the download progress screen: an aggregate bar
// for completed images, plus a per-image mini bar for each in-flight pull
// so the user can see all concurrent downloads advancing.
func (m model) viewDownloading() string {
	percent := 0.0
	if m.downTotal > 0 {
		percent = float64(m.downCurrent) / float64(m.downTotal)
	}

	lines := []string{
		fmt.Sprintf("%s  %d/%d", m.progress.ViewAs(percent), m.downCurrent, m.downTotal),
		"",
	}

	// Render up to a screenful of in-flight images, each with a mini bar.
	// Sort refs for stable display (map iteration order is random).
	refs := make([]string, 0, len(m.imageProgress))
	for ref := range m.imageProgress {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		p := m.imageProgress[ref]
		lines = append(lines, fmt.Sprintf("%s %s %s",
			m.miniBar(p.written, p.total, 20), ref, m.byteLabel(p.written, p.total)))
	}
	if len(refs) == 0 {
		lines = append(lines, m.styles.muted.Render(fmt.Sprintf("%s waiting for first bytes…", m.spinner.View())))
	}

	if len(m.failures) > 0 {
		lines = append(lines, m.styles.errorMsg.Render(
			fmt.Sprintf("%d failed so far", len(m.failures))))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.screen("Downloading images", "", body, "esc cancel · ctrl+c quit")
}

// miniBar renders a width-cell ASCII progress bar for (written/total).
// When total is 0 or unknown, it renders an indeterminate spinner-ish bar.
func (m model) miniBar(written, total int64, width int) string {
	if total <= 0 {
		// Indeterminate: fill proportionally to written but cap at width,
		// so the user sees motion without a false percentage.
		fill := min(int(written/(1024*1024)), width) // 1 cell per MiB written
		return m.bar(fill, width)
	}
	if written > total {
		written = total
	}
	fill := min(int(float64(width)*float64(written)/float64(total)), width)
	return m.bar(fill, width)
}

// bar renders a width-cell progress bar with a gold fill on a faint track,
// bracketed in the faint tone so the bar recedes until it fills.
func (m model) bar(fill, width int) string {
	bracket := m.styles.faint
	filled := m.styles.selected.Render(strings.Repeat("━", fill))
	track := m.styles.faint.Render(strings.Repeat("─", width-fill))
	return bracket.Render("[") + filled + track + bracket.Render("]")
}

// byteLabel renders "written / total" (human-readable) or just "written"
// when total is unknown.
func (m model) byteLabel(written, total int64) string {
	if total > 0 {
		return m.styles.muted.Render(fmt.Sprintf("%s / %s", humanBytes(written), humanBytes(total)))
	}
	return m.styles.muted.Render(humanBytes(written))
}

// viewBundling renders the brief archive-assembly step.
// Bundle ignores ctx, so Esc is a no-op; only ctrl+c aborts the whole process.
func (m model) viewBundling() string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("%s Assembling bundle…", m.spinner.View()),
		"",
		m.renderHelp("ctrl+c quit"),
	)
	return m.frame(body)
}

// viewDownloadReview lists the images that failed to download and the reasons,
// letting the user retry, continue, or abort.
func (m model) viewDownloadReview() string {
	var rows strings.Builder
	for index, f := range m.failures {
		rows.WriteString(m.styles.selected.Render(f.Ref))
		rows.WriteString("\n")
		rows.WriteString(m.styles.muted.Render("  " + errLine(f.Err)))
		if index < len(m.failures)-1 {
			rows.WriteString("\n")
		}
	}

	ok := m.styles.muted.Render(fmt.Sprintf("%d downloaded successfully", len(m.entries)))
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
		m.renderHelp(footer),
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

// viewDone renders the success summary with image counts and next-step hints.
func (m model) viewDone() string {
	lines := []string{
		m.styles.success.Render("✓ Bundle created"),
		"",
		m.bundlePath,
	}
	if sizeHint := bundleSizeHint(m.bundlePath); sizeHint != "" {
		lines = append(lines, m.styles.muted.Render(sizeHint))
	}
	lines = append(lines, "")
	summary := fmt.Sprintf("%d images", len(m.entries))
	if len(m.failures) > 0 {
		summary = fmt.Sprintf("%d images · %d failed (skipped)", len(m.entries), len(m.failures))
		lines = append(lines, m.styles.errorMsg.Render(summary))
	} else {
		lines = append(lines, m.styles.muted.Render(summary))
	}
	lines = append(lines,
		"",
		m.styles.muted.Render("Next:"),
		m.styles.muted.Render(fmt.Sprintf("  helmdownloader verify %s", m.bundlePath)),
		m.styles.muted.Render(fmt.Sprintf("  tar xzf %s && ./load.sh", m.bundlePath)),
		"",
		m.renderHelp("n new bundle · q quit"),
	)
	return m.frame(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// bundleSizeHint returns a human-readable size for path, or empty if unavailable.
func bundleSizeHint(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return humanBytes(info.Size())
}

// viewError renders the error screen with an optional step label.
func (m model) viewError() string {
	title := "Error"
	if m.errStep != "" {
		title = fmt.Sprintf("Error · %s", m.errStep)
	}
	lines := []string{m.styles.errorMsg.Render(title), ""}
	if m.err != nil {
		lines = append(lines, m.err.Error())
	}
	lines = append(lines, "", m.renderHelp("n new bundle · q quit"))
	return m.frame(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
