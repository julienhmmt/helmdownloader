package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/julienhmmt/helmdownloader/internal/config"
)

// Run starts the TUI program with cfg and blocks until the user quits.
func Run(cfg config.Config) error {
	program := tea.NewProgram(New(cfg), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
