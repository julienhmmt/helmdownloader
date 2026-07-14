package tui

import (
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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
//
// Light values are deliberately darker / higher-contrast than a pure "paper"
// theme: on forced light mode the host terminal's default FG is often still a
// light grey (from a dark host theme), so every role must set an explicit
// foreground and the frame must paint its own surface.
var (
	// Accent: deeper bronze on light so gold stays legible on cream; bright
	// metallic gold on dark.
	hexAccentLight = lipgloss.Color("#6B5314")
	hexAccentDark  = lipgloss.Color("#E0B84A")
	// Primary body text — near-ink on light, off-white on dark.
	hexPrimaryLight = lipgloss.Color("#141820")
	hexPrimaryDark  = lipgloss.Color("#E8EBF1")
	// Secondary meta (subtitle, repo line) — still clearly readable on cream.
	hexSecondaryLight = lipgloss.Color("#3A4558")
	hexSecondaryDark  = lipgloss.Color("#A2ABC0")
	// Muted help / quiet labels — stepped down but not grey-on-grey.
	hexMutedLight = lipgloss.Color("#5A6478")
	hexMutedDark  = lipgloss.Color("#79839B")
	// Faint separators only.
	hexFaintLight = lipgloss.Color("#8A93A6")
	hexFaintDark  = lipgloss.Color("#525C72")
	// Hover wash: warm parchment on light, raised slate on dark.
	hexHoverLight  = lipgloss.Color("#E4D9BE")
	hexHoverDark   = lipgloss.Color("#2A3344")
	hexGoodLight   = lipgloss.Color("#0F6B52")
	hexGoodDark    = lipgloss.Color("#4FC9A6")
	hexBadLight    = lipgloss.Color("#A83224")
	hexBadDark     = lipgloss.Color("#E8786B")
	hexBorderLight = lipgloss.Color("#9AA3B5")
	hexBorderDark  = lipgloss.Color("#39414F")
	// Frame surface (panel fill). Distinct from the outer terminal wash so the
	// box reads as a card even when the host theme fights us.
	hexSurfaceLight = lipgloss.Color("#F4F1EA")
	hexSurfaceDark  = lipgloss.Color("#1A1E26")
	// Outer terminal background for forced themes (slightly darker than the
	// frame so the panel edges show).
	hexTermBGLight = lipgloss.Color("#E6E2D8")
	hexTermBGDark  = lipgloss.Color("#12151C")
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
	surface   color.Color
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
		surface:   ld(hexSurfaceLight, hexSurfaceDark),
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
		// Foreground + background on the frame so unstyled child text still
		// inherits a readable ink-on-surface pair when the host terminal theme
		// disagrees with the forced light/dark palette.
		frame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.border).
			Foreground(p.primary).
			Background(p.surface).
			Padding(1, 2),
		title:     lipgloss.NewStyle().Bold(true).Foreground(p.accent).Background(p.surface),
		subtitle:  lipgloss.NewStyle().Foreground(p.secondary).Background(p.surface),
		primary:   lipgloss.NewStyle().Foreground(p.primary).Background(p.surface),
		secondary: lipgloss.NewStyle().Foreground(p.secondary).Background(p.surface),
		muted:     lipgloss.NewStyle().Foreground(p.muted).Background(p.surface),
		faint:     lipgloss.NewStyle().Foreground(p.faint).Background(p.surface),
		help:      lipgloss.NewStyle().Foreground(p.muted).Background(p.surface),
		selected:  lipgloss.NewStyle().Foreground(p.accent).Bold(true).Background(p.surface),
		cursor:    lipgloss.NewStyle().Foreground(p.accent).Bold(true).Background(p.surface),
		hover:     lipgloss.NewStyle().Foreground(p.primary).Background(p.hover),
		checked:   lipgloss.NewStyle().Foreground(p.good).Background(p.surface),
		errorMsg:  lipgloss.NewStyle().Foreground(p.bad).Bold(true).Background(p.surface),
		success:   lipgloss.NewStyle().Foreground(p.good).Bold(true).Background(p.surface),
		palette:   p,
	}
}

// textInputStyles builds bubbles textinput styles that match the active palette.
// Defaults assume a dark terminal (ANSI 7 white prompt) and render unreadable on
// a forced light background without this override.
func textInputStyles(p palette) textinput.Styles {
	return textinput.Styles{
		Focused: textinput.StyleState{
			Text:        lipgloss.NewStyle().Foreground(p.primary),
			Placeholder: lipgloss.NewStyle().Foreground(p.muted),
			Suggestion:  lipgloss.NewStyle().Foreground(p.muted),
			Prompt:      lipgloss.NewStyle().Foreground(p.accent).Bold(true),
		},
		Blurred: textinput.StyleState{
			Text:        lipgloss.NewStyle().Foreground(p.secondary),
			Placeholder: lipgloss.NewStyle().Foreground(p.faint),
			Suggestion:  lipgloss.NewStyle().Foreground(p.faint),
			Prompt:      lipgloss.NewStyle().Foreground(p.muted),
		},
		Cursor: textinput.CursorStyle{
			Color: p.accent,
			Shape: tea.CursorBlock,
			Blink: true,
		},
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
		return lipgloss.Color("#E6C766"), lipgloss.Color("#3A4230")
	}
	// Darker bronze fill + mid slate track on light so the bar is visible.
	return lipgloss.Color("#6B5314"), lipgloss.Color("#C8BFA8")
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
