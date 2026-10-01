package pipeline

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

func TestMissingImageRefs_SelectedOnly(t *testing.T) {
	var requested = []images.Image{{Ref: "a:1", Selected: true}, {Ref: "b:1", Selected: true}, {Ref: "c:1", Selected: false}, {Ref: "b:1", Selected: true}}
	var cases = []struct {
		name    string
		entries []bundle.ImageEntry
		want    []string
	}{
		{"partial", []bundle.ImageEntry{{SourceRef: "a:1"}}, []string{"b:1"}},
		{"complete", []bundle.ImageEntry{{SourceRef: "b:1"}, {SourceRef: "a:1"}}, nil},
		{"all failed", nil, []string{"a:1", "b:1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, missingImageRefs(requested, tc.entries))
		})
	}
}

func TestBundle_RecordsPartialHandoff(t *testing.T) {
	var work = t.TempDir()
	var cfg = config.Default()
	cfg.OutputDir = t.TempDir()
	cfg.Platform = "linux/arm64"
	cfg.RegistryPrefix = "mirror.local/team"
	var chart = filepath.Join(work, "app.tgz")
	require.NoError(t, os.WriteFile(chart, []byte("chart"), 0o600))
	var prepared = Prepared{ChartPath: chart, WorkDir: work, TempWorkDir: true, Images: []images.Image{{Ref: "redis:7", Selected: true}, {Ref: "ignored:1", Selected: false}}}
	var path, err = New(cfg, log.Discard()).Bundle(prepared, artifacthub.Package{Name: "app"}, "1", nil)
	require.NoError(t, err)
	require.NoError(t, bundle.Verify(path))
	var file *os.File
	file, err = os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	var reader *gzip.Reader
	reader, err = gzip.NewReader(file)
	require.NoError(t, err)
	defer reader.Close()
	var archive = tar.NewReader(reader)
	for {
		var header *tar.Header
		header, err = archive.Next()
		require.NoError(t, err)
		if header.Name != "manifest.json" {
			continue
		}
		var manifest struct {
			Status         string   `json:"status"`
			Platform       string   `json:"platform"`
			RegistryPrefix string   `json:"registryPrefix"`
			MissingImages  []string `json:"missingImages"`
		}
		require.NoError(t, json.NewDecoder(archive).Decode(&manifest))
		assert.Equal(t, "partial", manifest.Status)
		assert.Equal(t, cfg.Platform, manifest.Platform)
		assert.Equal(t, cfg.RegistryPrefix, manifest.RegistryPrefix)
		assert.Equal(t, []string{"redis:7"}, manifest.MissingImages)
		break
	}
}

func TestBundle_CleansHelmCacheFromPersistentWorkDir(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()

	// Simulate what isolatedHelmEnv leaves behind.
	require.NoError(t, os.MkdirAll(filepath.Join(work, ".helm", "repository"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, ".helm", "repository", "index.yaml"),
		[]byte("x"), 0o644))

	// Minimal chart archive + image tar so bundle.Create can read them.
	chartPath := filepath.Join(work, "argo-cd-1.0.0.tgz")
	require.NoError(t, os.WriteFile(chartPath, []byte("chart"), 0o644))
	imgPath := filepath.Join(work, "img.tar")
	require.NoError(t, os.WriteFile(imgPath, []byte("tar"), 0o644))

	cfg := config.Default()
	cfg.WorkDir = work
	cfg.OutputDir = out
	pl := New(cfg, log.Discard())

	prepared := Prepared{ChartPath: chartPath, WorkDir: work, TempWorkDir: false}
	pkg := artifacthub.Package{Name: "argo-cd", RepoURL: "https://charts.argoproj.io"}
	entries := []bundle.ImageEntry{{TarPath: imgPath, SourceRef: "x:1", DestRef: "r/x:1"}}

	_, err := pl.Bundle(prepared, pkg, "1.0.0", entries)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(work, ".helm"))
	assert.True(t, os.IsNotExist(err), ".helm cache should be removed from persistent work dir")
	_, err = os.Stat(work)
	assert.NoError(t, err, "persistent work dir itself must be preserved")
}

func TestBundle_ChartOnlyPreservesImagesDir(t *testing.T) {
	// Chart-only bundles must not wipe a persistent images/ cache used by --resume.
	work := t.TempDir()
	out := t.TempDir()
	imagesDir := filepath.Join(work, "images")
	require.NoError(t, os.MkdirAll(imagesDir, 0o755))
	cached := filepath.Join(imagesDir, "cached.tar")
	require.NoError(t, os.WriteFile(cached, []byte("resume-cache"), 0o644))
	chartPath := filepath.Join(work, "crd-1.0.0.tgz")
	require.NoError(t, os.WriteFile(chartPath, []byte("chart"), 0o644))
	cfg := config.Default()
	cfg.WorkDir = work
	cfg.OutputDir = out
	pl := New(cfg, log.Discard())
	prepared := Prepared{ChartPath: chartPath, WorkDir: work, TempWorkDir: false}
	_, err := pl.Bundle(prepared, artifacthub.Package{Name: "crd"}, "1.0.0", nil)
	require.NoError(t, err)
	_, err = os.Stat(cached)
	assert.NoError(t, err, "chart-only bundle must preserve images/ for --resume")
	_, err = os.Stat(chartPath)
	assert.True(t, os.IsNotExist(err), "chart archive is still cleaned up")
}
