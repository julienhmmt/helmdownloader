// Package registry pulls container images directly from registries (no Docker
// daemon) and writes them to docker-compatible tar archives, retagged for the
// destination private registry.
package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/julienhmmt/helmdownloader/internal/log"
)

// Puller fetches and saves images for a fixed target platform.
type Puller struct {
	platform string
	proxy    string
	logger   *log.Logger
}

// NewPuller returns a Puller that pulls images for the given platform string,
// e.g. "linux/amd64".
func NewPuller(platform, proxy string, logger *log.Logger) *Puller {
	if platform == "" {
		platform = "linux/amd64"
	}
	return &Puller{platform: platform, proxy: proxy, logger: logger}
}

// Save pulls srcRef for the configured platform and writes it to destPath as a
// docker-style tarball, embedding destRef as the image's tag so a later
// "docker load" yields the retagged image ready to push to the airgap registry.
func (p *Puller) Save(ctx context.Context, srcRef, destRef, destPath string) error {
	p.logger.Infof("pulling image %s for platform %s", srcRef, p.platform)

	platform, err := v1.ParsePlatform(p.platform)
	if err != nil {
		return fmt.Errorf("parse platform %q: %w", p.platform, err)
	}

	// Build crane options with proxy support
	opts := []crane.Option{crane.WithPlatform(platform)}
	if p.proxy != "" {
		proxyURL, err := url.Parse(p.proxy)
		if err != nil {
			return fmt.Errorf("parse proxy URL %q: %w", p.proxy, err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		opts = append(opts, crane.WithTransport(transport))
	}

	// Pull the image. The timeout configuration is not used here because
	// crane.Pull returns a lazy-loading image that retains the context,
	// which would cause crane.Save to timeout during layer downloads.
	img, err := crane.Pull(srcRef, append(opts, crane.WithContext(ctx))...)
	if err != nil {
		return fmt.Errorf("pull %s: %w", srcRef, err)
	}

	tag, err := name.NewTag(destRef)
	if err != nil {
		return fmt.Errorf("parse dest ref %q: %w", destRef, err)
	}

	p.logger.Debugf("saving %s to %s", destRef, destPath)

	// Atomic writability check: try to create and immediately remove a temp file
	testFile := destPath + ".tmp"
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("cannot create file in destination directory: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(testFile)
		return fmt.Errorf("cannot close test file: %w", err)
	}
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("cannot remove test file: %w", err)
	}

	// Save performs the heavy layer download and tarball write.
	if err := crane.Save(img, tag.Name(), destPath); err != nil {
		return fmt.Errorf("save %s: %w", destRef, err)
	}

	p.logger.Infof("saved %s", destPath)
	return nil
}
