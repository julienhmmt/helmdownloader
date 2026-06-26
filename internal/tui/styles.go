package tui

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Palette — warm sandstone: a soft amber/gold accent over neutral sand,
// muted teal for success, warm red for errors. Adaptive so the UI reads on
// both light and dark terminals.
var (
	colorAccent = compat.AdaptiveColor{Light: lipgloss.Color("136"), Dark: lipgloss.Color("179")}
	colorGood   = compat.AdaptiveColor{Light: lipgloss.Color("30"), Dark: lipgloss.Color("37")}
	colorBad    = compat.AdaptiveColor{Light: lipgloss.Color("124"), Dark: lipgloss.Color("160")}
	colorMuted  = compat.AdaptiveColor{Light: lipgloss.Color("245"), Dark: lipgloss.Color("241")}
	colorBorder = compat.AdaptiveColor{Light: lipgloss.Color("250"), Dark: lipgloss.Color("238")}
)

// styleSet groups the lipgloss styles used across the TUI views.
type styleSet struct {
	frame    lipgloss.Style
	title    lipgloss.Style
	subtitle lipgloss.Style
	subtle   lipgloss.Style
	help     lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	checked  lipgloss.Style
	errorMsg lipgloss.Style
	success  lipgloss.Style
}

// newStyles builds the application's style set.
func newStyles() styleSet {
	return styleSet{
		frame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2),
		title:    lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		subtitle: lipgloss.NewStyle().Foreground(colorMuted),
		subtle:   lipgloss.NewStyle().Foreground(colorMuted),
		help:     lipgloss.NewStyle().Foreground(colorMuted),
		selected: lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		cursor:   lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		checked:  lipgloss.NewStyle().Foreground(colorGood),
		errorMsg: lipgloss.NewStyle().Foreground(colorBad).Bold(true),
		success:  lipgloss.NewStyle().Foreground(colorGood).Bold(true),
	}
}

// warmDelegateStyles returns list.DefaultItemStyles with the selected item
// recolored to the warm sandstone accent instead of the library's default
// neon-pink, so the highlighted/hovered chart matches the rest of the UI.
func warmDelegateStyles() list.DefaultItemStyles {
	s := list.NewDefaultItemStyles(true)
	s.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Foreground(colorAccent).
		Bold(true).
		Padding(0, 0, 0, 1)
	s.SelectedDesc = s.SelectedTitle.Foreground(colorMuted)
	return s
}
