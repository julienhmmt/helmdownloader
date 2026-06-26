package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// newTestModel returns a model sized to a known terminal so framed views
// render deterministically.
func newTestModel() model {
	m := newModel(config.Default(), log.Discard())
	m.width, m.height = 100, 40
	return m
}

func TestViewRendersEveryScreen(t *testing.T) {
	base := newTestModel()
	base.selectedPkg = artifacthub.Package{Name: "argo-cd"}
	base.selectedVersion = "5.51.6"
	base.reviewImages = []images.Image{
		{Ref: "quay.io/argoproj/argocd:v2.9.3", Selected: true},
		{Ref: "ghcr.io/dexidp/dex:v2.37.0", Selected: false},
	}
	base.entries = nil
	base.failures = []pipeline.ImageFailure{{Ref: "redis:7.0", Err: errors.New("not found")}}
	base.bundlePath = "archives/argo-cd-5.51.6.tar.gz"
	base.err = errors.New("boom")

	states := []state{
		stateSearch, stateSearching, statePreparing, stateResults, stateFilterInput,
		stateVersions, stateReview, stateAddImage, stateDownloading,
		stateDownloadReview, stateBundling, stateDone, stateError,
	}
	for _, s := range states {
		m := base
		m.state = s
		out := m.render()
		assert.NotEmpty(t, strings.TrimSpace(out), "state %d rendered empty", s)
	}
}

func TestViewReviewShowsSelectionAndCursor(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "argo-cd"}
	m.selectedVersion = "5.51.6"
	m.reviewImages = []images.Image{
		{Ref: "quay.io/argoproj/argocd:v2.9.3", Selected: true},
	}

	out := m.render()
	assert.Contains(t, out, "argo-cd")
	assert.Contains(t, out, "1 selected of 1")
	assert.Contains(t, out, "quay.io/argoproj/argocd:v2.9.3")
}

func TestViewResultsShowsSortFilterStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateResults
	m.allPackages = []artifacthub.Package{
		{Name: "argo-cd", Stars: 200, Author: "jdoe", Organization: "argoproj"},
		{Name: "redis", Stars: 50, Author: "bitnami", Organization: "bitnami"},
	}
	m.sortField = sortStars
	m.sortDir = sortDesc
	m.filterField = filterAuthor
	m.filterValue = "bit"
	m.results.SetItems(packagesToItems(m.visiblePackages()))

	out := m.render()
	assert.Contains(t, out, "sort: stars↓")
	assert.Contains(t, out, "filter: author=\"bit\"")
	// The count number is accent-styled, so it's split by ANSI codes from
	// "shown" — check both parts independently.
	assert.Contains(t, out, "shown")
	assert.Contains(t, out, "sort:")
}

func TestViewResultsShowsFilterOff(t *testing.T) {
	m := newTestModel()
	m.state = stateResults
	m.allPackages = []artifacthub.Package{{Name: "argo-cd", Stars: 200}}
	m.sortField = sortName
	m.sortDir = sortAsc
	m.filterField = filterNone
	m.results.SetItems(packagesToItems(m.visiblePackages()))

	out := m.render()
	assert.Contains(t, out, "sort: name↑")
	assert.Contains(t, out, "filter: off")
}

func TestViewFilterInputShowsPrompt(t *testing.T) {
	m := newTestModel()
	m.state = stateFilterInput
	m.filterField = filterCompany
	out := m.render()
	assert.Contains(t, out, "Filter by company")
	// "tab" and "cycle" are styled separately by renderHelp, so check
	// each token independently rather than as a contiguous substring.
	assert.Contains(t, out, "tab")
	assert.Contains(t, out, "cycle")
}

func TestRenderHelp_HighlightsKeys(t *testing.T) {
	m := newTestModel()
	out := m.renderHelp("enter select · q quit")
	// Keys are rendered in the accent (selected) style, labels in secondary.
	// Both tokens must be present; the "·" separator must also be present.
	assert.Contains(t, out, "enter")
	assert.Contains(t, out, "select")
	assert.Contains(t, out, "q")
	assert.Contains(t, out, "quit")
	assert.Contains(t, out, "·")
}

func TestRenderHelp_SingleToken(t *testing.T) {
	m := newTestModel()
	out := m.renderHelp("q")
	assert.Contains(t, out, "q")
}

func TestFrameCentersWhenSized(t *testing.T) {
	m := newTestModel()
	out := m.frame("hello")
	// A rounded border draws corner glyphs around the content.
	assert.Contains(t, out, "╭")
	assert.Contains(t, out, "╰")
	assert.Contains(t, out, "hello")
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{5 * 1024 * 1024 * 1024, "5.0 GiB"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, humanBytes(c.in), "humanBytes(%d)", c.in)
	}
}

// stripANSI removes SGR escape sequences so a styled bar can be compared by
// its visible glyphs. The bar fills with "━" and leaves a "─" track.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestMiniBar_Determinate(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	bar := stripANSI(m.miniBar(5, 10, 10))
	assert.Equal(t, "[━━━━━─────]", bar)
}

func TestMiniBar_Full(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	bar := stripANSI(m.miniBar(10, 10, 10))
	assert.Equal(t, "[━━━━━━━━━━]", bar)
}

func TestMiniBar_OverFillClamps(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	bar := stripANSI(m.miniBar(20, 10, 10))
	assert.Equal(t, "[━━━━━━━━━━]", bar)
}

func TestMiniBar_Indeterminate(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	// 1 cell per MiB; 5 MiB written -> 5 cells of a 10-cell bar.
	bar := stripANSI(m.miniBar(5*1024*1024, 0, 10))
	assert.Equal(t, "[━━━━━─────]", bar)
}

func TestByteLabel_WithTotal(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	label := m.byteLabel(1024, 2048)
	assert.Contains(t, label, "1.0 KiB")
	assert.Contains(t, label, "2.0 KiB")
}

func TestByteLabel_WithoutTotal(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	label := m.byteLabel(1024, 0)
	assert.Contains(t, label, "1.0 KiB")
}
