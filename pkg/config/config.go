// Package config defines the runtime configuration for helmdownloader.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Supported TUI theme names.
const (
	ThemeAuto         = "auto"
	ThemeLight        = "light"
	ThemeDark         = "dark"
	ThemeHighContrast = "high-contrast"
	ThemeOcean        = "ocean"
	ThemeMatrix       = "matrix"
)

// ThemeCycle is the order used when cycling themes in the TUI (Ctrl+T).
// auto is omitted: once the user toggles, they leave terminal-follow mode.
var ThemeCycle = []string{
	ThemeLight,
	ThemeDark,
	ThemeHighContrast,
	ThemeOcean,
	ThemeMatrix,
}

// Config holds all tunable settings for the application.
type Config struct {
	// RegistryPrefix is prepended to every image reference when retagging,
	// e.g. "rgy01.domain.local". The airgapped registry images will be pushed to.
	RegistryPrefix string `yaml:"registry_prefix"`
	// Platform is the OS/arch the images are pulled for, e.g. "linux/amd64".
	Platform string `yaml:"platform"`
	// OutputDir is where chart bundles are written.
	OutputDir string `yaml:"output_dir"`
	// WorkDir is where intermediate files (charts, images) are stored during processing.
	// If empty, a temporary directory is used.
	WorkDir string `yaml:"work_dir"`
	// Concurrency is the maximum number of images downloaded in parallel.
	// Values below 1 are treated as 1.
	Concurrency int `yaml:"concurrency"`
	// Retries is the number of additional attempts made for a failed image
	// pull, on top of the initial try, using exponential backoff. Negative
	// values are treated as 0.
	Retries int `yaml:"retries"`
	// HTTPSProxy, when set, is exported for helm and registry network calls.
	HTTPSProxy string `yaml:"https_proxy"`
	// HelmBin is the helm executable name or path.
	HelmBin string `yaml:"helm_bin"`
	// ArtifactHubURL is the base URL of the ArtifactHub API.
	ArtifactHubURL string `yaml:"artifacthub_url"`
	// SearchLimit caps the number of search results requested.
	SearchLimit int `yaml:"search_limit"`
	// ValuesFiles are extra values files layered onto the chart when rendering
	// for image discovery, so images gated on non-default values are found.
	ValuesFiles []string `yaml:"values_files"`
	// SetValues are "key=value" overrides applied when rendering for image
	// discovery (helm --set), complementing ValuesFiles.
	SetValues []string `yaml:"set_values"`
	// Resume, when true, reuses image tarballs already present in a persistent
	// work directory instead of pulling them again. Only meaningful with a
	// fixed work_dir; a temporary work dir is empty on each run.
	Resume bool `yaml:"resume"`
	// Compression selects the bundle archive codec: "gzip" (default) or "zstd"
	// for a smaller archive.
	Compression string `yaml:"compression"`
	// MinFreeDiskMB is the minimum free space, in MiB, required on the work
	// directory's filesystem before a download starts. 0 disables the check.
	MinFreeDiskMB int `yaml:"min_free_disk_mb"`
	// Theme selects the TUI palette: "auto" (default, follow the terminal),
	// "light", "dark", "high-contrast", "ocean", or "matrix". Named themes
	// (everything except auto) also set a matching terminal background so
	// adaptive text remains readable.
	Theme string `yaml:"theme"`
	// Verbose enables detailed logging to a file.
	Verbose bool `yaml:"verbose"`
	// LogFile is the path where verbose output is written.
	LogFile string `yaml:"log_file"`
	// LogLevel controls logging verbosity: silent, info, or debug.
	LogLevel string `yaml:"log_level"`
	// ExportImages, when set, is the path to write the discovered image list
	// (JSON) after Prepare, so a security team can review it before pulling.
	ExportImages string `yaml:"export_images"`
	// ImportImages, when set, is the path to read an approved image list (JSON)
	// from at download time, overriding the discovered/selected set.
	ImportImages string `yaml:"import_images"`
	// RegistryAuth, when true, enables authenticated pulls using the default
	// Docker keychain (reads ~/.docker/config.json or $DOCKER_CONFIG/config.json,
	// and Podman's containers/auth.json). Use this to pull from private
	// registries; log in with `docker login` (or `podman login`) first.
	RegistryAuth bool `yaml:"registry_auth"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		ArtifactHubURL: "https://artifacthub.io",
		Compression:    "gzip",
		Concurrency:    4,
		MinFreeDiskMB:  500,
		HTTPSProxy:     "",
		HelmBin:        "helm",
		LogLevel:       "info",
		OutputDir:      "archives",
		Platform:       "linux/amd64",
		RegistryPrefix: "",
		Retries:        2,
		SearchLimit:    20,
		Theme:          ThemeAuto,
	}
}

// ValidateTheme reports whether name is a supported TUI theme.
func ValidateTheme(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", ThemeAuto, ThemeLight, ThemeDark, ThemeHighContrast, ThemeOcean, ThemeMatrix:
		return nil
	default:
		return fmt.Errorf("unsupported theme %q (want auto, light, dark, high-contrast, ocean, or matrix)", name)
	}
}

// NormalizeTheme returns a canonical theme name. Empty becomes auto.
func NormalizeTheme(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ThemeAuto
	}
	return n
}

// NextTheme returns the next theme in ThemeCycle after name. Unknown or auto
// names start the cycle at ThemeLight.
func NextTheme(name string) string {
	n := NormalizeTheme(name)
	for i, t := range ThemeCycle {
		if t == n {
			return ThemeCycle[(i+1)%len(ThemeCycle)]
		}
	}
	return ThemeCycle[0]
}

// ThemeIsForced reports whether name paints a fixed palette (not auto).
func ThemeIsForced(name string) bool {
	return NormalizeTheme(name) != ThemeAuto
}

// Load reads configuration from path, falling back to defaults for any
// unset field. A missing file is not an error: defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// DefaultPath returns the conventional config file location under the user's
// config directory, or an empty string when it cannot be determined.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "helmdownloader", "config.yaml")
}
