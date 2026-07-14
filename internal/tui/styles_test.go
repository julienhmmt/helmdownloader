package tui

import (
	"fmt"
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	assert.Equal(t, colorHex(hexTermBGLight), colorHex(themeBackground(config.ThemeLight)))
	assert.Equal(t, colorHex(hexTermBGDark), colorHex(themeBackground(config.ThemeDark)))
}

func TestNewStyles_LightAndDarkDiffer(t *testing.T) {
	light := newStyles(false)
	dark := newStyles(true)
	assert.NotEqual(t, colorHex(light.palette.primary), colorHex(dark.palette.primary))
	assert.NotEqual(t, colorHex(light.palette.accent), colorHex(dark.palette.accent))
	assert.NotEqual(t, colorHex(light.palette.hover), colorHex(dark.palette.hover))
}

func TestNewModel_LightThemeForcesPalette(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeLight
	m := newModel(cfg, log.Discard())
	assert.False(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(hexPrimaryLight), colorHex(m.styles.palette.primary))

	view := m.View()
	require.NotNil(t, view.BackgroundColor)
	assert.Equal(t, colorHex(hexTermBGLight), colorHex(view.BackgroundColor))
}

func TestNewModel_DarkThemeForcesPalette(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	assert.True(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(hexPrimaryDark), colorHex(m.styles.palette.primary))

	view := m.View()
	require.NotNil(t, view.BackgroundColor)
	assert.Equal(t, colorHex(hexTermBGDark), colorHex(view.BackgroundColor))
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
	assert.Equal(t, colorHex(hexPrimaryLight), colorHex(got.styles.palette.primary))
}

func TestApplyTheme_SwitchesPaletteAndListMeta(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = config.ThemeDark
	m := newModel(cfg, log.Discard())
	m.allPackages = []artifacthub.Package{
		{Name: "redis", Stars: 10, RepoName: "bitnami"},
	}
	m.refreshResults()
	m.applyTheme(false)
	assert.False(t, m.bgIsDark)
	assert.True(t, m.bgKnown)
	assert.Equal(t, colorHex(hexPrimaryLight), colorHex(m.styles.palette.primary))
	require.Len(t, m.results.Items(), 1)
	item, ok := m.results.Items()[0].(packageItem)
	require.True(t, ok)
	assert.Equal(t, colorHex(hexAccentLight), colorHex(item.palette.accent))
}

// colorHex formats a color as #RRGGBB for stable equality checks.
func colorHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
