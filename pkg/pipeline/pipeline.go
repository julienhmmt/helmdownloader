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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/helm"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/registry"
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

// imageSaver pulls a source image and writes it to a tarball retagged as
// destRef, returning the resolved manifest digest. *registry.Puller is the
// production implementation; tests substitute a fake.
type imageSaver interface {
	Save(ctx context.Context, srcRef, destRef, destPath string) (string, error)
}

// defaultRetryBaseDelay is the first backoff interval; it doubles each retry.
const defaultRetryBaseDelay = 1 * time.Second

// Pipeline runs chart preparation and bundle creation using the configured
// helm client and image puller.
type Pipeline struct {
	cfg    config.Config
	helm   *helm.Client
	puller imageSaver
	logger *log.Logger
	// retryBaseDelay is the first backoff interval between pull attempts.
	// Tests shrink it to keep retry coverage fast.
	retryBaseDelay time.Duration
}

// New returns a Pipeline configured from cfg.
func New(cfg config.Config, logger *log.Logger) *Pipeline {
	return &Pipeline{
		cfg:            cfg,
		helm:           helm.New(cfg.HelmBin, cfg.HTTPSProxy, logger),
		puller:         registry.NewPuller(cfg.Platform, cfg.HTTPSProxy, logger),
		logger:         logger,
		retryBaseDelay: defaultRetryBaseDelay,
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
	// Scan both the rendered manifests and the chart's values.yaml. Values
	// often declare images for components disabled in the default render
	// (using the split registry/repository/tag form), which the manifests
	// alone would miss.
	extracted := images.Extract(manifests, values)
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
// the successful bundle entries and any failures. Images are pulled in parallel
// up to the configured concurrency limit. It does not assemble the bundle, so
// callers can present failures to the user and retry the failed references
// before committing to a bundle.
//
// The returned entries and failures preserve the order of refs. Progress is
// reported as each image finishes; current is the number completed so far,
// which may not match the position of ref in refs because pulls finish out of
// order.
func (p *Pipeline) Download(ctx context.Context, prepared Prepared, refs []string, progress ProgressFunc) ([]bundle.ImageEntry, []ImageFailure, error) {
	limit := p.concurrency()
	p.logger.Infof("downloading %d images (concurrency %d)", len(refs), limit)
	imagesDir := filepath.Join(prepared.WorkDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create images dir: %w", err)
	}

	// Each ref writes its outcome to a fixed slot so the final results stay in
	// input order regardless of completion order.
	type result struct {
		entry *bundle.ImageEntry
		fail  *ImageFailure
	}
	results := make([]result, len(refs))

	var (
		mu        sync.Mutex
		completed int
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(limit)
	for index, ref := range refs {
		group.Go(func() error {
			srcRef := images.PullRef(ref)
			destRef := images.Retag(ref, p.cfg.RegistryPrefix)
			tarPath := filepath.Join(imagesDir, tarballName(ref))
			p.logger.Debugf("saving image %d/%d: %s -> %s", index+1, len(refs), srcRef, destRef)
			digest, err := p.saveWithRetry(groupCtx, srcRef, destRef, tarPath)

			mu.Lock()
			completed++
			done := completed
			if err != nil {
				results[index] = result{fail: &ImageFailure{Ref: ref, Err: err}}
			} else {
				results[index] = result{entry: &bundle.ImageEntry{
					TarPath:   tarPath,
					SourceRef: ref,
					DestRef:   destRef,
					Digest:    digest,
				}}
			}
			mu.Unlock()

			if err != nil {
				p.logger.Errorf("failed to save %s: %v", ref, err)
			}
			if progress != nil {
				progress(done, len(refs), ref, err)
			}
			// A failed pull is recorded, not propagated: we want every image
			// attempted so the user sees the full set of failures at once.
			return nil
		})
	}
	_ = group.Wait()

	entries := make([]bundle.ImageEntry, 0, len(refs))
	failures := make([]ImageFailure, 0)
	for _, r := range results {
		switch {
		case r.entry != nil:
			entries = append(entries, *r.entry)
		case r.fail != nil:
			failures = append(failures, *r.fail)
		}
	}
	return entries, failures, nil
}

// concurrency returns the effective parallel download limit, never below 1.
func (p *Pipeline) concurrency() int {
	if p.cfg.Concurrency < 1 {
		return 1
	}
	return p.cfg.Concurrency
}

// retries returns the number of additional pull attempts, never below 0.
func (p *Pipeline) retries() int {
	if p.cfg.Retries < 0 {
		return 0
	}
	return p.cfg.Retries
}

// saveWithRetry pulls srcRef, retrying transient failures with exponential
// backoff up to the configured retry count, and returns the resolved digest on
// success. Backoff waits are cancellable: if ctx is done, the last error is
// returned immediately without sleeping.
func (p *Pipeline) saveWithRetry(ctx context.Context, srcRef, destRef, tarPath string) (string, error) {
	attempts := p.retries() + 1
	delay := p.retryBaseDelay
	if delay <= 0 {
		delay = defaultRetryBaseDelay
	}
	var (
		digest string
		err    error
	)
	for attempt := 1; attempt <= attempts; attempt++ {
		if digest, err = p.puller.Save(ctx, srcRef, destRef, tarPath); err == nil {
			return digest, nil
		}
		if ctx.Err() != nil || attempt == attempts {
			break
		}
		p.logger.Debugf("retry %d/%d for %s after %s: %v", attempt, attempts-1, srcRef, delay, err)
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(delay):
		}
		delay *= 2
	}
	return "", err
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
