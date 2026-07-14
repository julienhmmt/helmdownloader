package bundle

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/julienhmmt/helmdownloader/pkg/version"
)

// tool is the producer name recorded in the provenance manifest.
const tool = "helmdownloader"

// provenance is a lightweight, machine-readable record of what a bundle
// contains: the chart, the codec, and every image with its pinned digest. It is
// a provenance stub — not a full SPDX/CycloneDX SBOM — but it is enough to audit
// or diff a bundle on the airgapped side.
type provenance struct {
	Tool        string            `json:"tool"`
	ToolVersion string            `json:"toolVersion,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	Chart       provenanceChart   `json:"chart"`
	Compression string            `json:"compression"`
	Images      []provenanceImage `json:"images"`
}

type provenanceChart struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Archive string `json:"archive"`
}

type provenanceImage struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
	Digest string `json:"digest,omitempty"`
	Tar    string `json:"tar"`
}

// buildProvenance renders the manifest.json contents for spec. now supplies the
// timestamp so callers (and tests) can control it.
func buildProvenance(spec Spec, chartArchive, compression string, now time.Time) ([]byte, error) {
	p := provenance{
		Tool:        tool,
		ToolVersion: version.Version,
		CreatedAt:   now.UTC().Format(time.RFC3339),
		Chart:       provenanceChart{Name: spec.ChartName, Version: spec.ChartVersion, Archive: chartArchive},
		Compression: compression,
	}
	for _, img := range spec.Images {
		p.Images = append(p.Images, provenanceImage{
			Source: img.SourceRef,
			Dest:   img.DestRef,
			Digest: img.Digest,
			Tar:    "images/" + filepath.Base(img.TarPath),
		})
	}
	return json.MarshalIndent(p, "", "  ")
}
