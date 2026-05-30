// Package registry pulls container images directly from registries (no Docker
// daemon) and writes them to docker-compatible tar archives, retagged for the
// destination private registry.
package registry

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Puller fetches and saves images for a fixed target platform.
type Puller struct {
	platform string
}

// NewPuller returns a Puller that pulls images for the given platform string,
// e.g. "linux/amd64".
func NewPuller(platform string) *Puller {
	if platform == "" {
		platform = "linux/amd64"
	}
	return &Puller{platform: platform}
}

// Save pulls srcRef for the configured platform and writes it to destPath as a
// docker-style tarball, embedding destRef as the image's tag so a later
// "docker load" yields the retagged image ready to push to the airgap registry.
func (p *Puller) Save(srcRef, destRef, destPath string) error {
	platform, err := v1.ParsePlatform(p.platform)
	if err != nil {
		return fmt.Errorf("parse platform %q: %w", p.platform, err)
	}
	img, err := crane.Pull(srcRef, crane.WithPlatform(platform))
	if err != nil {
		return fmt.Errorf("pull %s: %w", srcRef, err)
	}
	tag, err := name.NewTag(destRef)
	if err != nil {
		return fmt.Errorf("parse dest ref %q: %w", destRef, err)
	}
	if err := crane.Save(img, tag.Name(), destPath); err != nil {
		return fmt.Errorf("save %s: %w", destRef, err)
	}
	return nil
}
