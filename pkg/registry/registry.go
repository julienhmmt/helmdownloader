// Package registry pulls container images directly from registries (no Docker
// daemon) and writes them to docker-compatible tar archives, retagged for the
// destination private registry.
package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// BytesFunc reports cumulative bytes written for an image and the best-effort
// total. total may be 0 when it cannot be determined up front.
type BytesFunc func(written, total int64)

// progressThreshold is the minimum number of new bytes between BytesFunc calls,
// so a fast stream does not flood the caller with updates.
const progressThreshold = 512 * 1024

// countingWriter forwards writes while reporting cumulative progress no more
// often than progressThreshold bytes.
type countingWriter struct {
	w        io.Writer
	total    int64
	written  int64
	reported int64
	onBytes  BytesFunc
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.written += int64(n)
	if c.onBytes != nil && (c.written-c.reported >= progressThreshold) {
		c.reported = c.written
		c.onBytes(c.written, c.total)
	}
	return n, err
}

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
//
// It returns the resolved manifest digest of the pulled, platform-specific
// image (e.g. "sha256:..."). The digest pins exactly what was bundled so the
// airgapped side can verify it, even though the tarball itself is tagged rather
// than digest-referenced.
func (p *Puller) Save(ctx context.Context, srcRef, destRef, destPath string, onBytes BytesFunc) (string, error) {
	p.logger.Infof("pulling image %s for platform %s", srcRef, p.platform)

	platform, err := v1.ParsePlatform(p.platform)
	if err != nil {
		return "", fmt.Errorf("parse platform %q: %w", p.platform, err)
	}

	// Build crane options with proxy support
	opts := []crane.Option{crane.WithPlatform(platform)}
	if p.proxy != "" {
		proxyURL, err := url.Parse(p.proxy)
		if err != nil {
			return "", fmt.Errorf("parse proxy URL %q: %w", p.proxy, err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		opts = append(opts, crane.WithTransport(transport))
	}

	// Pull the image. The timeout configuration is not used here because
	// crane.Pull returns a lazy-loading image that retains the context,
	// which would cause the layer write below to time out during downloads.
	img, err := crane.Pull(srcRef, append(opts, crane.WithContext(ctx))...)
	if err != nil {
		return "", fmt.Errorf("pull %s: %w", srcRef, err)
	}

	tag, err := name.NewTag(destRef)
	if err != nil {
		return "", fmt.Errorf("parse dest ref %q: %w", destRef, err)
	}

	p.logger.Debugf("saving %s to %s", destRef, destPath)

	// Create the destination file ourselves so the heavy layer write can stream
	// through a counting writer for byte-level progress.
	file, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("cannot create file in destination directory: %w", err)
	}
	cw := &countingWriter{w: file, total: estimateSize(img, p.logger), onBytes: onBytes}
	if err := tarball.Write(tag, img, cw); err != nil {
		_ = file.Close()
		_ = os.Remove(destPath)
		return "", fmt.Errorf("save %s: %w", destRef, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("close %s: %w", destPath, err)
	}

	// Resolve the digest after pull so it reflects exactly what was written.
	digest := ""
	if d, err := img.Digest(); err != nil {
		p.logger.Debugf("could not resolve digest for %s: %v", srcRef, err)
	} else {
		digest = d.String()
	}

	p.logger.Infof("saved %s (%s)", destPath, digest)
	return digest, nil
}

// estimateSize sums an image's compressed layer sizes and config size as a
// best-effort total for progress reporting. It returns 0 when sizes cannot be
// resolved (e.g. a registry that omits them), in which case progress is
// reported as raw bytes with no percentage.
func estimateSize(img v1.Image, logger *log.Logger) int64 {
	var total int64
	layers, err := img.Layers()
	if err != nil {
		logger.Debugf("could not list layers for size estimate: %v", err)
		return 0
	}
	for _, l := range layers {
		s, err := l.Size()
		if err != nil {
			return 0
		}
		total += s
	}
	if m, err := img.Manifest(); err == nil {
		total += m.Config.Size
	}
	return total
}
