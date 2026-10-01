package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/bundle"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
)

// newResultsModel returns a model on the results screen with a known package
// set, so key-handler tests have deterministic state.
func newResultsModel() model {
	m := newModel(config.Default(), log.Discard())
	m.width, m.height = 100, 40
	m.state = stateResults
	m.allPackages = []artifacthub.Package{
		{Name: "redis", Stars: 50, Author: "bitnami", Organization: "bitnami",
			OrganizationDisplayName: "Bitnami", LastUpdated: 100},
		{Name: "argo-cd", Stars: 200, Author: "jdoe", Organization: "argoproj",
			OrganizationDisplayName: "Argo Project", LastUpdated: 300},
		{Name: "nginx", Stars: 150, Author: "bitnami", Organization: "bitnami",
			OrganizationDisplayName: "Bitnami", LastUpdated: 200},
	}
	m.results.SetItems(packagesToItems(m.visiblePackages(), m.styles.palette))
	return m
}

func keyPress(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 0, Text: key}
}

func TestDoneMsg_AppendsSessionBundle(t *testing.T) {
	type testCase struct {
		name     string
		included int
		missing  int
	}
	var cases = []testCase{{"complete", 3, 0}, {"partial", 2, 1}, {"chart only", 0, 0}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m = newTestModel()
			m.state = stateBundling
			m.entries = make([]bundle.ImageEntry, tc.included)
			m.failures = make([]pipeline.ImageFailure, tc.missing)
			var got tea.Model
			got, _ = m.Update(doneMsg{bundlePath: "/tmp/a.tar.gz"})
			var finished = got.(model)
			assert.Equal(t, stateDone, finished.state)
			assert.Equal(t, []sessionBundle{{path: "/tmp/a.tar.gz", included: tc.included, missing: tc.missing}}, finished.sessionBundles)
		})
	}
}

func TestAddAnotherChart_KeepsSessionBundles(t *testing.T) {
	m := newModel(config.Default(), log.Discard())
	m.width, m.height = 100, 40
	m.state = stateDone
	m.sessionBundles = []sessionBundle{{path: "/tmp/a.tar.gz", included: 2, missing: 1}}
	got, _ := m.handleEndKey(keyPress("a"))
	m2 := got.(model)
	assert.Equal(t, stateSearch, m2.state)
	assert.Equal(t, []sessionBundle{{path: "/tmp/a.tar.gz", included: 2, missing: 1}}, m2.sessionBundles, "add-another must carry bundles forward")

	// n starts a clean session — history dropped.
	fresh, _ := m.handleEndKey(keyPress("n"))
	assert.Empty(t, fresh.(model).sessionBundles)
}

func TestDoneMsg_KeepsEarlierPartial(t *testing.T) {
	var m = newTestModel()
	m.state = stateBundling
	m.entries = make([]bundle.ImageEntry, 2)
	m.failures = make([]pipeline.ImageFailure, 1)
	var got tea.Model
	got, _ = m.Update(doneMsg{bundlePath: "a.tar.gz"})
	got, _ = got.(model).handleEndKey(keyPress("a"))
	m = got.(model)
	m.state = stateBundling
	m.entries = make([]bundle.ImageEntry, 4)
	got, _ = m.Update(doneMsg{bundlePath: "b.tar.zst"})
	got, _ = got.(model).handleEndKey(keyPress("q"))
	assert.Equal(t, []sessionBundle{{path: "a.tar.gz", included: 2, missing: 1}, {path: "b.tar.zst", included: 4}}, got.(model).sessionBundles)
}

func TestDoneMsg_CountsUnbundledSelectedImages(t *testing.T) {
	var m = newTestModel()
	defer m.cancel()
	m.state = stateBundling
	m.entries = []bundle.ImageEntry{{SourceRef: "a:1"}}
	m.reviewImages = []images.Image{{Ref: "a:1", Selected: true}, {Ref: "b:1", Selected: true}, {Ref: "c:1", Selected: false}}
	var got tea.Model
	got, _ = m.Update(doneMsg{bundlePath: "partial.tar.gz"})
	assert.Equal(t, []sessionBundle{{path: "partial.tar.gz", included: 1, missing: 1}}, got.(model).sessionBundles)
}

func TestResetSession_SummaryHistory(t *testing.T) {
	for _, keep := range []bool{true, false} {
		t.Run(fmt.Sprintf("keep=%t", keep), func(t *testing.T) {
			var m = newTestModel()
			m.state = stateDone
			m.sessionBundles = []sessionBundle{{path: "partial.tar.gz", included: 2, missing: 1}}
			var fresh model
			fresh, _ = m.resetSession(keep)
			defer fresh.cancel()
			var output strings.Builder
			require.NoError(t, writeExitSummary(&output, fresh.sessionBundles))
			if keep {
				assert.Contains(t, output.String(), "PARTIAL: partial.tar.gz")
				assert.Contains(t, output.String(), "2 included, 1 missing")
			} else {
				assert.Empty(t, output.String())
			}
			assert.Equal(t, stateSearch, fresh.state)
			assert.Equal(t, m.cfg, fresh.cfg)
		})
	}
}

func TestDoneMsg_StaleIgnored(t *testing.T) {
	for _, current := range []state{stateSearch, stateReview, stateDone} {
		t.Run(fmt.Sprintf("state=%d", current), func(t *testing.T) {
			var m = newTestModel()
			defer m.cancel()
			m.state = current
			var got tea.Model
			var cmd tea.Cmd
			got, cmd = m.Update(doneMsg{bundlePath: "not-created.tar.gz"})
			assert.Empty(t, got.(model).sessionBundles)
			assert.Equal(t, current, got.(model).state)
			assert.Nil(t, cmd)
		})
	}
}

func TestHandleResultsKey_SCycleSortField(t *testing.T) {
	m := newResultsModel()
	require.Equal(t, sortStars, m.sortField)
	got, _ := m.handleResultsKey(keyPress("s"))
	m2 := got.(model)
	assert.Equal(t, sortName, m2.sortField)
}

func TestHandleResultsKey_STwiceCyclesToUpdated(t *testing.T) {
	m := newResultsModel()
	got, _ := m.handleResultsKey(keyPress("s"))
	got, _ = got.(model).handleResultsKey(keyPress("s"))
	assert.Equal(t, sortUpdated, got.(model).sortField)
}

func TestHandleResultsKey_OTogglesDir(t *testing.T) {
	m := newResultsModel()
	require.Equal(t, sortDesc, m.sortDir)
	got, _ := m.handleResultsKey(keyPress("o"))
	assert.Equal(t, sortAsc, got.(model).sortDir)
	got, _ = got.(model).handleResultsKey(keyPress("o"))
	assert.Equal(t, sortDesc, got.(model).sortDir)
}

func TestHandleResultsKey_FCyclesFilterField(t *testing.T) {
	m := newResultsModel()
	require.Equal(t, filterNone, m.filterField)
	got, _ := m.handleResultsKey(keyPress("f"))
	assert.Equal(t, filterAuthor, got.(model).filterField)
	got, _ = got.(model).handleResultsKey(keyPress("f"))
	assert.Equal(t, filterCompany, got.(model).filterField)
	got, _ = got.(model).handleResultsKey(keyPress("f"))
	assert.Equal(t, filterNone, got.(model).filterField)
}

func TestHandleResultsKey_FClearsValueWhenCycling(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	m.filterValue = "bit"
	got, _ := m.handleResultsKey(keyPress("f"))
	m2 := got.(model)
	assert.Equal(t, filterCompany, m2.filterField)
	assert.Equal(t, "", m2.filterValue) // value cleared when field changes
	got, _ = m2.handleResultsKey(keyPress("f"))
	assert.Equal(t, filterNone, got.(model).filterField)
	assert.Equal(t, "", got.(model).filterValue)
}

func TestHandleResultsKey_ShiftFOpensFilterInput(t *testing.T) {
	m := newResultsModel()
	got, _ := m.handleResultsKey(keyPress("F"))
	m2 := got.(model)
	assert.Equal(t, stateFilterInput, m2.state)
	assert.Equal(t, filterAuthor, m2.filterField) // defaults to author
}

func TestHandleResultsKey_TabCyclesAuthorValues(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	got, _ := m.handleResultsKey(keyPress("tab"))
	m2 := got.(model)
	// first cycle from empty -> first unique author (bitnami, first-seen)
	assert.Equal(t, "bitnami", m2.filterValue)
}

func TestHandleResultsKey_TabCyclesCompanyValues(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterCompany
	got, _ := m.handleResultsKey(keyPress("tab"))
	assert.Equal(t, "bitnami", got.(model).filterValue)
}

func TestHandleResultsKey_TabNoopWhenFilterOff(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterNone
	got, _ := m.handleResultsKey(keyPress("tab"))
	assert.Equal(t, "", got.(model).filterValue)
}

func TestHandleResultsKey_SortAppliesToVisiblePackages(t *testing.T) {
	m := newResultsModel()
	// default: stars desc -> argo-cd(200), nginx(150), redis(50)
	assert.Equal(t, "argo-cd", m.visiblePackages()[0].Name)
	// toggle to asc -> redis(50), nginx(150), argo-cd(200)
	got, _ := m.handleResultsKey(keyPress("o"))
	assert.Equal(t, "redis", got.(model).visiblePackages()[0].Name)
}

func TestHandleResultsKey_FilterReducesVisiblePackages(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	m.filterValue = "bit"
	// bitnami authored redis and nginx -> 2 visible
	assert.Len(t, m.visiblePackages(), 2)
}

func TestHandleFilterInputKey_EnterAppliesFilter(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	m.state = stateFilterInput
	m.filter.SetValue("jdoe")
	got, _ := m.handleFilterInputKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateResults, m2.state)
	assert.Equal(t, "jdoe", m2.filterValue)
	assert.Len(t, m2.visiblePackages(), 1) // only argo-cd by jdoe
}

func TestHandleFilterInputKey_EscCancels(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	m.filterValue = "bit"
	m.state = stateFilterInput
	m.filter.SetValue("jdoe")
	got, _ := m.handleFilterInputKey(keyPress("esc"))
	m2 := got.(model)
	assert.Equal(t, stateResults, m2.state)
	// esc cancels: the old filterValue is preserved, the typed text discarded
	assert.Equal(t, "bit", m2.filterValue)
}

func TestHandleFilterInputKey_TabCyclesValues(t *testing.T) {
	m := newResultsModel()
	m.filterField = filterAuthor
	m.state = stateFilterInput
	got, _ := m.handleFilterInputKey(keyPress("tab"))
	m2 := got.(model)
	assert.Equal(t, "bitnami", m2.filterValue)
	assert.Equal(t, "bitnami", m2.filter.Value())
}

func TestSearchResultMsg_StoresAllPackagesAndAppliesSortFilter(t *testing.T) {
	m := newResultsModel()
	m.state = stateSearching // required so the result is not treated as stale
	m.allPackages = nil
	m.results.SetItems(nil)
	m.filterField = filterAuthor
	m.filterValue = "bitnami"
	pkgs := []artifacthub.Package{
		{Name: "redis", Stars: 50, Author: "bitnami"},
		{Name: "argo-cd", Stars: 200, Author: "jdoe"},
	}
	got, _ := m.Update(searchResultMsg{packages: pkgs})
	m2 := got.(model)
	assert.Equal(t, stateResults, m2.state)
	assert.Len(t, m2.allPackages, 2)
	assert.Equal(t, filterNone, m2.filterField) // filter reset on new search
	assert.Equal(t, "", m2.filterValue)
	// default sort stars desc -> argo-cd first
	assert.Equal(t, "argo-cd", m2.visiblePackages()[0].Name)
}

func TestVisiblePackages_RespectsSortAndFilter(t *testing.T) {
	m := newResultsModel()
	m.sortField = sortName
	m.sortDir = sortAsc
	m.filterField = filterCompany
	m.filterValue = "bitnami"
	got := m.visiblePackages()
	require.Len(t, got, 2) // redis + nginx
	assert.Equal(t, "nginx", got[0].Name)
	assert.Equal(t, "redis", got[1].Name)
}

func TestDownloadDoneMsg_StaleWhileReviewIgnored(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.entries = nil
	got, cmd := m.Update(downloadDoneMsg{
		entries: []bundle.ImageEntry{{SourceRef: "x:1"}},
	})
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.Empty(t, m2.entries)
	assert.Nil(t, cmd)
}

func TestHandleBusyKey_FirstEscArmsDownloadCancel(t *testing.T) {
	m := newTestModel()
	m.state = stateDownloading
	m.errStep = "download"
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	assert.Equal(t, stateDownloading, m2.state)
	assert.True(t, m2.cancelArmed)
	assert.Contains(t, m2.status, "esc again")
}

func TestHandleBusyKey_EscDownloadingReturnsReview(t *testing.T) {
	m := newTestModel()
	m.state = stateDownloading
	m.errStep = "download"
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	got, _ = m2.handleBusyKey(keyPress("esc"))
	m3 := got.(model)
	assert.Equal(t, stateReview, m3.state)
	assert.Empty(t, m3.errStep)
	assert.False(t, m3.cancelArmed)
	assert.Empty(t, m3.status)
}

func TestHandleBusyKey_EscDownloadingWithEntriesGoesDownloadReview(t *testing.T) {
	m := newTestModel()
	m.state = stateDownloading
	m.entries = []bundle.ImageEntry{{SourceRef: "x:1"}}
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	got, _ = m2.handleBusyKey(keyPress("esc"))
	assert.Equal(t, stateDownloadReview, got.(model).state)
}

func TestHandleBusyKey_OtherKeyDisarmsDownloadCancel(t *testing.T) {
	m := newTestModel()
	m.state = stateDownloading
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	require.True(t, m2.cancelArmed)
	got, _ = m2.handleBusyKey(keyPress("j"))
	m3 := got.(model)
	assert.Equal(t, stateDownloading, m3.state)
	assert.False(t, m3.cancelArmed)
	assert.Empty(t, m3.status)
	// A fresh esc re-arms instead of cancelling.
	got, _ = m3.handleBusyKey(keyPress("esc"))
	m4 := got.(model)
	assert.Equal(t, stateDownloading, m4.state)
	assert.True(t, m4.cancelArmed)
}

func TestHandleBusyKey_EscSearchingBackToSearch(t *testing.T) {
	m := newTestModel()
	m.state = stateSearching
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	assert.Equal(t, stateSearch, m2.state)
}

func TestHandleBusyKey_EscPreparingGoesVersions(t *testing.T) {
	m := newTestModel()
	m.state = statePreparing
	m.selectedPkg = artifacthub.Package{Name: "argo-cd"}
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	assert.Equal(t, stateVersions, m2.state)
}

func TestHandleBusyKey_EscBundlingIsNoop(t *testing.T) {
	m := newTestModel()
	m.state = stateBundling
	got, _ := m.handleBusyKey(keyPress("esc"))
	m2 := got.(model)
	assert.Equal(t, stateBundling, m2.state)
}

func TestHandleSearchKey_EmptyEnterSetsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateSearch
	got, _ := m.handleSearchKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateSearch, m2.state)
	assert.Equal(t, "Type a chart name to search.", m2.status)
}

func TestHandleSearchKey_EnterClearsStatus(t *testing.T) {
	m := newTestModel()
	m.state = stateSearch
	m.search.SetValue("argo-cd")
	m.setStatus("Type a chart name to search.")
	got, _ := m.handleSearchKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateSearching, m2.state)
	assert.Empty(t, m2.status)
}

func TestHandleReviewKey_ChartOnlyEnterBundles(t *testing.T) {
	// A CRD chart discovers no images: enter must skip download and go straight
	// to bundling instead of demanding an image selection.
	m := newTestModel()
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "crd"}
	m.selectedVersion = "1.0.0"
	m.reviewImages = nil
	got, cmd := m.handleReviewKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateBundling, m2.state)
	assert.NotNil(t, cmd)
	assert.Empty(t, m2.status)
}

func TestHandleReviewKey_ChartOnlyDeprecatedRequiresSecondEnter(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "old-crd", Deprecated: true}
	m.selectedVersion = "1.0.0"
	m.reviewImages = nil
	got, cmd := m.handleReviewKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.True(t, m2.reviewWarnAck)
	assert.Contains(t, m2.status, "deprecated")
	assert.Nil(t, cmd)
	got, cmd = m2.handleReviewKey(keyPress("enter"))
	m3 := got.(model)
	assert.Equal(t, stateBundling, m3.state)
	assert.NotNil(t, cmd)
}

func TestPreparedMsg_RestoresSavedReviewIncludingEmpty(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty=%t", empty), func(t *testing.T) {
			var m = newTestModel()
			defer m.cancel()
			m.state = statePreparing
			m.cfg.ImportImages = filepath.Join(t.TempDir(), "saved.json")
			m.cfg.ExportImages = filepath.Join(t.TempDir(), "discovered.json")
			var approved = []images.Image{{Ref: "busybox:1", Selected: false}}
			if empty {
				approved = []images.Image{}
			}
			require.NoError(t, exportImages(m.cfg.ImportImages, approved))
			var discovered = []images.Image{{Ref: "redis:7", Selected: true}}
			var got, _ = m.Update(preparedMsg{prepared: pipeline.Prepared{Images: discovered}})
			var review = got.(model)
			assert.Equal(t, stateReview, review.state)
			assert.Equal(t, approved, review.reviewImages)
			var exported, err = importImages(m.cfg.ExportImages)
			require.NoError(t, err)
			assert.Equal(t, discovered, exported)
			if empty {
				got, _ = review.handleReviewKey(keyPress("enter"))
				assert.Equal(t, stateBundling, got.(model).state)
				assert.Empty(t, got.(model).prepared.Images)
			}
		})
	}
}

func TestPreparedMsg_ImportImagesOverridesEmptyDiscovery(t *testing.T) {
	// A chart with no discovered images must still honour -import-images when
	// entering review so the operator sees the approved list before download.
	path := filepath.Join(t.TempDir(), "approved.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{"ref": "nginx:1.27", "selected": true},
		{"ref": "redis:7", "selected": false}
	]`), 0o644))
	m := newTestModel()
	m.state = statePreparing
	m.cfg.ImportImages = path
	got, cmd := m.Update(preparedMsg{prepared: pipeline.Prepared{Images: nil}})
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.Nil(t, cmd)
	assert.Len(t, m2.reviewImages, 2)
	assert.True(t, m2.reviewImages[0].Selected)
	assert.False(t, m2.reviewImages[1].Selected)
	got, cmd = m2.handleReviewKey(keyPress("enter"))
	m3 := got.(model)
	assert.Equal(t, stateDownloading, m3.state)
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m3.downTotal)
}

func TestPreparedMsg_ImportImagesErrorFailsClosed(t *testing.T) {
	m := newTestModel()
	m.state = statePreparing
	m.cfg.ImportImages = filepath.Join(t.TempDir(), "missing.json")
	got, _ := m.Update(preparedMsg{prepared: pipeline.Prepared{}})
	m2 := got.(model)
	assert.Equal(t, stateError, m2.state)
	assert.Equal(t, "prepare", m2.errStep)
	assert.Error(t, m2.err)
}

func TestHandleReviewKey_ImportEditsSurviveWarnAck(t *testing.T) {
	// After deprecated ack, Enter must not re-import and wipe space/d edits.
	path := filepath.Join(t.TempDir(), "approved.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{"ref": "nginx:1.27", "selected": true},
		{"ref": "redis:7", "selected": true}
	]`), 0o644))
	m := newTestModel()
	m.state = statePreparing
	m.selectedPkg = artifacthub.Package{Name: "old", Deprecated: true}
	m.selectedVersion = "1.0.0"
	m.cfg.ImportImages = path
	got, _ := m.Update(preparedMsg{prepared: pipeline.Prepared{Images: nil}})
	m2 := got.(model)
	require.Equal(t, stateReview, m2.state)
	require.Len(t, m2.reviewImages, 2)
	// Deselect nginx before the safety gate.
	m2.reviewCursor = 0
	got, _ = m2.handleReviewKey(keyPress("space"))
	m2 = got.(model)
	assert.False(t, m2.reviewImages[0].Selected)
	got, cmd := m2.handleReviewKey(keyPress("enter"))
	m3 := got.(model)
	assert.Equal(t, stateReview, m3.state)
	assert.True(t, m3.reviewWarnAck)
	assert.Nil(t, cmd)
	assert.False(t, m3.reviewImages[0].Selected, "toggle must survive warn ack")
	got, cmd = m3.handleReviewKey(keyPress("enter"))
	m4 := got.(model)
	assert.Equal(t, stateDownloading, m4.state)
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m4.downTotal, "only still-selected redis should download")
	assert.False(t, m4.reviewImages[0].Selected)
	assert.True(t, m4.reviewImages[1].Selected)
}

func TestHandleReviewKey_InvertSelect(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.reviewImages = []images.Image{
		{Ref: "a:1", Selected: true},
		{Ref: "b:1", Selected: false},
		{Ref: "c:1", Selected: true},
	}
	got, _ := m.handleReviewKey(keyPress("i"))
	m2 := got.(model)
	assert.Equal(t, []bool{false, true, false},
		[]bool{m2.reviewImages[0].Selected, m2.reviewImages[1].Selected, m2.reviewImages[2].Selected})
}

func TestHandleReviewKey_DeprecatedRequiresSecondEnter(t *testing.T) {
	m := newTestModel()
	m.state = stateReview
	m.selectedPkg = artifacthub.Package{Name: "old", Deprecated: true}
	m.selectedVersion = "1.0.0"
	m.reviewImages = []images.Image{{Ref: "nginx:1.27", Selected: true}}
	got, cmd := m.handleReviewKey(keyPress("enter"))
	m2 := got.(model)
	assert.Equal(t, stateReview, m2.state)
	assert.True(t, m2.reviewWarnAck)
	assert.Contains(t, m2.status, "deprecated")
	assert.Nil(t, cmd)

	got, cmd = m2.handleReviewKey(keyPress("enter"))
	m3 := got.(model)
	assert.Equal(t, stateDownloading, m3.state)
	assert.NotNil(t, cmd)
}
