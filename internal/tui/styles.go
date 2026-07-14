package tui

import (
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/julienhmmt/helmdownloader/pkg/config"
)

// palette holds resolved colors for the active theme.
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
	termBG    color.Color // nil for auto (do not paint terminal background)
	progress  [2]color.Color
	isDark    bool
}

// hex is a short helper so palette tables stay readable.
func hex(s string) color.Color { return lipgloss.Color(s) }

// lightPalette is warm cream + bronze (default forced light).
func lightPalette() palette {
	return palette{
		accent:    hex("#6B5314"),
		primary:   hex("#141820"),
		secondary: hex("#3A4558"),
		muted:     hex("#5A6478"),
		faint:     hex("#8A93A6"),
		hover:     hex("#E4D9BE"),
		good:      hex("#0F6B52"),
		bad:       hex("#A83224"),
		border:    hex("#9AA3B5"),
		surface:   hex("#F4F1EA"),
		termBG:    hex("#E6E2D8"),
		progress:  [2]color.Color{hex("#6B5314"), hex("#C8BFA8")},
		isDark:    false,
	}
}

// darkPalette is cool slate + metallic gold (default forced dark / auto start).
func darkPalette() palette {
	return palette{
		accent:    hex("#E0B84A"),
		primary:   hex("#E8EBF1"),
		secondary: hex("#A2ABC0"),
		muted:     hex("#79839B"),
		faint:     hex("#525C72"),
		hover:     hex("#2A3344"),
		good:      hex("#4FC9A6"),
		bad:       hex("#E8786B"),
		border:    hex("#39414F"),
		surface:   hex("#1A1E26"),
		termBG:    hex("#12151C"),
		progress:  [2]color.Color{hex("#E6C766"), hex("#3A4230")},
		isDark:    true,
	}
}

// highContrastPalette is pure black/white with a bright amber accent for
// maximum readability (accessibility, projectors, bright offices).
func highContrastPalette() palette {
	return palette{
		accent:    hex("#FFD400"),
		primary:   hex("#FFFFFF"),
		secondary: hex("#E0E0E0"),
		muted:     hex("#B0B0B0"),
		faint:     hex("#808080"),
		hover:     hex("#2A2A2A"),
		good:      hex("#00FF88"),
		bad:       hex("#FF5555"),
		border:    hex("#FFFFFF"),
		surface:   hex("#000000"),
		termBG:    hex("#000000"),
		progress:  [2]color.Color{hex("#FFD400"), hex("#404040")},
		isDark:    true,
	}
}

// oceanPalette is cool blue-slate with a cyan accent — ops/console feel without gold.
func oceanPalette() palette {
	return palette{
		accent:    hex("#3DDCFF"),
		primary:   hex("#E8F4FA"),
		secondary: hex("#8FB8C9"),
		muted:     hex("#6A91A3"),
		faint:     hex("#3F5F70"),
		hover:     hex("#1A3544"),
		good:      hex("#3DDBA0"),
		bad:       hex("#FF7A7A"),
		border:    hex("#2A4A5C"),
		surface:   hex("#0D1B24"),
		termBG:    hex("#081118"),
		progress:  [2]color.Color{hex("#3DDCFF"), hex("#1A3544")},
		isDark:    true,
	}
}

// matrixPalette is green-on-black vanity theme (terminal classic).
func matrixPalette() palette {
	return palette{
		accent:    hex("#39FF14"),
		primary:   hex("#00FF66"),
		secondary: hex("#00CC52"),
		muted:     hex("#1FA84A"),
		faint:     hex("#0F6B2E"),
		hover:     hex("#0A2A12"),
		good:      hex("#39FF14"),
		bad:       hex("#FF3333"),
		border:    hex("#1FA84A"),
		surface:   hex("#050A05"),
		termBG:    hex("#000000"),
		progress:  [2]color.Color{hex("#39FF14"), hex("#0A2A12")},
		isDark:    true,
	}
}

// resolvePalette picks colors for the named theme. For auto, preferredIsDark
// selects the default light or dark palette (no forced termBG).
func resolvePalette(theme string, preferredIsDark bool) palette {
	switch config.NormalizeTheme(theme) {
	case config.ThemeLight:
		return lightPalette()
	case config.ThemeDark:
		return darkPalette()
	case config.ThemeHighContrast:
		return highContrastPalette()
	case config.ThemeOcean:
		return oceanPalette()
	case config.ThemeMatrix:
		return matrixPalette()
	default: // auto
		if preferredIsDark {
			p := darkPalette()
			p.termBG = nil
			return p
		}
		p := lightPalette()
		p.termBG = nil
		return p
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

// newStyles builds the application's style set for the named theme.
// preferredIsDark is used only when theme is auto.
func newStyles(theme string, preferredIsDark bool) styleSet {
	p := resolvePalette(theme, preferredIsDark)
	return styleSet{
		// Foreground + background on the frame so unstyled child text still
		// inherits a readable ink-on-surface pair when the host terminal theme
		// disagrees with the forced palette.
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

// progressColors returns the fill / empty track colors for the palette.
func progressColors(p palette) (fill, empty color.Color) {
	return p.progress[0], p.progress[1]
}

// themeBackground returns a forced terminal background color for named themes,
// or nil for auto (leave the host terminal background alone).
func themeBackground(theme string) color.Color {
	p := resolvePalette(theme, true)
	if config.NormalizeTheme(theme) == config.ThemeAuto {
		return nil
	}
	return p.termBG
}

// themeIsDark reports whether styles should treat theme as dark for fallback
// paths. Named themes use their own palette.isDark; auto follows preferredIsDark.
func themeIsDark(theme string, preferredIsDark bool) bool {
	return resolvePalette(theme, preferredIsDark).isDark
}
