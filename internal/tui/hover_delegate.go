package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// hoverDelegate paints the focused list row as a full-width soft background
// wash. bubbles' DefaultDelegate only styles the text content, so without a
// fixed Width the hover bar ends at the last character.
type hoverDelegate struct {
	styles  list.DefaultItemStyles
	height  int
	spacing int
}

// newHoverDelegate builds the delegate used by the chart and version lists.
func newHoverDelegate(p palette) hoverDelegate {
	return hoverDelegate{
		styles:  chartDelegateStyles(p),
		height:  2,
		spacing: 1,
	}
}

// Height returns the preferred item height (title + description).
func (d hoverDelegate) Height() int { return d.height }

// Spacing returns blank lines between items.
func (d hoverDelegate) Spacing() int { return d.spacing }

// Update is a no-op; selection is driven by the list model.
func (d hoverDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render prints one list item. Selected rows use styles with Width set to the
// list width so the hover background spans the whole line.
func (d hoverDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	di, ok := item.(list.DefaultItem)
	if !ok {
		return
	}
	if m.Width() <= 0 {
		return
	}

	s := d.styles
	title := di.Title()
	desc := di.Description()

	padL := s.NormalTitle.GetPaddingLeft()
	padR := s.NormalTitle.GetPaddingRight()
	textwidth := m.Width() - padL - padR
	if textwidth < 1 {
		textwidth = 1
	}
	title = ansi.Truncate(title, textwidth, "…")

	var lines []string
	for i, line := range strings.Split(desc, "\n") {
		if i >= d.height-1 {
			break
		}
		lines = append(lines, ansi.Truncate(line, textwidth, "…"))
	}
	desc = strings.Join(lines, "\n")

	isSelected := index == m.Index()
	emptyFilter := m.FilterState() == list.Filtering && m.FilterValue() == ""
	isFiltered := m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	matchedRunes := m.MatchesForItem(index)

	switch {
	case emptyFilter:
		title = s.DimmedTitle.Render(title)
		desc = s.DimmedDesc.Render(desc)
	case isSelected && m.FilterState() != list.Filtering:
		// Width pads the background to the full list row, not just the title text.
		titleStyle := s.SelectedTitle.Width(m.Width())
		descStyle := s.SelectedDesc.Width(m.Width())
		if isFiltered {
			unmatched := titleStyle.Inline(true)
			matched := unmatched.Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = titleStyle.Render(title)
		desc = descStyle.Render(desc)
	default:
		if isFiltered {
			unmatched := s.NormalTitle.Inline(true)
			matched := unmatched.Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = s.NormalTitle.Render(title)
		desc = s.NormalDesc.Render(desc)
	}

	fmt.Fprintf(w, "%s\n%s", title, desc) //nolint:errcheck
}
