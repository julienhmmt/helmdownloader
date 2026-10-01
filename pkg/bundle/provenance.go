package bundle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
	Tool           string            `json:"tool"`
	ToolVersion    string            `json:"toolVersion,omitempty"`
	CreatedAt      string            `json:"createdAt"`
	Chart          provenanceChart   `json:"chart"`
	Compression    string            `json:"compression"`
	Images         []provenanceImage `json:"images"`
	Status         string            `json:"status"`
	Platform       string            `json:"platform,omitempty"`
	RegistryPrefix string            `json:"registryPrefix,omitempty"`
	MissingImages  []string          `json:"missingImages,omitempty"`
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
		Tool:           tool,
		ToolVersion:    version.Version,
		CreatedAt:      now.UTC().Format(time.RFC3339),
		Chart:          provenanceChart{Name: spec.ChartName, Version: spec.ChartVersion, Archive: chartArchive},
		Compression:    compression,
		Status:         bundleStatus(spec),
		Platform:       spec.Platform,
		RegistryPrefix: spec.RegistryPrefix,
		MissingImages:  spec.MissingImages,
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

func bundleStatus(spec Spec) string {
	if len(spec.MissingImages) > 0 {
		return "partial"
	}
	return "complete"
}

func buildHandoff(spec Spec, archive, chartArchive string) string {
	var text strings.Builder
	fmt.Fprintf(&text, "HelmDownloader airgap handoff\nChart: %s %s\nPlatform: %s\nRegistry prefix: %s\nStatus: %s\nImages: %d included, %d missing\n",
		spec.ChartName, spec.ChartVersion, spec.Platform, spec.RegistryPrefix, strings.ToUpper(bundleStatus(spec)), len(spec.Images), len(spec.MissingImages))
	if len(spec.MissingImages) > 0 {
		text.WriteString("\nWARNING: this bundle is incomplete. Missing selected/requested images:\n")
		for _, ref := range spec.MissingImages {
			fmt.Fprintf(&text, "  %s\n", ref)
		}
	}
	text.WriteString("\nIntegrity verification checks bundled bytes, not image completeness.\nComplete means all selected/requested images were included; discovery is best-effort.\nAn empty registry prefix keeps the original image references.\n")
	text.WriteString(handoffExtractCommands(spec, archive))
	if len(spec.Images) > 0 {
		text.WriteString("\nFrom the extracted directory, authenticate to the target registry, then:\n  DRY_RUN=1 ./load.sh\n  ./load.sh\n  ENGINE=podman ./load.sh\nUse Docker or Podman, not both; load.sh checks sha256sums.txt before load/push.\nSet chart image references to the destinations in images.txt before deploying.\n")
	}
	text.WriteString(handoffInstallCommands(spec, chartArchive))
	return text.String()
}

func handoffInstallCommands(spec Spec, chartArchive string) string {
	var values = ""
	if spec.Values != "" {
		values = " -f values.yaml"
	}
	return fmt.Sprintf("\nInstall the chart with your deployment values:\n  helm install RELEASE %s%s\n", shellQuote("./"+chartArchive), values) +
		"When present, values.yaml contains chart defaults. Custom -values/-set overrides used for discovery are not bundled.\nSupply your deployment values separately; image references are not automatically rewritten.\n"
}

func handoffExtractCommands(spec Spec, archive string) string {
	var directory = safeBundleName(spec.ChartName) + "-" + safeBundleName(spec.ChartVersion) + "-bundle"
	var extract = "tar -xzf"
	if spec.Compression == "zstd" || spec.Compression == "zst" {
		extract = "tar --zstd -xf"
	}
	return fmt.Sprintf("\nBefore extraction, from the archive directory:\n  helmdownloader verify %s\n  mkdir -p %s\n  %s %s -C %s\n  cd %s\nAfter extraction, inspect HOWTO.txt and manifest.json for missing images.\n",
		shellQuote(archive), shellQuote(directory), extract, shellQuote(archive), shellQuote(directory), shellQuote(directory))
}
