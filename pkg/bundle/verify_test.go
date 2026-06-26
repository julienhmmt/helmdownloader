package bundle

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_IntactBundle(t *testing.T) {
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
	assert.NoError(t, Verify(path))
}

func TestVerify_MissingChecksumsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.tar.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "foo.txt", Mode: 0o644, Size: 3}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write([]byte("bar"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	err = Verify(path)
	assert.ErrorContains(t, err, "missing sha256sums.txt")
}

func TestVerify_TamperedContent(t *testing.T) {
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
	// Read the bundle, tamper with the chart archive (always checksummed),
	// re-tar with the original (now stale) sha256sums.txt.
	entries, err := readBundle(path)
	require.NoError(t, err)
	chartEntry := filepath.Base(chart)
	entries[chartEntry] = []byte("TAMPERED")
	tamperedPath := filepath.Join(out, "tampered.tar.gz")
	f, err := os.Create(tamperedPath)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, data := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err = tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	err = Verify(tamperedPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestVerify_UnknownExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.zip")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	err := Verify(path)
	assert.ErrorContains(t, err, "unknown bundle extension")
}

func TestDiff_NoDifferences(t *testing.T) {
	a := buildTestBundle(t, "a", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:aaa"}})
	b := buildTestBundle(t, "b", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:aaa"}})
	result, err := Diff(a, b)
	require.NoError(t, err)
	assert.Empty(t, result.Added)
	assert.Empty(t, result.Removed)
	assert.Empty(t, result.Changed)
}

func TestDiff_AddedRemovedChanged(t *testing.T) {
	a := buildTestBundle(t, "a", []ImageEntry{
		{SourceRef: "x:1", Digest: "sha256:aaa"},
		{SourceRef: "y:2", Digest: "sha256:bbb"},
	})
	b := buildTestBundle(t, "b", []ImageEntry{
		{SourceRef: "x:1", Digest: "sha256:ccc"},
		{SourceRef: "z:3", Digest: "sha256:ddd"},
	})
	result, err := Diff(a, b)
	require.NoError(t, err)
	assert.Equal(t, []string{"z:3"}, result.Added)
	assert.Equal(t, []string{"y:2"}, result.Removed)
	require.Len(t, result.Changed, 1)
	assert.Equal(t, "x:1", result.Changed[0].Ref)
	assert.Equal(t, "sha256:aaa", result.Changed[0].FromDigest)
	assert.Equal(t, "sha256:ccc", result.Changed[0].ToDigest)
}

func TestDiff_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.tar.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "foo.txt", Mode: 0o644, Size: 3}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write([]byte("bar"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	b := buildTestBundle(t, "b", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:aaa"}})
	_, err = Diff(path, b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing manifest.json")
}

// buildTestBundle creates a minimal bundle with the given image entries and
// returns its path. The chart and image tar files are dummies on disk.
func buildTestBundle(t *testing.T, name string, imgs []ImageEntry) string {
	t.Helper()
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, name+".tgz", "chart")
	entries := make([]ImageEntry, len(imgs))
	for i, img := range imgs {
		img.TarPath = writeTemp(t, work, "img.tar", "tar")
		img.DestRef = "r/" + img.SourceRef
		entries[i] = img
	}
	path, err := Create(Spec{
		ChartName:    name,
		ChartVersion: "1.0.0",
		ChartPath:    chart,
		OutputDir:    out,
		Images:       entries,
	})
	require.NoError(t, err)
	return path
}
