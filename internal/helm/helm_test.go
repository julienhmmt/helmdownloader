package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/julienhmmt/helmdownloader/internal/log"
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
