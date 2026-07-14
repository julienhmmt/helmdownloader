package bundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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
	contents, _ := readArchive(t, path)
	chartEntry := filepath.Base(chart)
	contents[chartEntry] = "TAMPERED"
	tamperedPath := writeGzipTar(t, out, "tampered.tar.gz", contents)
	err = Verify(tamperedPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestVerify_DetectsLoadShTampering(t *testing.T) {
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
	contents, _ := readArchive(t, path)
	contents["load.sh"] = "#!/bin/sh\necho TAMPERED\n"
	tamperedPath := writeGzipTar(t, out, "loadsh-tampered.tar.gz", contents)
	err = Verify(tamperedPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	assert.Contains(t, err.Error(), "load.sh")
}

func TestVerify_RejectsMissingImageDigest(t *testing.T) {
	t.Run("empty digest", func(t *testing.T) {
		work := t.TempDir()
		out := t.TempDir()
		chart := writeTemp(t, work, "c-1.0.0.tgz", "chart")
		img := writeTemp(t, work, "i.tar", "tar")
		path, err := Create(Spec{
			ChartName:    "c",
			ChartVersion: "1.0.0",
			ChartPath:    chart,
			OutputDir:    out,
			Images:       []ImageEntry{{TarPath: img, SourceRef: "x:1", DestRef: "r/x:1", Digest: ""}},
		})
		require.NoError(t, err)
		err = Verify(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing digest")
	})
	t.Run("dash placeholder digest", func(t *testing.T) {
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
		contents, _ := readArchive(t, path)
		// Rewrite manifest with a "-" digest while keeping checksums in sync so
		// the digest contract (not checksums) is what fails.
		manifest := contents["manifest.json"]
		manifest = strings.Replace(manifest, `"digest": "sha256:abc"`, `"digest": "-"`, 1)
		contents["manifest.json"] = manifest
		sum := sha256.Sum256([]byte(manifest))
		hexSum := hex.EncodeToString(sum[:])
		var sumLines []string
		for line := range strings.SplitSeq(strings.TrimRight(contents["sha256sums.txt"], "\n"), "\n") {
			parts := strings.SplitN(line, "  ", 2)
			if len(parts) == 2 && parts[1] == "manifest.json" {
				sumLines = append(sumLines, hexSum+"  manifest.json")
				continue
			}
			sumLines = append(sumLines, line)
		}
		contents["sha256sums.txt"] = strings.Join(sumLines, "\n") + "\n"
		badPath := writeGzipTar(t, out, "dash-digest.tar.gz", contents)
		err = Verify(badPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing digest")
	})
}

// writeGzipTar writes contents as a .tar.gz under dir/name and returns the path.
func writeGzipTar(t *testing.T, dir, name string, contents map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for entryName, data := range contents {
		mode := int64(0o644)
		if entryName == "load.sh" {
			mode = 0o755
		}
		hdr := &tar.Header{Name: entryName, Mode: mode, Size: int64(len(data))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err = tw.Write([]byte(data))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	return path
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

func TestVerify_RejectsOversizedChecksums(t *testing.T) {
	old := maxMetadataFileSize
	maxMetadataFileSize = 64
	t.Cleanup(func() { maxMetadataFileSize = old })
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized-sums.tar.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	blob := strings.Repeat("a", 200)
	hdr := &tar.Header{Name: "sha256sums.txt", Mode: 0o644, Size: int64(len(blob))}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err = tw.Write([]byte(blob))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	err = Verify(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sha256sums.txt")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestVerify_RejectsOversizedManifest(t *testing.T) {
	old := maxMetadataFileSize
	maxMetadataFileSize = 64
	t.Cleanup(func() { maxMetadataFileSize = old })
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized-manifest.tar.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	// Minimal valid sums so Verify gets past the missing-sums check if it ever
	// read the oversized manifest first; entry order is whatever we write.
	sums := "deadbeef  foo.txt\n"
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "sha256sums.txt", Mode: 0o644, Size: int64(len(sums))}))
	_, err = tw.Write([]byte(sums))
	require.NoError(t, err)
	blob := strings.Repeat("m", 200)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(blob))}))
	_, err = tw.Write([]byte(blob))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	err = Verify(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.json")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestDiff_RejectsOversizedManifest(t *testing.T) {
	old := maxMetadataFileSize
	maxMetadataFileSize = 64
	t.Cleanup(func() { maxMetadataFileSize = old })
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized-diff.tar.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	blob := strings.Repeat("d", 200)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(blob))}))
	_, err = tw.Write([]byte(blob))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	ok := buildTestBundle(t, "b", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:aaa"}})
	_, err = Diff(path, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.json")
	assert.Contains(t, err.Error(), "exceeds")
}

// TestVerify_LargeBundleDoesNotLoadImagesIntoMemory guards the streaming
// verify path: a bundle whose image tar is several MiB verifies successfully
// without the reader needing to hold the image bytes in memory.
func TestVerify_LargeBundleDoesNotLoadImagesIntoMemory(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	chart := writeTemp(t, work, "c-1.0.0.tgz", "chart")
	// 8 MiB image tar: large enough that the old readBundle would have held
	// it all in RAM, small enough to keep the test fast.
	blob := make([]byte, 8<<20)
	for i := range blob {
		blob[i] = byte(i)
	}
	img := filepath.Join(work, "big.tar")
	require.NoError(t, os.WriteFile(img, blob, 0o644))
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

// TestDiff_ReadsOnlyManifest confirms the streaming readManifestImages still
// finds manifest.json and reports digest changes.
func TestDiff_ReadsOnlyManifest(t *testing.T) {
	a := buildTestBundle(t, "a", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:aaa"}})
	b := buildTestBundle(t, "b", []ImageEntry{{SourceRef: "x:1", Digest: "sha256:bbb"}})
	result, err := Diff(a, b)
	require.NoError(t, err)
	require.Len(t, result.Changed, 1)
	assert.Equal(t, "x:1", result.Changed[0].Ref)
	assert.Equal(t, "sha256:aaa", result.Changed[0].FromDigest)
	assert.Equal(t, "sha256:bbb", result.Changed[0].ToDigest)
}
