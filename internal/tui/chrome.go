package tui

import "github.com/charmbracelet/lipgloss"

// maxFrameWidth caps the bordered panel so content stays readable on wide
// terminals; on narrow ones the panel shrinks to fit.
const maxFrameWidth = 76

// frame wraps screen content in the app's bordered panel and centers it in the
// terminal when the window size is known.
func (m model) frame(body string) string {
	inner := maxFrameWidth
	if m.width > 0 {
		inner = max(0, min(maxFrameWidth, m.width-4))
	}
	boxed := m.styles.frame.Width(inner).Render(body)
	if m.width <= 0 || m.height <= 0 {
		return boxed
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

// screen composes a titled panel: accent title, optional subtitle, a body, and
// a dim help footer, all inside the bordered frame.
func (m model) screen(title, subtitle, body, help string) string {
	parts := []string{m.styles.title.Render(title)}
	if subtitle != "" {
		parts = append(parts, m.styles.subtitle.Render(subtitle))
	}
	parts = append(parts, "", body)
	if help != "" {
		parts = append(parts, "", m.styles.help.Render(help))
	}
	return m.frame(lipgloss.JoinVertical(lipgloss.Left, parts...))
}
