// Package pipeline orchestrates the end-to-end flow: pulling a chart, rendering
// it to discover images, then downloading those images and assembling a bundle.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/julienhmmt/helmdownloader/internal/artifacthub"
	"github.com/julienhmmt/helmdownloader/internal/bundle"
	"github.com/julienhmmt/helmdownloader/internal/config"
	"github.com/julienhmmt/helmdownloader/internal/helm"
	"github.com/julienhmmt/helmdownloader/internal/images"
	"github.com/julienhmmt/helmdownloader/internal/registry"
)

// Prepared holds the result of pulling and rendering a chart, ready for the
// user to review the discovered images before downloading.
type Prepared struct {
	// ChartPath is the pulled chart archive on disk.
	ChartPath string
	// Values is the chart's default values.yaml.
	Values string
	// Images is the auto-discovered set of image references.
	Images []images.Image
	// WorkDir is the temporary directory holding intermediate artifacts.
	WorkDir string
}

// Pipeline runs chart preparation and bundle creation using the configured
// helm client and image puller.
type Pipeline struct {
	cfg    config.Config
	helm   *helm.Client
	puller *registry.Puller
}

// New returns a Pipeline configured from cfg.
func New(cfg config.Config) *Pipeline {
	return &Pipeline{
		cfg:    cfg,
		helm:   helm.New(cfg.HelmBin, cfg.HTTPSProxy),
		puller: registry.NewPuller(cfg.Platform),
	}
}

// Prepare pulls the chart at version, renders it, and extracts its images.
func (p *Pipeline) Prepare(ctx context.Context, pkg artifacthub.Package, version string) (Prepared, error) {
	workDir, err := os.MkdirTemp("", "helmdownloader-")
	if err != nil {
		return Prepared{}, err
	}
	pull, err := p.helm.Pull(ctx, pkg.Name, pkg.RepoURL, version, workDir, pkg.IsOCI())
	if err != nil {
		return Prepared{}, err
	}
	values, err := p.helm.ShowValues(ctx, pull.ChartPath)
	if err != nil {
		return Prepared{}, err
	}
	manifests, err := p.helm.Template(ctx, pull.ChartPath)
	if err != nil {
		return Prepared{}, err
	}
	return Prepared{
		ChartPath: pull.ChartPath,
		Values:    values,
		Images:    images.Extract(manifests),
		WorkDir:   workDir,
	}, nil
}

// ProgressFunc reports download progress as each image is processed.
type ProgressFunc func(current, total int, ref string, err error)

// Build downloads the selected images and writes a single bundle archive,
// returning its path. Only images with Selected == true are included.
func (p *Pipeline) Build(prepared Prepared, pkg artifacthub.Package, version string, progress ProgressFunc) (string, error) {
	selected := make([]images.Image, 0, len(prepared.Images))
	for _, img := range prepared.Images {
		if img.Selected {
			selected = append(selected, img)
		}
	}
	imagesDir := filepath.Join(prepared.WorkDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", err
	}
	entries := make([]bundle.ImageEntry, 0, len(selected))
	for index, img := range selected {
		destRef := images.Retag(img.Ref, p.cfg.RegistryPrefix)
		tarPath := filepath.Join(imagesDir, tarballName(img.Ref))
		err := p.puller.Save(img.Ref, destRef, tarPath)
		if progress != nil {
			progress(index+1, len(selected), img.Ref, err)
		}
		if err != nil {
			return "", err
		}
		entries = append(entries, bundle.ImageEntry{
			TarPath:   tarPath,
			SourceRef: img.Ref,
			DestRef:   destRef,
		})
	}
	return bundle.Create(bundle.Spec{
		ChartName:    pkg.Name,
		ChartVersion: version,
		ChartPath:    prepared.ChartPath,
		Values:       prepared.Values,
		Images:       entries,
		OutputDir:    p.cfg.OutputDir,
	})
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// tarballName derives a filesystem-safe tar filename from an image reference.
func tarballName(ref string) string {
	safe := unsafeChars.ReplaceAllString(ref, "_")
	safe = strings.Trim(safe, "_")
	return fmt.Sprintf("%s.tar", safe)
}
