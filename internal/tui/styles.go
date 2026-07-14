package tui

import (
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/julienhmmt/helmdownloader/pkg/config"
)

// Palette — cool slate neutrals with a metallic-gold accent. The neutrals are
// true slate (blue-tinted) rather than the terminal's flat grey ramp, giving a
// clear three-step hierarchy — bright name, readable meta, legible-but-quiet
// description — so the UI never reads as grey-on-grey. The gold accent is warm
// and saturated enough to guide the eye without the harshness of pure orange.
// Light and dark hex pairs below are selected via lipgloss.LightDark.

// Fixed hex colors for the light and dark palettes. Theme-aware styles resolve
// these through lipgloss.LightDark rather than package-level AdaptiveColor so a
// user-forced theme cannot be overridden by the terminal's detected background.
var (
	hexAccentLight    = lipgloss.Color("#8A6D1B")
	hexAccentDark     = lipgloss.Color("#E0B84A")
	hexPrimaryLight   = lipgloss.Color("#1F2530")
	hexPrimaryDark    = lipgloss.Color("#E8EBF1")
	hexSecondaryLight = lipgloss.Color("#54607A")
	hexSecondaryDark  = lipgloss.Color("#A2ABC0")
	hexMutedLight     = lipgloss.Color("#7B8499")
	hexMutedDark      = lipgloss.Color("#79839B")
	hexFaintLight     = lipgloss.Color("#A7AEBE")
	hexFaintDark      = lipgloss.Color("#525C72")
	hexHoverLight     = lipgloss.Color("#EDE6D4")
	hexHoverDark      = lipgloss.Color("#2A3344")
	hexGoodLight      = lipgloss.Color("#1F8A6B")
	hexGoodDark       = lipgloss.Color("#4FC9A6")
	hexBadLight       = lipgloss.Color("#C5402F")
	hexBadDark        = lipgloss.Color("#E8786B")
	hexBorderLight    = lipgloss.Color("#C4CAD6")
	hexBorderDark     = lipgloss.Color("#39414F")
	// Terminal backgrounds applied when the user forces light/dark so forced
	// palette colors stay readable on a mismatched host theme.
	hexTermBGLight = lipgloss.Color("#F7F5F0")
	hexTermBGDark  = lipgloss.Color("#1A1E26")
)

// palette holds resolved colors for the active light/dark mode.
type palette struct {
	accent    color.Color
	primary   color.Color
	secondary color.Color
	muted     color.Color
	faint     color.Color
	hover     color.Color
	good      color.Color
	bad       color.Color
	border    color.Color
}

// resolvePalette picks light or dark hex values from bgIsDark.
func resolvePalette(bgIsDark bool) palette {
	ld := lipgloss.LightDark(bgIsDark)
	return palette{
		accent:    ld(hexAccentLight, hexAccentDark),
		primary:   ld(hexPrimaryLight, hexPrimaryDark),
		secondary: ld(hexSecondaryLight, hexSecondaryDark),
		muted:     ld(hexMutedLight, hexMutedDark),
		faint:     ld(hexFaintLight, hexFaintDark),
		hover:     ld(hexHoverLight, hexHoverDark),
		good:      ld(hexGoodLight, hexGoodDark),
		bad:       ld(hexBadLight, hexBadDark),
		border:    ld(hexBorderLight, hexBorderDark),
	}
}

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
	// raw colors for components that build styles outside styleSet (spinner,
	// package list meta, progress).
	palette palette
}

// newStyles builds the application's style set for the given background.
func newStyles(bgIsDark bool) styleSet {
	p := resolvePalette(bgIsDark)
	return styleSet{
		frame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.border).
			Padding(1, 2),
		title:     lipgloss.NewStyle().Bold(true).Foreground(p.accent),
		subtitle:  lipgloss.NewStyle().Foreground(p.secondary),
		primary:   lipgloss.NewStyle().Foreground(p.primary),
		secondary: lipgloss.NewStyle().Foreground(p.secondary),
		muted:     lipgloss.NewStyle().Foreground(p.muted),
		faint:     lipgloss.NewStyle().Foreground(p.faint),
		help:      lipgloss.NewStyle().Foreground(p.muted),
		selected:  lipgloss.NewStyle().Foreground(p.accent).Bold(true),
		cursor:    lipgloss.NewStyle().Foreground(p.accent).Bold(true),
		hover:     lipgloss.NewStyle().Foreground(p.primary).Background(p.hover),
		checked:   lipgloss.NewStyle().Foreground(p.good),
		errorMsg:  lipgloss.NewStyle().Foreground(p.bad).Bold(true),
		success:   lipgloss.NewStyle().Foreground(p.good).Bold(true),
		palette:   p,
	}
}

// chartDelegateStyles returns the list item styles used for the chart and
// version lists. The focused row uses a soft background wash rather than
// recoloring the text, so selection stays obvious without fighting the
// primary/secondary hierarchy. No side-stripe border — that creates an
// awkward visual bump on narrow terminals.
func chartDelegateStyles(p palette) list.DefaultItemStyles {
	s := list.NewDefaultItemStyles(true)
	s.NormalTitle = lipgloss.NewStyle().
		Foreground(p.primary).
		Padding(0, 0, 0, 1)
	s.NormalDesc = lipgloss.NewStyle().
		Foreground(p.secondary).
		Padding(0, 0, 0, 1)
	s.SelectedTitle = lipgloss.NewStyle().
		Foreground(p.primary).
		Background(p.hover).
		Bold(true).
		Padding(0, 0, 0, 1)
	s.SelectedDesc = lipgloss.NewStyle().
		Foreground(p.secondary).
		Background(p.hover).
		Padding(0, 0, 0, 1)
	s.DimmedTitle = lipgloss.NewStyle().
		Foreground(p.muted).
		Padding(0, 0, 0, 1)
	s.DimmedDesc = lipgloss.NewStyle().
		Foreground(p.muted).
		Padding(0, 0, 0, 1)
	s.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(p.accent)
	return s
}

// progressColors returns the fill / empty track colors for the download bar.
func progressColors(bgIsDark bool) (fill, empty color.Color) {
	if bgIsDark {
		return lipgloss.Color("#E6C766"), lipgloss.Color("#B8902E")
	}
	return lipgloss.Color("#B8902E"), lipgloss.Color("#D4C9A8")
}

// themeBackground returns a forced terminal background color for light/dark
// themes, or nil for auto (leave the host terminal background alone).
func themeBackground(theme string) color.Color {
	switch config.NormalizeTheme(theme) {
	case config.ThemeLight:
		return hexTermBGLight
	case config.ThemeDark:
		return hexTermBGDark
	default:
		return nil
	}
}

// themeIsDark reports whether styles should use the dark palette for theme.
// For auto, preferredIsDark is the detected terminal background (or default).
func themeIsDark(theme string, preferredIsDark bool) bool {
	switch config.NormalizeTheme(theme) {
	case config.ThemeLight:
		return false
	case config.ThemeDark:
		return true
	default:
		return preferredIsDark
	}
}
