package tui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Palette — adaptive colors so the UI reads on both light and dark terminals.
var (
	colorAccent = compat.AdaptiveColor{Light: lipgloss.Color("63"), Dark: lipgloss.Color("63")}
	colorGood   = compat.AdaptiveColor{Light: lipgloss.Color("28"), Dark: lipgloss.Color("42")}
	colorBad    = compat.AdaptiveColor{Light: lipgloss.Color("160"), Dark: lipgloss.Color("196")}
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
