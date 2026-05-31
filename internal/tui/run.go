package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// Run starts the TUI program with cfg and blocks until the user quits.
func Run(cfg config.Config, logger *log.Logger) error {
	program := tea.NewProgram(New(cfg, logger), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
