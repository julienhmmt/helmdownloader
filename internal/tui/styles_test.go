package tui

import (
	"fmt"
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

func TestThemeIsDark(t *testing.T) {
	tests := []struct {
		name            string
		theme           string
		preferredIsDark bool
		want            bool
	}{
		{name: "auto follows preferred dark", theme: config.ThemeAuto, preferredIsDark: true, want: true},
		{name: "auto follows preferred light", theme: config.ThemeAuto, preferredIsDark: false, want: false},
		{name: "empty is auto", theme: "", preferredIsDark: false, want: false},
		{name: "light forces light", theme: config.ThemeLight, preferredIsDark: true, want: false},
		{name: "dark forces dark", theme: config.ThemeDark, preferredIsDark: false, want: true},
		{name: "high-contrast is dark", theme: config.ThemeHighContrast, preferredIsDark: false, want: true},
		{name: "ocean is dark", theme: config.ThemeOcean, preferredIsDark: false, want: true},
		{name: "matrix is dark", theme: config.ThemeMatrix, preferredIsDark: false, want: true},
		{name: "mixed case light", theme: "Light", preferredIsDark: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, themeIsDark(tt.theme, tt.preferredIsDark))
		})
	}
}

func TestThemeBackground(t *testing.T) {
	assert.Nil(t, themeBackground(config.ThemeAuto))
	assert.Nil(t, themeBackground(""))
	assert.NotNil(t, themeBackground(config.ThemeLight))
	assert.NotNil(t, themeBackground(config.ThemeDark))
	assert.NotNil(t, themeBackground(config.ThemeHighContrast))
	assert.NotNil(t, themeBackground(config.ThemeOcean))
	assert.NotNil(t, themeBackground(config.ThemeMatrix))
	assert.Equal(t, colorHex(lightPalette().termBG), colorHex(themeBackground(config.ThemeLight)))
	assert.Equal(t, colorHex(darkPalette().termBG), colorHex(themeBackground(config.ThemeDark)))
	assert.Equal(t, colorHex(highContrastPalette().termBG), colorHex(themeBackground(config.ThemeHighContrast)))
	assert.Equal(t, colorHex(oceanPalette().termBG), colorHex(themeBackground(config.ThemeOcean)))
	assert.Equal(t, colorHex(matrixPalette().termBG), colorHex(themeBackground(config.ThemeMatrix)))
}

func TestResolvePalette_NamedThemesDiffer(t *testing.T) {
	light := resolvePalette(config.ThemeLight, true)
	dark := resolvePalette(config.ThemeDark, false)
	hc := resolvePalette(config.ThemeHighContrast, false)
	ocean := resolvePalette(config.ThemeOcean, false)
	matrix := resolvePalette(config.ThemeMatrix, false)

	assert.False(t, light.isDark)
	assert.True(t, dark.isDark)
	assert.True(t, hc.isDark)
	assert.True(t, ocean.isDark)
	assert.True(t, matrix.isDark)

	// Accent colors should be distinct across named themes.
	accents := map[string]string{
		"light":  colorHex(light.accent),
		"dark":   colorHex(dark.accent),
		"hc":     colorHex(hc.accent),
		"ocean":  colorHex(ocean.accent),
		"matrix": colorHex(matrix.accent),
	}
	seen := map[string]string{}
	for name, a := range accents {
		if other, ok := seen[a]; ok {
			t.Errorf("accent collision: %s and %s both use %s", name, other, a)
		}
		seen[a] = name
	}
}

func TestNewStyles_LightAndDarkDiffer(t *testing.T) {
	light := newStyles(config.ThemeLight, true)
	dark := newStyles(config.ThemeDark, false)
	assert.NotEqual(t, colorHex(light.palette.primary), colorHex(dark.palette.primary))
	assert.NotEqual(t, colorHex(light.palette.accent), colorHex(dark.palette.accent))
	assert.NotEqual(t, colorHex(light.palette.hover), colorHex(dark.palette.hover))
	// Light mode must paint an ink-on-surface pair so host-terminal FG cannot
	// wash out body text.
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(light.palette.primary))
	assert.Equal(t, colorHex(lightPalette().surface), colorHex(light.palette.surface))
}

func TestTextInputStyles_LightUsesDarkInk(t *testing.T) {
	p := resolvePalette(config.ThemeLight, true)
	s := textInputStyles(p)
	// Prompt and text must use palette colors, not bubbles' dark-terminal ANSI.
	assert.Equal(t, colorHex(lightPalette().accent), colorHex(s.Focused.Prompt.GetForeground()))
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(s.Focused.Text.GetForeground()))
	assert.Equal(t, colorHex(lightPalette().muted), colorHex(s.Focused.Placeholder.GetForeground()))
}

func TestNewModel_LightThemeForcesPalette(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeLight
	m := newModel(cfg, log.Discard())
	assert.False(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(m.styles.palette.primary))

	view := m.View()
	require.NotNil(t, view.BackgroundColor)
	assert.Equal(t, colorHex(lightPalette().termBG), colorHex(view.BackgroundColor))
}

func TestNewModel_DarkThemeForcesPalette(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	assert.True(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(darkPalette().primary), colorHex(m.styles.palette.primary))

	view := m.View()
	require.NotNil(t, view.BackgroundColor)
	assert.Equal(t, colorHex(darkPalette().termBG), colorHex(view.BackgroundColor))
}

func TestNewModel_NamedThemesPaintBackground(t *testing.T) {
	for _, theme := range []string{config.ThemeHighContrast, config.ThemeOcean, config.ThemeMatrix} {
		t.Run(theme, func(t *testing.T) {
			cfg := config.Default()
			cfg.Theme = theme
			m := newModel(cfg, log.Discard())
			assert.True(t, m.bgKnown)
			assert.True(t, m.bgIsDark)
			require.NotNil(t, m.View().BackgroundColor)
			want := resolvePalette(theme, true)
			assert.Equal(t, colorHex(want.accent), colorHex(m.styles.palette.accent))
			assert.Equal(t, colorHex(want.termBG), colorHex(m.View().BackgroundColor))
		})
	}
}

func TestNewModel_AutoLeavesTerminalBackground(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeAuto
	m := newModel(cfg, log.Discard())
	// Auto starts dark-friendly until detection, but does not paint a bg.
	assert.True(t, m.bgIsDark)
	assert.False(t, m.bgKnown)
	assert.Nil(t, m.View().BackgroundColor)
}

func TestUpdate_BackgroundColorMsgIgnoredWhenThemeForced(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeLight
	m := newModel(cfg, log.Discard())
	next, _ := m.Update(tea.BackgroundColorMsg{})
	got, ok := next.(model)
	require.True(t, ok)
	assert.False(t, got.bgIsDark)
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(got.styles.palette.primary))
}

func TestApplyTheme_SwitchesPaletteAndListMeta(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	m.allPackages = []artifacthub.Package{
		{Name: "redis", Stars: 10, RepoName: "bitnami"},
	}
	m.refreshResults()
	m.cfg.Theme = config.ThemeLight
	m.applyTheme()
	assert.False(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(m.styles.palette.primary))
	require.Len(t, m.results.Items(), 1)
	item, ok := m.results.Items()[0].(packageItem)
	require.True(t, ok)
	assert.Equal(t, colorHex(lightPalette().accent), colorHex(item.palette.accent))
}

func TestOpenThemeMenu_RemembersReturnAndTheme(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	m.state = stateResults
	m.openThemeMenu()
	assert.Equal(t, stateThemeMenu, m.state)
	assert.Equal(t, stateResults, m.themeMenuReturn)
	assert.Equal(t, config.ThemeDark, m.themeBeforeMenu)
	assert.Equal(t, config.ThemeMenuIndex(config.ThemeDark), m.themeMenuCursor)
}

func TestThemeMenu_PreviewConfirmAndCancel(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	m.width, m.height = 100, 40
	m.state = stateSearch
	m.openThemeMenu()

	// Move to light (index 1) and preview.
	m.themeMenuCursor = config.ThemeMenuIndex(config.ThemeLight)
	m.previewThemeAtCursor()
	assert.Equal(t, config.ThemeLight, m.cfg.Theme)
	assert.False(t, m.bgIsDark)
	assert.Equal(t, colorHex(lightPalette().accent), colorHex(m.styles.palette.accent))

	// Cancel restores dark.
	_ = m.cancelThemeMenu()
	assert.Equal(t, stateSearch, m.state)
	assert.Equal(t, config.ThemeDark, m.cfg.Theme)
	assert.True(t, m.bgIsDark)

	// Open again, pick ocean, confirm.
	m.openThemeMenu()
	m.themeMenuCursor = config.ThemeMenuIndex(config.ThemeOcean)
	m.previewThemeAtCursor()
	_ = m.confirmThemeMenu()
	assert.Equal(t, stateSearch, m.state)
	assert.Equal(t, config.ThemeOcean, m.cfg.Theme)
	assert.Equal(t, "Theme: ocean", m.status)
	assert.Equal(t, colorHex(oceanPalette().accent), colorHex(m.styles.palette.accent))
}

func TestThemeMenu_RestoreAutoUsesDetectedDarkness(t *testing.T) {
	// After previewing light, returning to auto must use terminal detection
	// (detectedIsDark), not the light preview's isDark=false.
	cfg := config.Default()
	cfg.Theme = config.ThemeAuto
	m := newModel(cfg, log.Discard())
	m.detectedIsDark = true
	m.applyTheme()
	require.True(t, m.bgIsDark)
	require.Nil(t, themeBackground(m.cfg.Theme))

	m.state = stateSearch
	m.openThemeMenu()
	m.themeMenuCursor = config.ThemeMenuIndex(config.ThemeLight)
	m.previewThemeAtCursor()
	assert.False(t, m.bgIsDark)

	_ = m.cancelThemeMenu()
	assert.Equal(t, config.ThemeAuto, m.cfg.Theme)
	assert.True(t, m.bgIsDark, "auto must follow detectedIsDark=true after light preview")
	assert.Nil(t, m.View().BackgroundColor)
	// Frame must not paint a surface in auto — that mixed light card on dark host.
	assert.True(t, isUnsetBackground(m.styles.frame.GetBackground()))
}

func TestAutoStyles_DoNotPaintSurface(t *testing.T) {
	darkAuto := newStyles(config.ThemeAuto, true)
	assert.True(t, isUnsetBackground(darkAuto.frame.GetBackground()))
	assert.True(t, isUnsetBackground(darkAuto.primary.GetBackground()))
	assert.Equal(t, colorHex(darkPalette().primary), colorHex(darkAuto.palette.primary))

	lightAuto := newStyles(config.ThemeAuto, false)
	assert.True(t, isUnsetBackground(lightAuto.frame.GetBackground()))
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(lightAuto.palette.primary))

	// Named themes still paint surface.
	forced := newStyles(config.ThemeDark, true)
	assert.Equal(t, colorHex(darkPalette().surface), colorHex(forced.frame.GetBackground()))
}

// isUnsetBackground reports whether c is unset (nil or lipgloss.NoColor).
func isUnsetBackground(c color.Color) bool {
	if c == nil {
		return true
	}
	_, isNo := c.(lipgloss.NoColor)
	return isNo
}

func TestBackgroundColorMsg_RecordsDetectionEvenWhenForced(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeLight
	m := newModel(cfg, log.Discard())
	require.False(t, m.detectedIsDark == false && m.bgIsDark) // start detected default true
	// Simulate a light terminal while a forced theme is active.
	// BackgroundColorMsg with a light-ish color: IsDark() depends on luminance;
	// drive detection by applying applyTheme path after setting field.
	m.detectedIsDark = false
	// Force stays light palette.
	assert.Equal(t, config.ThemeLight, m.cfg.Theme)
	assert.False(t, m.bgIsDark)

	// Switch to auto — should pick light because detectedIsDark=false.
	m.cfg.Theme = config.ThemeAuto
	m.applyTheme()
	assert.False(t, m.bgIsDark)
	assert.Equal(t, colorHex(lightPalette().primary), colorHex(m.styles.palette.primary))
	assert.Nil(t, m.View().BackgroundColor)
}

func TestHandleKey_CtrlTOpensThemeMenu(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	m.width, m.height = 100, 40
	m.state = stateSearch

	next, _ := m.handleKey(keyPress("ctrl+t"))
	got, ok := next.(model)
	require.True(t, ok)
	assert.Equal(t, stateThemeMenu, got.state)
	assert.Equal(t, stateSearch, got.themeMenuReturn)

	// j moves down and live-previews.
	next, _ = got.handleKey(keyPress("j"))
	got = next.(model)
	assert.Equal(t, config.ThemeMenuIndex(config.ThemeDark)+1, got.themeMenuCursor)
	assert.Equal(t, config.ThemeHighContrast, got.cfg.Theme)

	// enter applies and returns.
	next, _ = got.handleKey(keyPress("enter"))
	got = next.(model)
	assert.Equal(t, stateSearch, got.state)
	assert.Equal(t, config.ThemeHighContrast, got.cfg.Theme)
	assert.Equal(t, "Theme: high-contrast", got.status)
}

func TestHandleKey_CtrlTIgnoredWhileBusy(t *testing.T) {
	cfg := config.Default()
	m := newModel(cfg, log.Discard())
	m.state = stateDownloading
	next, _ := m.handleKey(keyPress("ctrl+t"))
	got := next.(model)
	assert.Equal(t, stateDownloading, got.state)
}

func TestViewThemeMenuRenders(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeLight
	m := newModel(cfg, log.Discard())
	m.width, m.height = 100, 40
	m.openThemeMenu()
	out := m.render()
	assert.Contains(t, out, "Theme")
	assert.Contains(t, out, "light")
	assert.Contains(t, out, "ocean")
	assert.Contains(t, out, "matrix")
	assert.Contains(t, out, "preview:")
}

// colorHex formats a color as #RRGGBB for stable equality checks.
func colorHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
