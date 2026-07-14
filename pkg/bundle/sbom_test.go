package bundle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedTime is a deterministic timestamp for SBOM tests.
var fixedTime = time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

func TestBuildSBOM_HasRequiredFields(t *testing.T) {
	spec := Spec{
		ChartName:    "argo-cd",
		ChartVersion: "1.0.0",
		ChartPath:    "/work/argo-cd-1.0.0.tgz",
		Images: []ImageEntry{
			{SourceRef: "quay.io/x:1", DestRef: "rgy.local/x:1", Digest: "sha256:aaa"},
			{SourceRef: "redis:7", DestRef: "rgy.local/redis:7"},
		},
	}
	out, err := buildSBOM(spec, "argo-cd-1.0.0.tgz", fixedTime)
	require.NoError(t, err)
	var doc spdxDocument
	require.NoError(t, json.Unmarshal(out, &doc))
	assert.Equal(t, "SPDX-2.3", doc.SPDXVersion)
	assert.Equal(t, "CC0-1.0", doc.DataLicense)
	assert.Equal(t, "SPDXRef-DOCUMENT", doc.SPDXID)
	require.NotEmpty(t, doc.CreationInfo.Created)
	require.NotEmpty(t, doc.CreationInfo.Creators)
	// Chart package + one per image.
	require.Len(t, doc.Packages, 3)
	chartPkg := doc.Packages[0]
	assert.Equal(t, "SPDXRef-Package-Chart", chartPkg.SPDXID)
	assert.Equal(t, "argo-cd", chartPkg.Name)
	assert.Equal(t, "1.0.0", chartPkg.VersionInfo)
	assert.Equal(t, "argo-cd-1.0.0.tgz", chartPkg.DownloadLocation)
	assert.False(t, strings.HasPrefix(chartPkg.DownloadLocation, "/"))
	assert.Contains(t, doc.CreationInfo.Creators[0], "helmdownloader")
	// Each image package has a DEPENDS_ON relationship from the chart.
	assert.Len(t, doc.Relationships, 2)
	for _, rel := range doc.Relationships {
		assert.Equal(t, "SPDXRef-Package-Chart", rel.SPDXElementID)
		assert.Equal(t, "DEPENDS_ON", rel.RelationshipType)
		assert.True(t, strings.HasPrefix(rel.RelatedSPDXElement, "SPDXRef-Package-Image-"))
	}
}

func TestBuildSBOM_ImageDigestAsChecksum(t *testing.T) {
	spec := Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		Images:       []ImageEntry{{SourceRef: "x:1", Digest: "sha256:abc"}},
	}
	out, err := buildSBOM(spec, "c-1.0.0.tgz", fixedTime)
	require.NoError(t, err)
	var doc spdxDocument
	require.NoError(t, json.Unmarshal(out, &doc))
	// Package 0 is the chart; package 1 is the image.
	require.Len(t, doc.Packages, 2)
	imgPkg := doc.Packages[1]
	require.Len(t, imgPkg.Checksums, 1)
	assert.Equal(t, "SHA256", imgPkg.Checksums[0].Algorithm)
	assert.Equal(t, "abc", imgPkg.Checksums[0].ChecksumValue)
}

func TestBuildSBOM_NoDigestOmitsChecksum(t *testing.T) {
	spec := Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		Images:       []ImageEntry{{SourceRef: "x:1", Digest: ""}},
	}
	out, err := buildSBOM(spec, "c-1.0.0.tgz", fixedTime)
	require.NoError(t, err)
	var doc spdxDocument
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Packages, 2)
	assert.Empty(t, doc.Packages[1].Checksums, "image with no digest must omit checksums")
}

func TestBuildSBOM_DigestDashSentinelOmitsChecksum(t *testing.T) {
	spec := Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		Images:       []ImageEntry{{SourceRef: "x:1", Digest: "-"}},
	}
	out, err := buildSBOM(spec, "c-1.0.0.tgz", fixedTime)
	require.NoError(t, err)
	var doc spdxDocument
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Packages, 2)
	assert.Empty(t, doc.Packages[1].Checksums, "image with '-' digest sentinel must omit checksums")
}

func TestBuildSBOM_ValidJSON(t *testing.T) {
	spec := Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		Images:       []ImageEntry{{SourceRef: "x:1", Digest: "sha256:abc"}},
	}
	out, err := buildSBOM(spec, "c-1.0.0.tgz", fixedTime)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(out, &raw), "buildSBOM output must be valid JSON")
	assert.Equal(t, "SPDX-2.3", raw["spdxVersion"])
}

func TestCreate_IncludesSBOM(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "c-1.0.0.tgz", "chart")
	img := writeTemp(t, work, "i.tar", "tar")
	path, err := Create(Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		ChartPath:    chart,
		OutputDir:    out,
		Images:       []ImageEntry{{TarPath: img, SourceRef: "x:1", DestRef: "r/x:1", Digest: "sha256:abc"}},
	})
	require.NoError(t, err)
	contents, _ := readArchive(t, path)
	sbom, ok := contents["sbom.spdx.json"]
	require.True(t, ok, "bundle must contain sbom.spdx.json")
	var doc spdxDocument
	require.NoError(t, json.Unmarshal([]byte(sbom), &doc), "sbom.spdx.json must be valid JSON")
	assert.Equal(t, "SPDX-2.3", doc.SPDXVersion)
	// sha256sums.txt must cover the SBOM.
	assert.Contains(t, contents["sha256sums.txt"], "  sbom.spdx.json")
}
