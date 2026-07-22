package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

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
