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

func TestTruncateMiddle(t *testing.T) {
	assert.Equal(t, "short", truncateMiddle("short", 20))
	assert.Equal(t, "…", truncateMiddle("abcdef", 1))
	got := truncateMiddle("abcdefghijklmnop", 9)
	assert.True(t, strings.Contains(got, "…"))
	require.LessOrEqual(t, len([]rune(got)), 9)
}
