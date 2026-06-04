package helm

import (
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
