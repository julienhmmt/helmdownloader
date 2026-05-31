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
	"github.com/julienhmmt/helmdownloader/internal/log"
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
	logger *log.Logger
}

// New returns a Pipeline configured from cfg.
func New(cfg config.Config, logger *log.Logger) *Pipeline {
	return &Pipeline{
		cfg:    cfg,
		helm:   helm.New(cfg.HelmBin, cfg.HTTPSProxy, logger),
		puller: registry.NewPuller(cfg.Platform, cfg.HTTPSProxy, logger),
		logger: logger,
	}
}

// Prepare pulls the chart at version, renders it, and extracts its images.
func (p *Pipeline) Prepare(ctx context.Context, pkg artifacthub.Package, version string) (prep Prepared, err error) {
	p.logger.Infof("preparing chart %s version %s from %s", pkg.Name, version, pkg.RepoURL)
	var workDir string
	if p.cfg.WorkDir != "" {
		workDir = p.cfg.WorkDir
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return Prepared{}, fmt.Errorf("create work dir: %w", err)
		}
	} else {
		workDir, err = os.MkdirTemp("", "helmdownloader-")
		if err != nil {
			return Prepared{}, fmt.Errorf("create temp work dir: %w", err)
		}
		defer func() {
			if err != nil {
				_ = os.RemoveAll(workDir)
			}
		}()
	}
	p.logger.Debugf("work dir: %s", workDir)
	pull, err := p.helm.Pull(ctx, pkg.Name, pkg.RepoURL, version, workDir, pkg.IsOCI())
	if err != nil {
		return Prepared{}, err
	}
	p.logger.Debugf("pulled chart to %s", pull.ChartPath)
	values, err := p.helm.ShowValues(ctx, pull.ChartPath)
	if err != nil {
		return Prepared{}, err
	}
	p.logger.Debugf("loaded values.yaml (%d bytes)", len(values))
	manifests, err := p.helm.Template(ctx, pull.ChartPath)
	if err != nil {
		return Prepared{}, err
	}
	p.logger.Debugf("templated manifests (%d bytes)", len(manifests))
	extracted := images.Extract(manifests)
	p.logger.Infof("discovered %d images", len(extracted))
	for _, img := range extracted {
		p.logger.Debugf("  image: %s", img.Ref)
	}
	prep = Prepared{
		ChartPath: pull.ChartPath,
		Values:    values,
		Images:    extracted,
		WorkDir:   workDir,
	}
	return prep, nil
}

// ProgressFunc reports download progress as each image is processed.
type ProgressFunc func(current, total int, ref string, err error)

// ImageFailure records an image reference that could not be downloaded and the
// error that prevented it.
type ImageFailure struct {
	Ref string
	Err error
}

// Download saves the given image references into the work directory, returning
// the successful bundle entries and any failures. It does not assemble the
// bundle, so callers can present failures to the user and retry the failed
// references before committing to a bundle.
func (p *Pipeline) Download(ctx context.Context, prepared Prepared, refs []string, progress ProgressFunc) ([]bundle.ImageEntry, []ImageFailure, error) {
	p.logger.Infof("downloading %d images", len(refs))
	imagesDir := filepath.Join(prepared.WorkDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create images dir: %w", err)
	}
	entries := make([]bundle.ImageEntry, 0, len(refs))
	failures := make([]ImageFailure, 0)
	for index, ref := range refs {
		destRef := images.Retag(ref, p.cfg.RegistryPrefix)
		tarPath := filepath.Join(imagesDir, tarballName(ref))
		p.logger.Debugf("saving image %d/%d: %s -> %s", index+1, len(refs), ref, destRef)
		err := p.puller.Save(ctx, ref, destRef, tarPath)
		if progress != nil {
			progress(index+1, len(refs), ref, err)
		}
		if err != nil {
			p.logger.Errorf("failed to save %s: %v", ref, err)
			failures = append(failures, ImageFailure{Ref: ref, Err: err})
			continue
		}
		entries = append(entries, bundle.ImageEntry{
			TarPath:   tarPath,
			SourceRef: ref,
			DestRef:   destRef,
		})
	}
	return entries, failures, nil
}

// Bundle assembles the downloaded image entries, the chart, and its values into
// a single archive and returns its path. It then cleans up intermediate
// artifacts. At least one entry is required.
func (p *Pipeline) Bundle(prepared Prepared, pkg artifacthub.Package, version string, entries []bundle.ImageEntry) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("no images were successfully downloaded")
	}
	p.logger.Infof("creating bundle for %s %s with %d images", pkg.Name, version, len(entries))
	bundlePath, err := bundle.Create(bundle.Spec{
		ChartName:    pkg.Name,
		ChartVersion: version,
		ChartPath:    prepared.ChartPath,
		Values:       prepared.Values,
		Images:       entries,
		OutputDir:    p.cfg.OutputDir,
	})
	if err != nil {
		return "", err
	}
	p.logger.Infof("bundle created: %s", bundlePath)

	// Clean up intermediate artifacts from the work directory.
	// For temp work dirs, the entire dir is cleaned up by the caller.
	// For persistent work dirs, we remove the images subdirectory and the
	// pulled chart archive, both of which are already embedded in the bundle.
	if p.cfg.WorkDir != "" {
		imagesDir := filepath.Join(prepared.WorkDir, "images")
		if err := os.RemoveAll(imagesDir); err != nil {
			p.logger.Debugf("failed to clean up images directory: %v", err)
		}
		if err := os.Remove(prepared.ChartPath); err != nil && !os.IsNotExist(err) {
			p.logger.Debugf("failed to clean up chart archive %s: %v", prepared.ChartPath, err)
		}
	}

	return bundlePath, nil
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// tarballName derives a filesystem-safe tar filename from an image reference.
func tarballName(ref string) string {
	safe := unsafeChars.ReplaceAllString(ref, "_")
	safe = strings.Trim(safe, "_")
	return fmt.Sprintf("%s.tar", safe)
}
