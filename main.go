// Command helmdownloader is a TUI for downloading Helm charts and their
// container images, then bundling them for airgapped infrastructure.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/julienhmmt/helmdownloader/internal/tui"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/helm"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to config file")
	outputDir := flag.String("output", "", "override output directory for bundles")
	workDir := flag.String("work-dir", "", "override work directory for intermediate files (charts, images)")
	concurrency := flag.Int("concurrency", 0, "override max parallel image downloads (default 4)")
	retries := flag.Int("retries", -1, "override retry attempts per failed image pull (default 2)")
	prefix := flag.String("registry-prefix", "", "override the private registry prefix")
	platform := flag.String("platform", "", "override the image platform (e.g. linux/amd64)")
	proxy := flag.String("proxy", "", "override proxy URL (e.g. http://proxy.domain.local:3128)")
	verbose := flag.Bool("v", false, "enable verbose logging (shortcut for --log-level=debug)")
	logLevel := flag.String("log-level", "", "set log level: silent, info, or debug (default: info)")
	logFile := flag.String("log-file", "helmdownloader.log", "path for log output")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	if *outputDir != "" {
		cfg.OutputDir = *outputDir
	}
	if *workDir != "" {
		cfg.WorkDir = *workDir
	}
	if *concurrency > 0 {
		cfg.Concurrency = *concurrency
	}
	if *retries >= 0 {
		cfg.Retries = *retries
	}
	if *prefix != "" {
		cfg.RegistryPrefix = *prefix
	}
	if *platform != "" {
		cfg.Platform = *platform
	}
	if *proxy != "" {
		cfg.HTTPSProxy = *proxy
	}
	// Check environment variables if proxy not set via CLI or config
	if cfg.HTTPSProxy == "" {
		if envProxy := os.Getenv("HTTP_PROXY"); envProxy != "" {
			cfg.HTTPSProxy = envProxy
		} else if envProxy := os.Getenv("HTTPS_PROXY"); envProxy != "" {
			cfg.HTTPSProxy = envProxy
		}
	}
	if *verbose {
		cfg.Verbose = true
		cfg.LogLevel = "debug"
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
		cfg.Verbose = true
	}
	if cfg.LogFile == "" {
		cfg.LogFile = *logFile
	}

	logger := createLogger(cfg)

	// Preflight: fail fast with a clear message if helm is missing or broken,
	// rather than surfacing a cryptic error after the user picks a chart.
	if err := helm.New(cfg.HelmBin, cfg.HTTPSProxy, logger).Check(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := tui.Run(cfg, logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createLogger(cfg config.Config) *log.Logger {
	if !cfg.Verbose {
		return log.Discard()
	}
	level := parseLogLevel(cfg.LogLevel)
	f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open log file %s: %v\n", cfg.LogFile, err)
		return log.Discard()
	}
	return log.New(f, level)
}

func parseLogLevel(level string) log.Level {
	switch level {
	case "silent":
		return log.LevelSilent
	case "debug":
		return log.LevelDebug
	case "info":
		return log.LevelInfo
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown log level %q, using info\n", level)
		return log.LevelInfo
	}
}
