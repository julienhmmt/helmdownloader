// Package helm wraps the helm binary for the chart operations needed to build
// an airgap bundle: pulling a chart, rendering it, and dumping its values.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/julienhmmt/helmdownloader/internal/log"
)

// Client invokes the helm executable.
type Client struct {
	bin    string
	proxy  string
	logger *log.Logger
}

// New returns a Client using the given helm binary. proxy, when non-empty, is
// exported as HTTPS_PROXY for every invocation (mirroring the original scripts).
func New(bin, proxy string, logger *log.Logger) *Client {
	if bin == "" {
		bin = "helm"
	}
	return &Client{bin: bin, proxy: proxy, logger: logger}
}

// PullResult describes a chart fetched to disk.
type PullResult struct {
	// ChartPath is the path to the downloaded .tgz archive.
	ChartPath string
	// Dir is the directory the chart was pulled into.
	Dir string
}

// Pull downloads chart version from its repository into destDir. repoURL is the
// Helm repo URL from ArtifactHub; oci indicates an OCI-hosted chart.
func (c *Client) Pull(ctx context.Context, name, repoURL, version, destDir string, oci bool) (PullResult, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return PullResult{}, err
	}
	args := []string{"pull"}
	if oci {
		args = append(args, strings.TrimRight(repoURL, "/"))
	} else {
		args = append(args, name, "--repo", repoURL)
	}
	args = append(args, "--version", version, "--destination", destDir)
	c.logger.Debugf("helm pull: %s %s", c.bin, strings.Join(args, " "))
	if _, err := c.run(ctx, args...); err != nil {
		return PullResult{}, err
	}
	chartPath, err := findChart(destDir, name, version)
	if err != nil {
		return PullResult{}, err
	}
	return PullResult{ChartPath: chartPath, Dir: destDir}, nil
}

// Template renders the chart archive at chartPath with default values and
// returns the concatenated manifest YAML.
func (c *Client) Template(ctx context.Context, chartPath string) (string, error) {
	c.logger.Debugf("helm template: %s", chartPath)
	out, err := c.run(ctx, "template", "release", chartPath)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ShowValues returns the default values.yaml of the chart archive at chartPath.
func (c *Client) ShowValues(ctx context.Context, chartPath string) (string, error) {
	c.logger.Debugf("helm show values: %s", chartPath)
	return c.run(ctx, "show", "values", chartPath)
}

// run executes helm with args and returns stdout, wrapping failures with stderr.
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Env = os.Environ()
	if c.proxy != "" {
		cmd.Env = append(cmd.Env, "HTTPS_PROXY="+c.proxy, "HTTP_PROXY="+c.proxy)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("helm %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// findChart locates the pulled archive, preferring the conventional
// "<name>-<version>.tgz" filename and falling back to any .tgz in dir.
func findChart(dir, name, version string) (string, error) {
	expected := filepath.Join(dir, fmt.Sprintf("%s-%s.tgz", name, version))
	if _, err := os.Stat(expected); err == nil {
		return expected, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tgz") {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no chart archive found in %s", dir)
}
