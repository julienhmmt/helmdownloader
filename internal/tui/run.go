package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// Run starts the TUI program with cfg and blocks until the user quits. The alt
// screen is now requested declaratively via the model's View (v2).
func Run(cfg config.Config, logger *log.Logger) error {
	program := tea.NewProgram(New(cfg, logger))
	_, err := program.Run()
	return err
}
