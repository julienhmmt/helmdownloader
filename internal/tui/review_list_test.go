package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/images"
)

func TestReviewViewport_ClampsVisible(t *testing.T) {
	m := newTestModel()
	m.height = 20
	for i := 0; i < 50; i++ {
		m.reviewImages = append(m.reviewImages, images.Image{Ref: fmt.Sprintf("img-%02d:1", i)})
	}
	start, visible := m.reviewViewport()
	assert.Equal(t, 0, start)
	assert.Equal(t, 8, visible) // 20 - 12 chrome
}

func TestEnsureReviewCursorVisible_ScrollsDown(t *testing.T) {
	m := newTestModel()
	m.height = 20
	for i := 0; i < 50; i++ {
		m.reviewImages = append(m.reviewImages, images.Image{Ref: fmt.Sprintf("img-%02d:1", i)})
	}
	m.reviewCursor = 40
	m.ensureReviewCursorVisible()
	_, visible := m.reviewViewport()
	assert.GreaterOrEqual(t, m.reviewCursor, m.reviewOffset)
	assert.Less(t, m.reviewCursor, m.reviewOffset+visible)
}

func TestEnsureReviewCursorVisible_ScrollsUp(t *testing.T) {
	m := newTestModel()
	m.height = 20
	for i := 0; i < 50; i++ {
		m.reviewImages = append(m.reviewImages, images.Image{Ref: fmt.Sprintf("img-%02d:1", i)})
	}
	m.reviewOffset = 30
	m.reviewCursor = 5
	m.ensureReviewCursorVisible()
	assert.Equal(t, 5, m.reviewOffset)
}

func TestViewReview_WindowsLargeList(t *testing.T) {
	m := newTestModel()
	m.height = 20
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "big"}
	m.selectedVersion = "1.0.0"
	for i := 0; i < 50; i++ {
		m.reviewImages = append(m.reviewImages, images.Image{
			Ref:      fmt.Sprintf("registry.example/app/image-%03d:tag", i),
			Selected: true,
		})
	}
	m.reviewCursor = 40
	m.ensureReviewCursorVisible()
	out := m.render()
	assert.Contains(t, out, "more")
	// First image should be scrolled out of the window.
	assert.NotContains(t, out, "image-000:tag")
	// Cursor row should be visible.
	assert.Contains(t, out, "image-040:tag")
}

func TestHandleReviewKey_PageDownMovesCursor(t *testing.T) {
	m := newTestModel()
	m.height = 20
	m.state = stateReview
	for i := 0; i < 50; i++ {
		m.reviewImages = append(m.reviewImages, images.Image{Ref: fmt.Sprintf("img-%02d:1", i)})
	}
	got, _ := m.handleReviewKey(keyPress("pgdown"))
	m2 := got.(model)
	assert.Greater(t, m2.reviewCursor, 0)
}

func TestHandleReviewKey_BulkSelection(t *testing.T) {
	for _, size := range []int{0, 2, 50} {
		for _, key := range []string{"A", "N"} {
			t.Run(fmt.Sprintf("size=%d/key=%s", size, key), func(t *testing.T) {
				var m = newTestModel()
				defer m.cancel()
				m.state = stateReview
				m.height = 20
				for index := range size {
					m.reviewImages = append(m.reviewImages, images.Image{Ref: fmt.Sprintf("img-%d:1", index), Selected: index%2 == 0})
				}
				m.reviewCursor = max(0, size-1)
				m.ensureReviewCursorVisible()
				var cursor = m.reviewCursor
				var offset = m.reviewOffset
				var got, cmd = m.handleReviewKey(keyPress(key))
				var updated = got.(model)
				assert.Nil(t, cmd)
				assert.Equal(t, stateReview, updated.state)
				assert.Equal(t, cursor, updated.reviewCursor)
				assert.Equal(t, offset, updated.reviewOffset)
				for _, image := range updated.reviewImages {
					assert.Equal(t, key == "A", image.Selected)
				}
				assert.Contains(t, updated.render(), "save review")
				if size > 0 {
					assert.Contains(t, updated.render(), "all")
					assert.Contains(t, updated.render(), "none")
				}
				if size > 0 && key == "N" {
					got, cmd = updated.handleReviewKey(keyPress("enter"))
					assert.Nil(t, cmd)
					assert.Equal(t, stateReview, got.(model).state)
					assert.Contains(t, got.(model).status, "Select at least one image")
				}
			})
		}
	}
}

func TestTruncateMiddle(t *testing.T) {
	assert.Equal(t, "short", truncateMiddle("short", 20))
	assert.Equal(t, "…", truncateMiddle("abcdef", 1))
	got := truncateMiddle("abcdefghijklmnop", 9)
	assert.True(t, strings.Contains(got, "…"))
	require.LessOrEqual(t, len([]rune(got)), 9)
}
