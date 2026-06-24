package bundle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeBundleName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"argo-cd", "argo-cd"},
		{"../../etc/passwd", "etc_passwd"},
		{"a/b\\c", "a_b_c"},
		{"..", "unknown"}, // falls back to unknown because the name is empty after sanitization
		{"normal-1.0.0", "normal-1.0.0"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := safeBundleName(c.in)
			assert.Equal(t, c.want, got)
			assert.NotContains(t, got, "..", "result must not contain traversal")
			assert.NotContains(t, got, "/", "result must not contain path separator")
		})
	}
}

func TestCreate_SanitizesUnsafeChartName(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "c.tgz", "x")
	img := writeTemp(t, work, "i.tar", "y")

	path, err := Create(Spec{
		ChartName:    "../../etc/passwd",
		ChartVersion: "1.0.0/../x",
		ChartPath:    chart,
		OutputDir:    out,
		Images:       []ImageEntry{{TarPath: img, SourceRef: "x:1", DestRef: "r/x:1"}},
	})
	require.NoError(t, err)

	// The bundle must live inside out, not escape via ../ resolution.
	rel, err := filepath.Rel(out, path)
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(rel, ".."), "bundle escaped OutputDir: %s", rel)
	assert.NotContains(t, filepath.Base(path), "..")
	assert.NotContains(t, filepath.Base(path), "/")
}
