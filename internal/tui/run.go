package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// Run starts the TUI program with cfg and blocks until the user quits. The alt
// screen is now requested declaratively via the model's View (v2).
func Run(cfg config.Config, logger *log.Logger) error {
	var program = tea.NewProgram(newModel(cfg, logger))
	var final tea.Model
	var err error
	final, err = program.Run()
	if err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	var finished model
	var ok bool
	finished, ok = final.(model)
	if !ok {
		return fmt.Errorf("unexpected TUI model %T", final)
	}
	return writeExitSummary(os.Stdout, finished.sessionBundles)
}

func writeExitSummary(output io.Writer, bundles []sessionBundle) error {
	if len(bundles) == 0 {
		return nil
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Session bundles (%d):\n", len(bundles))
	for _, recorded := range bundles {
		text.WriteString("\n")
		text.WriteString(recorded.exitSummary())
	}
	if _, err := io.WriteString(output, text.String()); err != nil {
		return fmt.Errorf("write exit summary: %w", err)
	}
	return nil
}

func (recorded sessionBundle) exitSummary() string {
	var status = "COMPLETE"
	if recorded.missing > 0 {
		status = "PARTIAL"
	}
	var next = bundleExtractCmd(recorded.path)
	if recorded.included > 0 {
		next += " && ./load.sh"
	} else if recorded.missing == 0 {
		status += " (chart only)"
		next += "   # extract the chart, then helm install"
	}
	var size = bundleSizeHint(recorded.path)
	if size != "" {
		size = " (" + size + ")"
	}
	return fmt.Sprintf("%s: %s%s\n  Images: %d included, %d missing\n  helmdownloader verify %s\n  %s\n",
		status, shellArg(recorded.path), size, recorded.included, recorded.missing, shellArg(recorded.path), next)
}
