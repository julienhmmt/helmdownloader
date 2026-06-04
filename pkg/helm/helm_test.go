package helm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsBinary(t *testing.T) {
	assert.Equal(t, "helm", New("", "", log.Discard()).bin)
	assert.Equal(t, "/opt/helm", New("/opt/helm", "", log.Discard()).bin)
}

func TestFindChart_PrefersConventionalName(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "argo-cd-5.0.0.tgz")
	require.NoError(t, os.WriteFile(want, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.tgz"), []byte("y"), 0o644))

	got, err := findChart(dir, "argo-cd", "5.0.0")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestFindChart_FallsBackToAnyTgz(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "renamed-chart.tgz")
	require.NoError(t, os.WriteFile(other, []byte("y"), 0o644))

	got, err := findChart(dir, "argo-cd", "5.0.0")
	require.NoError(t, err)
	assert.Equal(t, other, got)
}

func TestFindChart_NoArchiveErrors(t *testing.T) {
	_, err := findChart(t.TempDir(), "argo-cd", "5.0.0")
	assert.ErrorContains(t, err, "no chart archive")
}

func TestTemplateOptions_AppendArgs(t *testing.T) {
	args := []string{"template", "release", "chart.tgz"}
	WithValuesFile("custom.yaml")(&args)
	WithSetValue("monitoring.enabled=true")(&args)
	assert.Equal(t, []string{
		"template", "release", "chart.tgz",
		"--values", "custom.yaml",
		"--set", "monitoring.enabled=true",
	}, args)
}

func TestCheck_MissingBinary(t *testing.T) {
	err := New("helm-binary-that-does-not-exist-xyz", "", log.Discard()).Check(context.Background())
	assert.ErrorContains(t, err, "not found")
}

func TestCheck_PresentButFails(t *testing.T) {
	// "false" exists on PATH but exits non-zero, standing in for a broken helm.
	if _, err := os.Stat("/usr/bin/false"); err != nil {
		t.Skip("/usr/bin/false unavailable")
	}
	err := New("false", "", log.Discard()).Check(context.Background())
	assert.ErrorContains(t, err, "failed to run")
}

func TestCheck_PresentAndRunnable(t *testing.T) {
	// "true" exists and exits zero, standing in for a working helm.
	err := New("true", "", log.Discard()).Check(context.Background())
	assert.NoError(t, err)
}

// writeChartArchive builds a minimal .tgz with the given "tar-path -> content"
// entries, mirroring how helm packages a chart with vendored subcharts.
func writeChartArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chart-1.0.0.tgz")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return path
}

func TestSubchartValues_ReturnsOnlySubchartFiles(t *testing.T) {
	path := writeChartArchive(t, map[string]string{
		"mychart/values.yaml":                     "parent: true\n",
		"mychart/Chart.yaml":                      "name: mychart\n",
		"mychart/charts/redis/values.yaml":        "redisImage: redis:7\n",
		"mychart/charts/redis/templates/dep.yaml": "kind: Deployment\n",
		"mychart/charts/sub/charts/x/values.yaml": "deepImage: nginx:1.27\n",
	})
	got, err := New("helm", "", log.Discard()).SubchartValues(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	joined := got[0] + "\n" + got[1]
	assert.Contains(t, joined, "redisImage: redis:7")
	assert.Contains(t, joined, "deepImage: nginx:1.27")
	assert.NotContains(t, joined, "parent: true")
}

func TestSubchartValues_MissingArchiveErrors(t *testing.T) {
	_, err := New("helm", "", log.Discard()).SubchartValues("/no/such/chart.tgz")
	assert.ErrorContains(t, err, "open chart archive")
}
