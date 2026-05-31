package bundle

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemp creates a file with content under dir and returns its path.
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// readArchive returns a map of archive entry name to its content and a map of
// name to its tar mode.
func readArchive(t *testing.T, path string) (map[string]string, map[string]int64) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	contents := map[string]string{}
	modes := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		contents[hdr.Name] = string(data)
		modes[hdr.Name] = hdr.Mode
	}
	return contents, modes
}

func TestCreate_WritesAllEntries(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "argo-cd-1.0.0.tgz", "chart-bytes")
	img1 := writeTemp(t, work, "img1.tar", "tar1")
	img2 := writeTemp(t, work, "img2.tar", "tar2")

	path, err := Create(Spec{
		ChartName:    "argo-cd",
		ChartVersion: "1.0.0",
		ChartPath:    chart,
		Values:       "replicaCount: 1\n",
		OutputDir:    out,
		Images: []ImageEntry{
			{TarPath: img1, SourceRef: "quay.io/x:1", DestRef: "rgy.local/quay.io/x:1"},
			{TarPath: img2, SourceRef: "redis:7", DestRef: "rgy.local/docker.io/library/redis:7"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(out, "argo-cd-1.0.0-bundle.tar.gz"), path)

	contents, modes := readArchive(t, path)
	assert.Equal(t, "chart-bytes", contents["argo-cd-1.0.0.tgz"])
	assert.Equal(t, "replicaCount: 1\n", contents["values.yaml"])
	assert.Equal(t, "tar1", contents["images/img1.tar"])
	assert.Equal(t, "tar2", contents["images/img2.tar"])
	assert.Contains(t, contents, "images.txt")
	assert.Contains(t, contents, "load.sh")
	assert.Equal(t, int64(0o755), modes["load.sh"], "load.sh must be executable")
}

func TestCreate_LoadScriptListsImages(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "c-1.0.0.tgz", "x")
	img := writeTemp(t, work, "i.tar", "y")

	path, err := Create(Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		ChartPath:    chart,
		OutputDir:    out,
		Images:       []ImageEntry{{TarPath: img, SourceRef: "redis:7", DestRef: "rgy.local/redis:7"}},
	})
	require.NoError(t, err)
	contents, _ := readArchive(t, path)
	script := contents["load.sh"]
	assert.True(t, strings.HasPrefix(script, "#!/bin/sh\n"))
	assert.Contains(t, script, "load_and_push 'images/i.tar' 'rgy.local/redis:7'")
	assert.Contains(t, script, `ENGINE="${ENGINE:-docker}"`)
}

func TestBuildLoadScript_QuotesAndCountsImages(t *testing.T) {
	script := buildLoadScript([]ImageEntry{
		{TarPath: "/work/images/a.tar", DestRef: "rgy.local/a:1"},
		{TarPath: "/work/images/b.tar", DestRef: "rgy.local/b:2"},
	})
	assert.Contains(t, script, "load_and_push 'images/a.tar' 'rgy.local/a:1'")
	assert.Contains(t, script, "load_and_push 'images/b.tar' 'rgy.local/b:2'")
	assert.Contains(t, script, `"$ENGINE" load -i "$DIR/$1"`)
	assert.Contains(t, script, `"$ENGINE" push "$2"`)
	assert.Contains(t, script, "2 image(s)")
}

func TestShellQuote_EscapesSingleQuotes(t *testing.T) {
	assert.Equal(t, `'plain'`, shellQuote("plain"))
	assert.Equal(t, `'a'\''b'`, shellQuote("a'b"))
}
