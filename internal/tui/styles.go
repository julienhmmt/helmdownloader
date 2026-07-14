package tui

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Palette — cool slate neutrals with a metallic-gold accent. The neutrals are
// true slate (blue-tinted) rather than the terminal's flat grey ramp, giving a
// clear three-step hierarchy — bright name, readable meta, legible-but-quiet
// description — so the UI never reads as grey-on-grey. The gold accent is warm
// and saturated enough to guide the eye without the harshness of pure orange.
// Truecolor hex values; adaptive so the interface reads on light and dark
// terminals alike.
var (
	// Gold accent — metallic, warmer and softer than the old orange.
	colorAccent = compat.AdaptiveColor{Light: lipgloss.Color("#8A6D1B"), Dark: lipgloss.Color("#E0B84A")}
	// Brightest text — chart names, primary values.
	colorPrimary = compat.AdaptiveColor{Light: lipgloss.Color("#1F2530"), Dark: lipgloss.Color("#E8EBF1")}
	// Mid slate — metadata (repo / by / app). Clearly readable, a step below name.
	colorSecondary = compat.AdaptiveColor{Light: lipgloss.Color("#54607A"), Dark: lipgloss.Color("#A2ABC0")}
	// Quiet slate — descriptions and help. Subordinate but still legible (the old
	// grey-on-grey lived here; this lifts it well above the background).
	colorMuted = compat.AdaptiveColor{Light: lipgloss.Color("#7B8499"), Dark: lipgloss.Color("#79839B")}
	// Faint slate — separators and de-emphasized chrome.
	colorFaint = compat.AdaptiveColor{Light: lipgloss.Color("#A7AEBE"), Dark: lipgloss.Color("#525C72")}
	// Soft hover wash for the focused list row. Light terminals get a warm
	// parchment tint; dark terminals get a raised slate slab. Content stays
	// primary/secondary on top so selection is a background, not a recolor.
	colorHover  = compat.AdaptiveColor{Light: lipgloss.Color("#EDE6D4"), Dark: lipgloss.Color("#2A3344")}
	colorGood   = compat.AdaptiveColor{Light: lipgloss.Color("#1F8A6B"), Dark: lipgloss.Color("#4FC9A6")}
	colorBad    = compat.AdaptiveColor{Light: lipgloss.Color("#C5402F"), Dark: lipgloss.Color("#E8786B")}
	colorBorder = compat.AdaptiveColor{Light: lipgloss.Color("#C4CAD6"), Dark: lipgloss.Color("#39414F")}
)

// styleSet groups the lipgloss styles used across the TUI views.
type styleSet struct {
	frame     lipgloss.Style
	title     lipgloss.Style
	subtitle  lipgloss.Style
	primary   lipgloss.Style
	secondary lipgloss.Style
	muted     lipgloss.Style
	faint     lipgloss.Style
	help      lipgloss.Style
	selected  lipgloss.Style
	cursor    lipgloss.Style
	hover     lipgloss.Style
	checked   lipgloss.Style
	errorMsg  lipgloss.Style
	success   lipgloss.Style
}

// newStyles builds the application's style set.
func newStyles() styleSet {
	return styleSet{
		frame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2),
		title:     lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		subtitle:  lipgloss.NewStyle().Foreground(colorSecondary),
		primary:   lipgloss.NewStyle().Foreground(colorPrimary),
		secondary: lipgloss.NewStyle().Foreground(colorSecondary),
		muted:     lipgloss.NewStyle().Foreground(colorMuted),
		faint:     lipgloss.NewStyle().Foreground(colorFaint),
		help:      lipgloss.NewStyle().Foreground(colorMuted),
		selected:  lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		cursor:    lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		hover:     lipgloss.NewStyle().Foreground(colorPrimary).Background(colorHover),
		checked:   lipgloss.NewStyle().Foreground(colorGood),
		errorMsg:  lipgloss.NewStyle().Foreground(colorBad).Bold(true),
		success:   lipgloss.NewStyle().Foreground(colorGood).Bold(true),
	}
}

// chartDelegateStyles returns the list item styles used for the chart and
// version lists. The focused row uses a soft background wash rather than
// recoloring the text, so selection stays obvious without fighting the
// primary/secondary hierarchy. No side-stripe border — that creates an
// awkward visual bump on narrow terminals.
func chartDelegateStyles() list.DefaultItemStyles {
	s := list.NewDefaultItemStyles(true)
	s.NormalTitle = lipgloss.NewStyle().
		Foreground(colorPrimary).
		Padding(0, 0, 0, 1)
	s.NormalDesc = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Padding(0, 0, 0, 1)
	s.SelectedTitle = lipgloss.NewStyle().
		Foreground(colorPrimary).
		Background(colorHover).
		Bold(true).
		Padding(0, 0, 0, 1)
	s.SelectedDesc = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Background(colorHover).
		Padding(0, 0, 0, 1)
	s.DimmedTitle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 0, 0, 1)
	s.DimmedDesc = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 0, 0, 1)
	s.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(colorAccent)
	return s
}
