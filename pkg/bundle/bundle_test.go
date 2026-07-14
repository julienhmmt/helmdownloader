package bundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
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
	defer func() { assert.NoError(t, f.Close()) }()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	contents := map[string]string{}
	modes := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
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
			{TarPath: img1, SourceRef: "quay.io/x:1", DestRef: "rgy.local/quay.io/x:1", Digest: "sha256:aaa"},
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

	// images.txt records source, dest, tar name, and digest (or "-" when absent).
	manifest := contents["images.txt"]
	assert.Contains(t, manifest, "quay.io/x:1\trgy.local/quay.io/x:1\timages/img1.tar\tsha256:aaa")
	assert.Contains(t, manifest, "images/img2.tar\t-")
	// A known digest is emitted as a comment above its load_and_push line.
	assert.Contains(t, contents["load.sh"], "# sha256:aaa")

	// sha256sums.txt covers every payload file including load.sh (not itself).
	sums := contents["sha256sums.txt"]
	require.Contains(t, contents, "sha256sums.txt")
	assert.Contains(t, sums, "  argo-cd-1.0.0.tgz")
	assert.Contains(t, sums, "  values.yaml")
	assert.Contains(t, sums, "  images/img1.tar")
	assert.Contains(t, sums, "  images.txt")
	assert.Contains(t, sums, "  load.sh")
	// Checksums must match the actual bundled bytes.
	for line := range strings.SplitSeq(strings.TrimSpace(sums), "\n") {
		parts := strings.SplitN(line, "  ", 2)
		require.Len(t, parts, 2, "malformed sum line %q", line)
		sum := sha256.Sum256([]byte(contents[parts[1]]))
		assert.Equal(t, hex.EncodeToString(sum[:]), parts[0], "checksum mismatch for %s", parts[1])
	}
	// load.sh verifies before pushing and fails closed without a checksum tool.
	assert.Contains(t, contents["load.sh"], "sha256sums.txt")
	assert.Contains(t, contents["load.sh"], "refuse to load without integrity check")

	// manifest.json provenance is present and references the images.
	require.Contains(t, contents, "manifest.json")
	assert.Contains(t, contents["manifest.json"], `"tool": "helmdownloader"`)
	assert.Contains(t, contents["manifest.json"], "sha256:aaa")
	assert.Contains(t, sums, "  manifest.json")
}

func TestCreate_ZstdProducesZstExtension(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "c-1.0.0.tgz", "chart")
	img := writeTemp(t, work, "i.tar", "tar")

	path, err := Create(Spec{
		ChartName:    "c",
		ChartVersion: "1.0.0",
		ChartPath:    chart,
		OutputDir:    out,
		Compression:  "zstd",
		Images:       []ImageEntry{{TarPath: img, SourceRef: "redis:7", DestRef: "rgy.local/redis:7"}},
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(out, "c-1.0.0-bundle.tar.zst"), path)

	// The archive must be readable back through the zstd decoder.
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { assert.NoError(t, f.Close()) }()
	zr, err := zstd.NewReader(f)
	require.NoError(t, err)
	defer zr.Close()
	tr := tar.NewReader(zr)
	var names []string
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
	}
	assert.Contains(t, names, "c-1.0.0.tgz")
	assert.Contains(t, names, "load.sh")
}

func TestCreate_RejectsUnknownCompression(t *testing.T) {
	_, err := Create(Spec{
		ChartName: "c", ChartVersion: "1.0.0",
		ChartPath: writeTemp(t, t.TempDir(), "c.tgz", "x"),
		OutputDir: t.TempDir(), Compression: "lzma",
		Images: []ImageEntry{{TarPath: writeTemp(t, t.TempDir(), "i.tar", "y"), DestRef: "r/x:1"}},
	})
	assert.ErrorContains(t, err, "unknown compression")
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
	// DRY_RUN preview support and idempotent skip-if-present.
	assert.Contains(t, script, `DRY_RUN="${DRY_RUN:-}"`)
	assert.Contains(t, script, `echo "DRY_RUN: $*"`)
	assert.Contains(t, script, `"$ENGINE" image inspect "$2"`)
	assert.Contains(t, script, "already present, skipping load")
	// Fail closed when no checksum tool is available.
	assert.Contains(t, script, "refuse to load without integrity check")
	assert.Contains(t, script, "exit 1")
	assert.NotContains(t, script, "skipping checksum verification")
}

func TestCreate_ChecksumsIncludeLoadSh(t *testing.T) {
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
	contents, modes := readArchive(t, path)
	require.Contains(t, contents, "load.sh")
	require.Contains(t, contents, "sha256sums.txt")
	assert.Equal(t, int64(0o755), modes["load.sh"])
	sums := contents["sha256sums.txt"]
	assert.Contains(t, sums, "  load.sh")
	loadSum := sha256.Sum256([]byte(contents["load.sh"]))
	wantLine := hex.EncodeToString(loadSum[:]) + "  load.sh"
	assert.Contains(t, sums, wantLine)
}

func TestBuildLoadScript_IsValidShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	script := buildLoadScript([]ImageEntry{
		{TarPath: "/work/images/a.tar", DestRef: "rgy.local/a:1", Digest: "sha256:abc"},
		{TarPath: "/work/images/b.tar", DestRef: "rgy.local/b:2"},
	})
	path := writeTemp(t, t.TempDir(), "load.sh", script)
	out, err := exec.Command("sh", "-n", path).CombinedOutput()
	require.NoError(t, err, "sh -n failed: %s", out)
}

func TestShellQuote_EscapesSingleQuotes(t *testing.T) {
	assert.Equal(t, `'plain'`, shellQuote("plain"))
	assert.Equal(t, `'a'\''b'`, shellQuote("a'b"))
}
