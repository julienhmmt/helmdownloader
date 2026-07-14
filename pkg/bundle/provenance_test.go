package bundle

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
