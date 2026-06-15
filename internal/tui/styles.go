package tui

import "github.com/charmbracelet/lipgloss"

// Palette — adaptive colors so the UI reads on both light and dark terminals.
var (
	colorAccent = lipgloss.AdaptiveColor{Light: "63", Dark: "63"}
	colorGood   = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	colorBad    = lipgloss.AdaptiveColor{Light: "160", Dark: "196"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "245", Dark: "241"}
	colorBorder = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
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
