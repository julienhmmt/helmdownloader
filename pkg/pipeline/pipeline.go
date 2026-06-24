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
	// WorkDir is the directory holding intermediate artifacts.
	WorkDir string
	// TempWorkDir reports whether WorkDir was created as a temporary directory
	// by Prepare and should be removed on cleanup; it is false when WorkDir is
	// the user-configured cfg.WorkDir, which must be preserved across runs for
	// -resume.
	TempWorkDir bool
}

// imageSaver pulls a source image and writes it to a tarball retagged as
// destRef, returning the resolved manifest digest. onBytes, when non-nil,
// receives byte-level progress during the write. *registry.Puller is the
// production implementation; tests substitute a fake.
type imageSaver interface {
	Save(ctx context.Context, srcRef, destRef, destPath string, onBytes registry.BytesFunc) (string, error)
}

// helmClient is the subset of *helm.Client that Pipeline uses. The concrete
// client satisfies it structurally; tests substitute a fake to drive Prepare
// without a real helm binary or network.
type helmClient interface {
	Pull(ctx context.Context, name, repoURL, version, destDir string, oci bool) (helm.PullResult, error)
	ShowValues(ctx context.Context, chartPath string) (string, error)
	Template(ctx context.Context, chartPath string, opts ...helm.TemplateOption) (string, error)
	SubchartValues(chartPath string) ([]string, error)
}

// defaultRetryBaseDelay is the first backoff interval; it doubles each retry.
const defaultRetryBaseDelay = 1 * time.Second

// Pipeline runs chart preparation and bundle creation using the configured
// helm client and image puller.
type Pipeline struct {
	cfg    config.Config
	helm   helmClient
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
	var tempDir bool
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
		tempDir = true
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
	var templateOpts []helm.TemplateOption
	for _, f := range p.cfg.ValuesFiles {
		templateOpts = append(templateOpts, helm.WithValuesFile(f))
	}
	for _, kv := range p.cfg.SetValues {
		templateOpts = append(templateOpts, helm.WithSetValue(kv))
	}
	if len(templateOpts) > 0 {
		p.logger.Infof("rendering with %d values file(s) and %d --set override(s) for discovery", len(p.cfg.ValuesFiles), len(p.cfg.SetValues))
	}
	manifests, err := p.helm.Template(ctx, pull.ChartPath, templateOpts...)
	if err != nil {
		return Prepared{}, err
	}
	p.logger.Debugf("templated manifests (%d bytes)", len(manifests))
	// Scan the rendered manifests, the chart's top-level values.yaml, and every
	// subchart values.yaml. Values often declare images for components disabled
	// in the default render (using the split registry/repository/tag form),
	// which the manifests alone would miss; subcharts hide them one level deeper.
	sources := []string{manifests, values}
	if subValues, err := p.helm.SubchartValues(pull.ChartPath); err != nil {
		p.logger.Debugf("could not scan subchart values: %v", err)
	} else {
		sources = append(sources, subValues...)
	}
	extracted := images.Extract(sources...)
	p.logger.Infof("discovered %d images", len(extracted))
	for _, img := range extracted {
		p.logger.Debugf("  image: %s", img.Ref)
	}
	prep = Prepared{
		ChartPath:   pull.ChartPath,
		Values:      values,
		Images:      extracted,
		WorkDir:     workDir,
		TempWorkDir: tempDir,
	}
	return prep, nil
}

// ProgressFunc reports download progress as each image is processed.
type ProgressFunc func(current, total int, ref string, err error)

// ByteProgressFunc reports byte-level progress for an in-flight image pull.
// total is best-effort and may be 0 when the registry does not report sizes.
type ByteProgressFunc func(ref string, written, total int64)

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
func (p *Pipeline) Download(ctx context.Context, prepared Prepared, refs []string, progress ProgressFunc, byteProgress ByteProgressFunc) ([]bundle.ImageEntry, []ImageFailure, error) {
	limit := p.concurrency()
	p.logger.Infof("downloading %d images (concurrency %d)", len(refs), limit)
	imagesDir := filepath.Join(prepared.WorkDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create images dir: %w", err)
	}
	if err := p.checkDiskSpace(imagesDir); err != nil {
		return nil, nil, err
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

			// Resume: reuse a tarball already on disk from a prior run instead of
			// pulling it again. The digest sidecar restores the manifest digest so
			// the bundle stays fully pinned.
			if p.cfg.Resume {
				if digest, ok := reusableTarball(tarPath); ok {
					p.logger.Infof("reusing existing tarball for %s", ref)
					mu.Lock()
					completed++
					done := completed
					results[index] = result{entry: &bundle.ImageEntry{
						TarPath: tarPath, SourceRef: ref, DestRef: destRef, Digest: digest,
					}}
					mu.Unlock()
					if progress != nil {
						progress(done, len(refs), ref, nil)
					}
					return nil
				}
			}

			p.logger.Debugf("saving image %d/%d: %s -> %s", index+1, len(refs), srcRef, destRef)
			var onBytes registry.BytesFunc
			if byteProgress != nil {
				onBytes = func(written, total int64) { byteProgress(ref, written, total) }
			}
			digest, err := p.saveWithRetry(groupCtx, srcRef, destRef, tarPath, onBytes)

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
				// Record the digest beside the tarball so a later --resume run can
				// reuse it without re-pulling and still pin the bundle.
				writeDigestSidecar(tarPath, digest)
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

// checkDiskSpace fails fast when the filesystem backing dir has less free space
// than the configured minimum, so a long download does not abort mid-way with a
// cryptic "no space left" error. A 0 threshold, an unsupported platform, or a
// stat error all skip the check rather than block progress.
func (p *Pipeline) checkDiskSpace(dir string) error {
	if p.cfg.MinFreeDiskMB <= 0 {
		return nil
	}
	free, err := freeBytes(dir)
	if err != nil {
		p.logger.Debugf("disk space check skipped: %v", err)
		return nil
	}
	if free == 0 {
		return nil
	}
	const mib = 1 << 20
	needed := uint64(p.cfg.MinFreeDiskMB) * mib
	if free < needed {
		return fmt.Errorf("insufficient disk space in %s: %d MiB free, need at least %d MiB",
			dir, free/mib, p.cfg.MinFreeDiskMB)
	}
	p.logger.Debugf("disk space ok: %d MiB free in %s", free/mib, dir)
	return nil
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
func (p *Pipeline) saveWithRetry(ctx context.Context, srcRef, destRef, tarPath string, onBytes registry.BytesFunc) (string, error) {
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
		if digest, err = p.puller.Save(ctx, srcRef, destRef, tarPath, onBytes); err == nil {
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
		Compression:  p.cfg.Compression,
	})
	if err != nil {
		return "", err
	}
	p.logger.Infof("bundle created: %s", bundlePath)

	// Clean up intermediate artifacts from the work directory.
	// For temp work dirs, the entire dir is cleaned up by the caller.
	// For persistent work dirs, we remove the images subdirectory and the
	// pulled chart archive, both of which are already embedded in the bundle.
	if prepared.WorkDir != "" && !prepared.TempWorkDir {
		imagesDir := filepath.Join(prepared.WorkDir, "images")
		if err := os.RemoveAll(imagesDir); err != nil {
			p.logger.Debugf("failed to clean up images directory: %v", err)
		}
		if err := os.Remove(prepared.ChartPath); err != nil && !os.IsNotExist(err) {
			p.logger.Debugf("failed to clean up chart archive %s: %v", prepared.ChartPath, err)
		}
		// The isolated helm repo cache (created by isolatedHelmEnv during Pull)
		// is per-bundle and regenerable; remove it so persistent work dirs don't
		// accumulate stale repo indexes across runs.
		if err := os.RemoveAll(filepath.Join(prepared.WorkDir, ".helm")); err != nil {
			p.logger.Debugf("failed to clean up isolated helm cache: %v", err)
		}
	}

	return bundlePath, nil
}

// digestSidecarPath returns the path of the file recording a tarball's digest.
func digestSidecarPath(tarPath string) string {
	return tarPath + ".digest"
}

// writeDigestSidecar records digest beside the tarball for later --resume runs.
// Failure is non-fatal: the tarball is still valid, resume just won't repin it.
func writeDigestSidecar(tarPath, digest string) {
	if digest == "" {
		return
	}
	_ = os.WriteFile(digestSidecarPath(tarPath), []byte(digest), 0o600)
}

// reusableTarball reports whether a non-empty tarball already exists at tarPath,
// returning the recorded digest (empty when no sidecar is present).
func reusableTarball(tarPath string) (string, bool) {
	info, err := os.Stat(tarPath)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", false
	}
	data, err := os.ReadFile(digestSidecarPath(tarPath))
	if err != nil {
		return "", true
	}
	return strings.TrimSpace(string(data)), true
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// tarballName derives a filesystem-safe tar filename from an image reference.
func tarballName(ref string) string {
	safe := unsafeChars.ReplaceAllString(ref, "_")
	safe = strings.Trim(safe, "_")
	return fmt.Sprintf("%s.tar", safe)
}
