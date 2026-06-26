package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportImages_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.json")
	imgs := []images.Image{
		{Ref: "quay.io/x:1", Selected: true},
		{Ref: "redis:7", Selected: false},
	}
	require.NoError(t, exportImages(path, imgs))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got []imageListEntry
	require.NoError(t, json.Unmarshal(data, &got))
	require.Len(t, got, 2)
	assert.Equal(t, "quay.io/x:1", got[0].Ref)
	assert.True(t, got[0].Selected)
	assert.Equal(t, "redis:7", got[1].Ref)
	assert.False(t, got[1].Selected)
}

func TestExportImages_EmptyPathNoop(t *testing.T) {
	require.NoError(t, exportImages("", []images.Image{{Ref: "x:1"}}))
}

func TestExportImages_AtomicRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.json")
	require.NoError(t, exportImages(path, []images.Image{{Ref: "x:1", Selected: true}}))
	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "no .tmp file should remain after atomic rename")
}

func TestImportImages_ReadsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.json")
	entries := []imageListEntry{
		{Ref: "quay.io/x:1", Selected: true},
		{Ref: "redis:7", Selected: false},
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	imgs, err := importImages(path)
	require.NoError(t, err)
	require.Len(t, imgs, 2)
	assert.Equal(t, "quay.io/x:1", imgs[0].Ref)
	assert.True(t, imgs[0].Selected)
	assert.Equal(t, "redis:7", imgs[1].Ref)
	assert.False(t, imgs[1].Selected)
}

func TestImportImages_EmptyPathReturnsNil(t *testing.T) {
	imgs, err := importImages("")
	require.NoError(t, err)
	assert.Nil(t, imgs)
}

func TestImportImages_MissingFileErrors(t *testing.T) {
	_, err := importImages(filepath.Join(t.TempDir(), "nonexistent.json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read image list")
}

func TestImportImages_MalformedJSONErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))
	_, err := importImages(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse image list")
}

func TestExportImport_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.json")
	original := []images.Image{
		{Ref: "quay.io/argoproj/argocd:v3.2.6", Selected: true},
		{Ref: "redis:7", Selected: false},
		{Ref: "nginx:1.25", Selected: true},
	}
	require.NoError(t, exportImages(path, original))
	imported, err := importImages(path)
	require.NoError(t, err)
	require.Len(t, imported, len(original))
	for i, img := range original {
		assert.Equal(t, img.Ref, imported[i].Ref, "ref mismatch at %d", i)
		assert.Equal(t, img.Selected, imported[i].Selected, "selected mismatch at %d", i)
	}
}
