package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/images"
)

func TestViewResults_EmptySearchStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateResults
	m.allPackages = nil
	m.setStatus("No charts found. Try a different query.")
	out := m.render()
	assert.Contains(t, out, "No charts")
}

func TestViewReview_ShowsStatusLine(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "argo-cd"}
	m.selectedVersion = "1.0.0"
	m.setStatus("Select at least one image (space), or press a to add one.")
	out := m.render()
	assert.Contains(t, out, "Select at least one image")
}

func TestUpdateReview_EnterZeroSelectedSetsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.reviewImages = []images.Image{
		{Ref: "nginx:1.27", Selected: false},
	}
	got, _ := m.handleReviewKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.Contains(t, m2.status, "Select at least one image")
}

func TestSearchResultMsg_EmptyPackagesSetsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateSearching
	got, _ := m.Update(searchResultMsg{packages: nil})
	m2 := got.(model)
	assert.Equal(t, stateResults, m2.state)
	assert.Contains(t, m2.status, "No charts found")
}

func TestSearchResultMsg_NonEmptyClearsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateSearching
	m.setStatus("stale")
	got, _ := m.Update(searchResultMsg{packages: []artifacthub.Package{{Name: "argo-cd"}}})
	m2 := got.(model)
	assert.Equal(t, stateResults, m2.state)
	assert.Empty(t, m2.status)
}

func TestVersionsMsg_EmptySetsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateSearching
	got, _ := m.Update(versionsMsg{versions: nil})
	m2 := got.(model)
	assert.Equal(t, stateVersions, m2.state)
	assert.Contains(t, m2.status, "No versions")
}

func TestHandleAddImageKey_InvalidRefSetsStatus(t *testing.T) {
	tests := []struct {
		name string
		ref  string
	}{
		{name: "spaces", ref: "not a ref"},
		{name: "unparseable", ref: "!!!invalid:ref"},
		{name: "template", ref: "image:{{ .Values.tag }}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.state = stateAddImage
			m.addInput.SetValue(tt.ref)
			got, _ := m.handleAddImageKey(keyPress("enter"))
			m2 := got.(model)
			assert.Equal(t, stateAddImage, m2.state)
			assert.Contains(t, m2.status, "Invalid image reference")
			assert.Empty(t, m2.reviewImages)
		})
	}
}

func TestHandleAddImageKey_ValidRefAppendsAndClearsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateAddImage
	m.addInput.SetValue("nginx:1.27")
	got, _ := m.handleAddImageKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.Empty(t, m2.status)
	require.Len(t, m2.reviewImages, 1)
	assert.Equal(t, "nginx:1.27", m2.reviewImages[0].Ref)
	assert.True(t, m2.reviewImages[0].Selected)
}
