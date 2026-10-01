package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/images"
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
		{Ref: "quay.io/argoproj/argocd:v2", Selected: true},
		{Ref: "redis:7", Selected: false},
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	imgs, err := importImages(path)
	require.NoError(t, err)
	require.Len(t, imgs, 2)
	assert.Equal(t, "quay.io/argoproj/argocd:v2", imgs[0].Ref)
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

func TestImportImages_RejectsInvalidRef(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.json")
	entries := []imageListEntry{
		{Ref: "nginx:1.27", Selected: true},
		{Ref: "not a ref", Selected: true},
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	_, err = importImages(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image ref")
	assert.Contains(t, err.Error(), "entry 1")
}

func TestReviewSave_PathAndCancel(t *testing.T) {
	for _, configured := range []string{"", "approved.json"} {
		t.Run("path="+configured, func(t *testing.T) {
			var m = newTestModel()
			defer m.cancel()
			m.state = stateReview
			m.cfg.ExportImages = configured
			m.reviewImages = []images.Image{{Ref: "redis:7", Selected: false}}
			var got, _ = m.handleReviewKey(keyPress("e"))
			var saving = got.(model)
			assert.Equal(t, stateSaveImages, saving.state)
			var expected = configured
			if expected == "" {
				expected = "reviewed-images.json"
			}
			assert.Equal(t, expected, saving.saveInput.Value())
			assert.Contains(t, saving.render(), "Save reviewed images")
			got, _ = saving.handleKey(keyPress("esc"))
			assert.Equal(t, stateReview, got.(model).state)
			assert.Equal(t, m.reviewImages, got.(model).reviewImages)
		})
	}
}

func TestReviewSave_CurrentSelection(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			var m = newTestModel()
			defer m.cancel()
			m.state = stateReview
			m.reviewImages = []images.Image{{Ref: "redis:7", Selected: true}, {Ref: "nginx:1", Selected: true}}
			var got, _ = m.handleReviewKey(keyPress("space"))
			m = got.(model)
			m.reviewCursor = 1
			got, _ = m.handleReviewKey(keyPress("d"))
			m = got.(model)
			m.reviewImages = append(m.reviewImages, images.Image{Ref: "busybox:1", Selected: true})
			if empty {
				m.reviewImages = nil
			}
			var expected = append([]images.Image{}, m.reviewImages...)
			got, _ = m.handleReviewKey(keyPress("e"))
			m = got.(model)
			var path = filepath.Join(t.TempDir(), "review.json")
			m.saveInput.SetValue(path)
			var cmd tea.Cmd
			got, cmd = m.handleKey(keyPress("enter"))
			require.NotNil(t, cmd)
			m = got.(model)
			assert.Equal(t, stateSavingImages, m.state)
			if !empty {
				m.reviewImages[0].Selected = true
			}
			got, _ = m.Update(cmd())
			m = got.(model)
			assert.Equal(t, stateReview, m.state)
			assert.Contains(t, m.status, path)
			var imported, err = importImages(path)
			require.NoError(t, err)
			assert.Equal(t, expected, imported)
		})
	}
}

func TestReviewSave_OverwriteConfirmation(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprintf("confirm=%t", confirm), func(t *testing.T) {
			var path = filepath.Join(t.TempDir(), "review.json")
			require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
			var m = newTestModel()
			defer m.cancel()
			m.state = stateReview
			m.reviewImages = []images.Image{{Ref: "redis:7", Selected: true}}
			var got, _ = m.handleReviewKey(keyPress("e"))
			m = got.(model)
			m.saveInput.SetValue(path)
			var cmd tea.Cmd
			got, cmd = m.handleKey(keyPress("enter"))
			m = got.(model)
			got, _ = m.Update(cmd())
			m = got.(model)
			assert.Equal(t, stateSaveImages, m.state)
			assert.Contains(t, m.status, "overwrite")
			var before, err = os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "original", string(before))
			if !confirm {
				got, _ = m.handleKey(keyPress("esc"))
				assert.Equal(t, stateReview, got.(model).state)
				return
			}
			got, cmd = m.handleKey(keyPress("enter"))
			m = got.(model)
			got, _ = m.Update(cmd())
			assert.Equal(t, stateReview, got.(model).state)
			var imported []images.Image
			imported, err = importImages(path)
			require.NoError(t, err)
			assert.Equal(t, m.reviewImages, imported)
		})
	}
}

func TestReviewSave_EmptyPathAndStaleResult(t *testing.T) {
	var m = newTestModel()
	defer m.cancel()
	m.state = stateReview
	var got, _ = m.handleReviewKey(keyPress("e"))
	m = got.(model)
	m.saveInput.SetValue("  ")
	var cmd tea.Cmd
	got, cmd = m.handleKey(keyPress("enter"))
	m = got.(model)
	assert.Nil(t, cmd)
	assert.Equal(t, stateSaveImages, m.state)
	assert.Contains(t, m.status, "file path")
	got, _ = m.handleKey(keyPress("esc"))
	m = got.(model)
	got, cmd = m.Update(savedReviewMsg{path: "late.json"})
	assert.Nil(t, cmd)
	assert.Equal(t, stateReview, got.(model).state)
	assert.Empty(t, got.(model).status)
}

func TestReviewSave_ChangedPathNeedsConfirmation(t *testing.T) {
	var first = filepath.Join(t.TempDir(), "first.json")
	var second = filepath.Join(t.TempDir(), "second.json")
	for _, path := range []string{first, second} {
		require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	}
	var m = newTestModel()
	defer m.cancel()
	m.state = stateSaveImages
	m.saveOverwritePath = first
	m.saveInput.SetValue(second)
	var got, cmd = m.handleKey(keyPress("enter"))
	m = got.(model)
	got, _ = m.Update(cmd())
	assert.Equal(t, stateSaveImages, got.(model).state)
	assert.Contains(t, got.(model).status, "overwrite")
	var data, err = os.ReadFile(second)
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
}

func TestReviewSave_ErrorRetry(t *testing.T) {
	var m = newTestModel()
	defer m.cancel()
	m.state = stateReview
	var got, _ = m.handleReviewKey(keyPress("e"))
	m = got.(model)
	var path = filepath.Join(t.TempDir(), "missing", "review.json")
	m.saveInput.SetValue(path)
	var cmd tea.Cmd
	got, cmd = m.handleKey(keyPress("enter"))
	m = got.(model)
	got, _ = m.Update(cmd())
	m = got.(model)
	assert.Equal(t, stateSaveImages, m.state)
	assert.Equal(t, path, m.saveInput.Value())
	assert.Contains(t, m.status, "write image list")
	path = filepath.Join(t.TempDir(), "review.json")
	m.saveInput.SetValue(path)
	got, cmd = m.handleKey(keyPress("enter"))
	m = got.(model)
	got, _ = m.Update(cmd())
	assert.Equal(t, stateReview, got.(model).state)
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
