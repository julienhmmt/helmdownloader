package bundle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildHandoff_StatusAndCommands(t *testing.T) {
	var cases = []struct {
		name     string
		codec    string
		included int
		missing  []string
		status   string
	}{
		{"complete gzip", "gzip", 1, nil, "COMPLETE"},
		{"partial zstd", "zstd", 1, []string{"redis:7", "busybox:1"}, "PARTIAL"},
		{"chart only", "gzip", 0, nil, "COMPLETE"},
		{"all failed", "gzip", 0, []string{"redis:7"}, "PARTIAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var spec = Spec{ChartName: "app", ChartVersion: "1.0", Platform: "linux/arm64", RegistryPrefix: "mirror.local/team", Compression: tc.codec, Images: make([]ImageEntry, tc.included), MissingImages: tc.missing}
			var text = buildHandoff(spec, "app-bundle.tar.gz", "it's chart.tgz")
			assert.Contains(t, text, tc.status)
			assert.Contains(t, text, "linux/arm64")
			assert.Contains(t, text, "mirror.local/team")
			assert.Contains(t, text, "helmdownloader verify 'app-bundle.tar.gz'")
			assert.Contains(t, text, "discovery")
			assert.Contains(t, text, "not bundled")
			assert.Contains(t, text, "'./it'\\''s chart.tgz'")
			if tc.codec == "zstd" {
				assert.Contains(t, text, "tar --zstd -xf")
			} else {
				assert.Contains(t, text, "tar -xzf")
			}
			for _, ref := range tc.missing {
				assert.Contains(t, text, ref)
			}
			if tc.included > 0 {
				assert.Contains(t, text, "ENGINE=podman ./load.sh")
				assert.Contains(t, text, "DRY_RUN=1 ./load.sh")
			} else {
				assert.NotContains(t, text, "./load.sh")
			}
			var data, err = buildProvenance(spec, "chart.tgz", "gz", time.Now())
			require.NoError(t, err)
			var manifest provenance
			require.NoError(t, json.Unmarshal(data, &manifest))
			assert.Equal(t, strings.ToLower(tc.status), manifest.Status)
			assert.Equal(t, tc.missing, manifest.MissingImages)
			assert.Equal(t, spec.Platform, manifest.Platform)
			assert.Equal(t, spec.RegistryPrefix, manifest.RegistryPrefix)
		})
	}
}

func TestBuildHandoff_OptionalValues(t *testing.T) {
	for _, values := range []string{"", "replicas: 1"} {
		t.Run("values="+values, func(t *testing.T) {
			var text = buildHandoff(Spec{Values: values}, "bundle.tar.gz", "chart.tgz")
			assert.Equal(t, values != "", strings.Contains(text, " -f values.yaml"))
			assert.Contains(t, text, "empty registry prefix keeps the original")
		})
	}
}

func TestBuildProvenance(t *testing.T) {
	spec := Spec{
		ChartName:    "argo-cd",
		ChartVersion: "1.0.0",
		Images: []ImageEntry{
			{TarPath: "/work/images/a.tar", SourceRef: "quay.io/x:1", DestRef: "rgy.local/quay.io/x:1", Digest: "sha256:aaa"},
			{TarPath: "/work/images/b.tar", SourceRef: "redis:7", DestRef: "rgy.local/redis:7"},
		},
	}
	when := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	data, err := buildProvenance(spec, "argo-cd-1.0.0.tgz", "zst", when)
	require.NoError(t, err)

	var got provenance
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "helmdownloader", got.Tool)
	assert.NotEmpty(t, got.ToolVersion)
	assert.Equal(t, "2026-06-04T12:00:00Z", got.CreatedAt)
	assert.Equal(t, "argo-cd", got.Chart.Name)
	assert.Equal(t, "1.0.0", got.Chart.Version)
	assert.Equal(t, "argo-cd-1.0.0.tgz", got.Chart.Archive)
	assert.Equal(t, "zst", got.Compression)
	require.Len(t, got.Images, 2)
	assert.Equal(t, "quay.io/x:1", got.Images[0].Source)
	assert.Equal(t, "sha256:aaa", got.Images[0].Digest)
	assert.Equal(t, "images/a.tar", got.Images[0].Tar)
	// A missing digest is omitted from the JSON.
	assert.Empty(t, got.Images[1].Digest)
}
