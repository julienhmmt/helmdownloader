// Package config defines the runtime configuration for helmdownloader.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all tunable settings for the application.
type Config struct {
	// RegistryPrefix is prepended to every image reference when retagging,
	// e.g. "rgy01.kalos.tdmc". The airgapped registry images will be pushed to.
	RegistryPrefix string `yaml:"registry_prefix"`
	// Platform is the OS/arch the images are pulled for, e.g. "linux/amd64".
	Platform string `yaml:"platform"`
	// OutputDir is where chart bundles are written.
	OutputDir string `yaml:"output_dir"`
	// HTTPSProxy, when set, is exported for helm and registry network calls.
	HTTPSProxy string `yaml:"https_proxy"`
	// HelmBin is the helm executable name or path.
	HelmBin string `yaml:"helm_bin"`
	// ArtifactHubURL is the base URL of the ArtifactHub API.
	ArtifactHubURL string `yaml:"artifacthub_url"`
	// SearchLimit caps the number of search results requested.
	SearchLimit int `yaml:"search_limit"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		RegistryPrefix: "",
		Platform:       "linux/amd64",
		OutputDir:      "archives",
		HTTPSProxy:     "",
		HelmBin:        "helm",
		ArtifactHubURL: "https://artifacthub.io",
		SearchLimit:    20,
	}
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
