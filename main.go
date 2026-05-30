// Command helmdownloader is a TUI for downloading Helm charts and their
// container images, then bundling them for airgapped infrastructure.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/julienhmmt/helmdownloader/internal/config"
	"github.com/julienhmmt/helmdownloader/internal/tui"
)

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to config file")
	outputDir := flag.String("output", "", "override output directory for bundles")
	prefix := flag.String("registry-prefix", "", "override the private registry prefix")
	platform := flag.String("platform", "", "override the image platform (e.g. linux/amd64)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	if *outputDir != "" {
		cfg.OutputDir = *outputDir
	}
	if *prefix != "" {
		cfg.RegistryPrefix = *prefix
	}
	if *platform != "" {
		cfg.Platform = *platform
	}

	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
