package tui

import "github.com/charmbracelet/lipgloss"

// styleSet groups the lipgloss styles used across the TUI views.
type styleSet struct {
	title    lipgloss.Style
	subtle   lipgloss.Style
	help     lipgloss.Style
	selected lipgloss.Style
	checked  lipgloss.Style
	errorMsg lipgloss.Style
	success  lipgloss.Style
}

// newStyles builds the application's style set.
func newStyles() styleSet {
	return styleSet{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			Padding(0, 1),
		subtle:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		help:     lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(1, 0, 0, 0),
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true),
		checked:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		errorMsg: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
	}
}
