package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/config"
)

func TestDefault_HasSensibleValues(t *testing.T) {
	cfg := config.Default()
	assert.Equal(t, "https://artifacthub.io", cfg.ArtifactHubURL)
	assert.Equal(t, "helm", cfg.HelmBin)
	assert.Equal(t, "linux/amd64", cfg.Platform)
	assert.Equal(t, "archives", cfg.OutputDir)
	assert.Equal(t, 4, cfg.Concurrency)
	assert.Equal(t, 2, cfg.Retries)
	assert.Equal(t, 20, cfg.SearchLimit)
	assert.Equal(t, os.TempDir(), cfg.TempDir)
	assert.Equal(t, config.ThemeAuto, cfg.Theme)
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestLoad_OverridesFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
registry_prefix: rgy.local
platform: linux/arm64
concurrency: 8
retries: 5
search_limit: 50
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "rgy.local", cfg.RegistryPrefix)
	assert.Equal(t, "linux/arm64", cfg.Platform)
	assert.Equal(t, 8, cfg.Concurrency)
	assert.Equal(t, 5, cfg.Retries)
	assert.Equal(t, 50, cfg.SearchLimit)
	// Unset fields keep their defaults.
	assert.Equal(t, "helm", cfg.HelmBin)
}

func TestLoad_InvalidYAMLReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("registry_prefix: [unterminated"), 0o644))
	_, err := config.Load(path)
	assert.Error(t, err)
}

func TestDefaultPath_EndsWithConventionalLocation(t *testing.T) {
	p := config.DefaultPath()
	if p == "" {
		t.Skip("user config dir unavailable")
	}
	assert.Equal(t, filepath.Join("helmdownloader", "config.yaml"), filepath.Join(filepath.Base(filepath.Dir(p)), filepath.Base(p)))
}

func TestDefaultPath_PrefersExistingHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	t.Setenv("XDG_CONFIG_HOME", "")
	// Clear AppData so UserConfigDir does not pull a system path on Windows.
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	xdgDir := filepath.Join(home, ".config", "helmdownloader")
	require.NoError(t, os.MkdirAll(xdgDir, 0o755))
	want := filepath.Join(xdgDir, "config.yaml")
	require.NoError(t, os.WriteFile(want, []byte("theme: ocean\n"), 0o644))
	// Also plant a UserConfigDir-style file so preference is explicit when both exist.
	appSupport := filepath.Join(home, "Library", "Application Support", "helmdownloader")
	require.NoError(t, os.MkdirAll(appSupport, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(appSupport, "config.yaml"), []byte("theme: dark\n"), 0o644))
	assert.Equal(t, want, config.DefaultPath())
}

func TestDefaultPath_FallsBackToUserConfigDirWhenOnlyThere(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	// Create only the platform UserConfigDir location.
	userCfg, err := os.UserConfigDir()
	require.NoError(t, err)
	dir := filepath.Join(userCfg, "helmdownloader")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	want := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(want, []byte("theme: dark\n"), 0o644))
	assert.Equal(t, want, config.DefaultPath())
}

func TestDefaultPath_NoFilePrefersXDGStyle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	want := filepath.Join(home, ".config", "helmdownloader", "config.yaml")
	assert.Equal(t, want, config.DefaultPath())
}

func TestDefaultPath_RespectsXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg-config")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	dir := filepath.Join(xdg, "helmdownloader")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	want := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(want, []byte("theme: matrix\n"), 0o644))
	assert.Equal(t, want, config.DefaultPath())
}

func TestLoad_ThemeFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("theme: light\n"), 0o644))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "light", cfg.Theme)
}

func TestLoad_TempDirFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("temp_dir: /custom/tmp\n"), 0o644))
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "/custom/tmp", cfg.TempDir)
}

func TestValidateTheme(t *testing.T) {
	assert.NoError(t, config.ValidateTheme(""))
	assert.NoError(t, config.ValidateTheme("auto"))
	assert.NoError(t, config.ValidateTheme("light"))
	assert.NoError(t, config.ValidateTheme("DARK"))
	assert.NoError(t, config.ValidateTheme("high-contrast"))
	assert.NoError(t, config.ValidateTheme("OCEAN"))
	assert.NoError(t, config.ValidateTheme("matrix"))
	assert.Error(t, config.ValidateTheme("sepia"))
}

func TestNormalizeTheme(t *testing.T) {
	assert.Equal(t, config.ThemeAuto, config.NormalizeTheme(""))
	assert.Equal(t, config.ThemeLight, config.NormalizeTheme(" Light "))
	assert.Equal(t, config.ThemeDark, config.NormalizeTheme("DARK"))
	assert.Equal(t, config.ThemeHighContrast, config.NormalizeTheme(" High-Contrast "))
}

func TestThemeMenuIndex(t *testing.T) {
	assert.Equal(t, 0, config.ThemeMenuIndex(config.ThemeAuto))
	assert.Equal(t, 0, config.ThemeMenuIndex(""))
	assert.Equal(t, 1, config.ThemeMenuIndex(config.ThemeLight))
	assert.Equal(t, 2, config.ThemeMenuIndex(config.ThemeDark))
	assert.Equal(t, 3, config.ThemeMenuIndex(config.ThemeHighContrast))
	assert.Equal(t, 4, config.ThemeMenuIndex(config.ThemeOcean))
	assert.Equal(t, 5, config.ThemeMenuIndex(config.ThemeMatrix))
	assert.Equal(t, 0, config.ThemeMenuIndex("unknown"))
	assert.Equal(t, len(config.ThemeMenu), 6)
}

func TestThemeIsForced(t *testing.T) {
	assert.False(t, config.ThemeIsForced(config.ThemeAuto))
	assert.False(t, config.ThemeIsForced(""))
	assert.True(t, config.ThemeIsForced(config.ThemeLight))
	assert.True(t, config.ThemeIsForced(config.ThemeMatrix))
}

func TestFindWritableTempDir_UsesUsablePreferred(t *testing.T) {
	preferred := t.TempDir()
	dir, warn, err := config.FindWritableTempDir(preferred)
	require.NoError(t, err)
	assert.Equal(t, preferred, dir)
	assert.Empty(t, warn)
}

func TestFindWritableTempDir_FallsBackWhenPreferredNotWritable(t *testing.T) {
	// Use a regular file as a non-directory candidate so MkdirAll fails.
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
	dir, warn, err := config.FindWritableTempDir(file)
	require.NoError(t, err)
	assert.NotEqual(t, file, dir)
	assert.Contains(t, warn, "not writable")
	assert.NotEmpty(t, dir)
}
