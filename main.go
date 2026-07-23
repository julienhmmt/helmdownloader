// Command helmdownloader is a TUI for downloading Helm charts and their
// container images, then bundling them for airgapped infrastructure.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/julienhmmt/helmdownloader/internal/tui"
	"github.com/julienhmmt/helmdownloader/pkg/batch"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/helm"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/version"
)

// stringSlice is a flag.Value that accumulates repeated flag occurrences, so
// "-values a.yaml -values b.yaml" yields ["a.yaml", "b.yaml"].
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "verify":
			runVerify(os.Args[2:])
			return
		case "diff":
			runDiff(os.Args[2:])
			return
		case "batch":
			runBatch(os.Args[2:])
			return
		case "version", "-version", "--version":
			fmt.Println(version.String())
			return
		}
	}
	var valuesFiles, setValues stringSlice
	flag.Var(&valuesFiles, "values", "extra values file for image discovery (repeatable)")
	flag.Var(&setValues, "set", "values override key=value for image discovery (repeatable)")
	configPath := flag.String("config", config.DefaultPath(), "path to config file")
	outputDir := flag.String("output", "", "override output directory for bundles")
	workDir := flag.String("work-dir", "", "override work directory for intermediate files (charts, images)")
	tempDir := flag.String("temp-dir", "", "override the parent directory for temporary work dirs (default: system temp dir, e.g. /tmp)")
	concurrency := flag.Int("concurrency", 0, "override max parallel image downloads (default 4)")
	retries := flag.Int("retries", -1, "override retry attempts per failed image pull (default 2)")
	prefix := flag.String("registry-prefix", "", "override the private registry prefix")
	platform := flag.String("platform", "", "override the image platform (e.g. linux/amd64)")
	resume := flag.Bool("resume", false, "reuse image tarballs already present in a persistent work dir")
	registryAuth := flag.Bool("registry-auth", false, "enable authenticated pulls from private registries using the default Docker keychain ($DOCKER_CONFIG or ~/.docker/config.json)")
	compression := flag.String("compression", "", "bundle compression: gzip (default) or zstd")
	minFreeDiskMB := flag.Int("min-free-mb", -1, "minimum free disk space in MiB before download (0 disables)")
	proxy := flag.String("proxy", "", "override proxy URL (e.g. http://proxy.domain.local:3128)")
	verbose := flag.Bool("v", false, "enable verbose logging (shortcut for --log-level=debug)")
	logLevel := flag.String("log-level", "", "set log level: silent, info, or debug (default: info)")
	logFile := flag.String("log-file", "helmdownloader.log", "path for log output")
	exportImages := flag.String("export-images", "", "write the discovered image list (JSON) to this path after rendering, for security review")
	importImages := flag.String("import-images", "", "read an approved image list (JSON) from this path when entering Review, overriding the discovered set")
	theme := flag.String("theme", "", "TUI theme: auto (default, follow terminal), light, dark, high-contrast, ocean, or matrix")
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
	if *tempDir != "" {
		cfg.TempDir = *tempDir
	}
	if cfg.WorkDir != "" {
		if err := config.EnsureWritableDir(cfg.WorkDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: work dir is not usable: %v\n", err)
			os.Exit(1)
		}
	} else {
		resolvedTemp, warn, err := config.FindWritableTempDir(cfg.TempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if warn != "" {
			fmt.Fprintln(os.Stderr, warn)
		}
		cfg.TempDir = resolvedTemp
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
	if len(valuesFiles) > 0 {
		cfg.ValuesFiles = valuesFiles
	}
	if len(setValues) > 0 {
		cfg.SetValues = setValues
	}
	if *resume {
		cfg.Resume = true
	}
	if *registryAuth {
		cfg.RegistryAuth = true
	}
	if *compression != "" {
		cfg.Compression = *compression
	}
	if *minFreeDiskMB >= 0 {
		cfg.MinFreeDiskMB = *minFreeDiskMB
	}
	if *proxy != "" {
		cfg.HTTPSProxy = *proxy
	}
	cfg = applyProxyEnv(cfg)
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
	if *exportImages != "" {
		cfg.ExportImages = *exportImages
	}
	if *importImages != "" {
		cfg.ImportImages = *importImages
	}
	if *theme != "" {
		cfg.Theme = *theme
	}
	cfg.Theme = config.NormalizeTheme(cfg.Theme)

	if err := bundle.ValidateCompression(cfg.Compression); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := config.ValidateTheme(cfg.Theme); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
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

// applyProxyEnv fills cfg.HTTPSProxy from HTTP_PROXY/HTTPS_PROXY when it was not
// set via CLI or config, returning the updated copy.
func applyProxyEnv(cfg config.Config) config.Config {
	if cfg.HTTPSProxy != "" {
		return cfg
	}
	if envProxy := os.Getenv("HTTP_PROXY"); envProxy != "" {
		cfg.HTTPSProxy = envProxy
	} else if envProxy := os.Getenv("HTTPS_PROXY"); envProxy != "" {
		cfg.HTTPSProxy = envProxy
	}
	return cfg
}

// runBatch runs the batch subcommand: `helmdownloader batch [-config path] <list.yaml>`.
// It downloads every chart in the YAML list headlessly. All settings other than
// the config path come from the config file (registry prefix, output dir, etc.).
func runBatch(args []string) {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to config file")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: helmdownloader batch [-config path] <charts.yaml>")
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg = applyProxyEnv(cfg)
	if err := bundle.ValidateCompression(cfg.Compression); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	logger := createLogger(cfg)

	data, err := os.ReadFile(rest[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read chart list: %v\n", err)
		os.Exit(1)
	}
	refs, err := batch.ParseList(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := helm.New(cfg.HelmBin, cfg.HTTPSProxy, logger).Check(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := batch.Run(ctx, cfg, logger, refs, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "batch: %v\n", err)
		os.Exit(1)
	}
}

func createLogger(cfg config.Config) *log.Logger {
	// verbose is the primary switch; an explicit log_level: debug in the config
	// also enables logging, so the config-only batch path can turn on logs
	// without a -v flag (info stays gated so default runs write no log file).
	if !cfg.Verbose && cfg.LogLevel != "debug" {
		return log.Discard()
	}
	level := parseLogLevel(cfg.LogLevel)
	// 0o600 so image refs, proxy hosts, and chart names in logs are not world-readable.
	f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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

// runVerify runs the verify subcommand: `helmdownloader verify <bundle>`.
func runVerify(args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: helmdownloader verify <bundle.tar.gz>\n")
		os.Exit(2)
	}
	if err := bundle.Verify(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok: %s is intact\n", args[0])
}

// runDiff runs the diff subcommand: `helmdownloader diff <a> <b>`.
func runDiff(args []string) {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: helmdownloader diff <bundle-a> <bundle-b>\n")
		os.Exit(2)
	}
	result, err := bundle.Diff(args[0], args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff: %v\n", err)
		os.Exit(1)
	}
	printDiff(result)
}

func printDiff(r bundle.DiffResult) {
	if len(r.Added) == 0 && len(r.Removed) == 0 && len(r.Changed) == 0 {
		fmt.Println("no image differences")
		return
	}
	for _, ref := range r.Added {
		fmt.Printf("+ %s\n", ref)
	}
	for _, ref := range r.Removed {
		fmt.Printf("- %s\n", ref)
	}
	for _, c := range r.Changed {
		fmt.Printf("~ %s\n    %s -> %s\n", c.Ref, digestOrNone(c.FromDigest), digestOrNone(c.ToDigest))
	}
}

func digestOrNone(d string) string {
	if d == "" {
		return "(none)"
	}
	return d
}
