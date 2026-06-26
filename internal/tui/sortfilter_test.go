package tui

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

// samplePackages returns a deterministic set used across sort/filter tests.
func samplePackages() []artifacthub.Package {
	return []artifacthub.Package{
		{Name: "redis", RepoName: "bitnami", Stars: 50, LastUpdated: 100,
			Author: "bitnami", Organization: "bitnami", OrganizationDisplayName: "Bitnami"},
		{Name: "argo-cd", RepoName: "argo", Stars: 200, LastUpdated: 300,
			Author: "jdoe", Organization: "argoproj", OrganizationDisplayName: "Argo Project"},
		{Name: "nginx", RepoName: "bitnami", Stars: 150, LastUpdated: 200,
			Author: "bitnami", Organization: "bitnami", OrganizationDisplayName: "Bitnami"},
		{Name: "traefik", RepoName: "traefik", Stars: 200, LastUpdated: 250,
			Author: "containous", Organization: "", OrganizationDisplayName: ""},
	}
}

func TestSortPackages_ByStarsDesc(t *testing.T) {
	got := sortPackages(samplePackages(), sortStars, sortDesc)
	require.Len(t, got, 4)
	// 200, 200, 150, 50 — ties keep stable input order (redis after argo-cd/traefik).
	assert.Equal(t, "argo-cd", got[0].Name)
	assert.Equal(t, "traefik", got[1].Name)
	assert.Equal(t, "nginx", got[2].Name)
	assert.Equal(t, "redis", got[3].Name)
}

func TestSortPackages_ByStarsAsc(t *testing.T) {
	got := sortPackages(samplePackages(), sortStars, sortAsc)
	require.Len(t, got, 4)
	assert.Equal(t, "redis", got[0].Name)
	assert.Equal(t, "nginx", got[1].Name)
	// argo-cd and traefik both 200 — stable order preserves input.
	assert.Equal(t, "argo-cd", got[2].Name)
	assert.Equal(t, "traefik", got[3].Name)
}

func TestSortPackages_ByNameAsc(t *testing.T) {
	got := sortPackages(samplePackages(), sortName, sortAsc)
	require.Len(t, got, 4)
	assert.Equal(t, "argo-cd", got[0].Name)
	assert.Equal(t, "nginx", got[1].Name)
	assert.Equal(t, "redis", got[2].Name)
	assert.Equal(t, "traefik", got[3].Name)
}

func TestSortPackages_ByNameDesc(t *testing.T) {
	got := sortPackages(samplePackages(), sortName, sortDesc)
	require.Len(t, got, 4)
	assert.Equal(t, "traefik", got[0].Name)
	assert.Equal(t, "redis", got[1].Name)
	assert.Equal(t, "nginx", got[2].Name)
	assert.Equal(t, "argo-cd", got[3].Name)
}

func TestSortPackages_ByUpdatedDesc(t *testing.T) {
	got := sortPackages(samplePackages(), sortUpdated, sortDesc)
	require.Len(t, got, 4)
	assert.Equal(t, "argo-cd", got[0].Name) // ts=300
	assert.Equal(t, "traefik", got[1].Name) // ts=250
	assert.Equal(t, "nginx", got[2].Name)   // ts=200
	assert.Equal(t, "redis", got[3].Name)   // ts=100
}

func TestSortPackages_ByUpdatedAsc(t *testing.T) {
	got := sortPackages(samplePackages(), sortUpdated, sortAsc)
	require.Len(t, got, 4)
	assert.Equal(t, "redis", got[0].Name)
	assert.Equal(t, "nginx", got[1].Name)
	assert.Equal(t, "traefik", got[2].Name)
	assert.Equal(t, "argo-cd", got[3].Name)
}

func TestSortPackages_DoesNotMutateInput(t *testing.T) {
	original := samplePackages()
	originalNames := make([]string, len(original))
	for i, p := range original {
		originalNames[i] = p.Name
	}
	_ = sortPackages(original, sortStars, sortDesc)
	gotNames := make([]string, len(original))
	for i, p := range original {
		gotNames[i] = p.Name
	}
	assert.Equal(t, originalNames, gotNames, "sortPackages must not reorder its input slice")
}

func TestSortPackages_Empty(t *testing.T) {
	got := sortPackages(nil, sortStars, sortDesc)
	assert.Empty(t, got)
}

func TestFilterPackages_ByAuthorSubstring(t *testing.T) {
	got := filterPackages(samplePackages(), "bit", "")
	require.Len(t, got, 2)
	assert.Equal(t, "redis", got[0].Name)
	assert.Equal(t, "nginx", got[1].Name)
}

func TestFilterPackages_ByAuthorCaseInsensitive(t *testing.T) {
	got := filterPackages(samplePackages(), "JDOE", "")
	require.Len(t, got, 1)
	assert.Equal(t, "argo-cd", got[0].Name)
}

func TestFilterPackages_ByCompanySubstring(t *testing.T) {
	got := filterPackages(samplePackages(), "", "argo")
	require.Len(t, got, 1)
	assert.Equal(t, "argo-cd", got[0].Name)
}

func TestFilterPackages_ByCompanyDisplayName(t *testing.T) {
	got := filterPackages(samplePackages(), "", "Bitnami")
	require.Len(t, got, 2)
	assert.Equal(t, "redis", got[0].Name)
	assert.Equal(t, "nginx", got[1].Name)
}

func TestFilterPackages_ByAuthorAndCompany(t *testing.T) {
	got := filterPackages(samplePackages(), "bit", "bitnami")
	require.Len(t, got, 2)
}

func TestFilterPackages_NoMatchReturnsEmpty(t *testing.T) {
	got := filterPackages(samplePackages(), "nobody", "")
	assert.Empty(t, got)
}

func TestFilterPackages_EmptyFiltersReturnAll(t *testing.T) {
	got := filterPackages(samplePackages(), "", "")
	require.Len(t, got, 4)
}

func TestFilterPackages_MatchesAuthorWhenNoOrganization(t *testing.T) {
	pkgs := []artifacthub.Package{
		{Name: "traefik", Author: "containous", Organization: "", OrganizationDisplayName: ""},
	}
	got := filterPackages(pkgs, "contain", "")
	require.Len(t, got, 1)
}

func TestUniqueAuthors(t *testing.T) {
	got := uniqueAuthors(samplePackages())
	sort.Strings(got)
	assert.Equal(t, []string{"bitnami", "containous", "jdoe"}, got)
}

func TestUniqueAuthors_Empty(t *testing.T) {
	assert.Empty(t, uniqueAuthors(nil))
}

func TestUniqueAuthors_DeduplicatesAndSkipsEmpty(t *testing.T) {
	pkgs := []artifacthub.Package{
		{Author: "a"}, {Author: "a"}, {Author: ""}, {Author: "b"},
	}
	got := uniqueAuthors(pkgs)
	sort.Strings(got)
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestUniqueCompanies(t *testing.T) {
	got := uniqueCompanies(samplePackages())
	sort.Strings(got)
	assert.Equal(t, []string{"argoproj", "bitnami"}, got)
}

func TestUniqueCompanies_Empty(t *testing.T) {
	assert.Empty(t, uniqueCompanies(nil))
}

func TestUniqueCompanies_DeduplicatesAndSkipsEmpty(t *testing.T) {
	pkgs := []artifacthub.Package{
		{Organization: "x"}, {Organization: "x"}, {Organization: ""},
	}
	got := uniqueCompanies(pkgs)
	assert.Equal(t, []string{"x"}, got)
}

func TestCycleSortField(t *testing.T) {
	assert.Equal(t, sortName, cycleSortField(sortStars))
	assert.Equal(t, sortUpdated, cycleSortField(sortName))
	assert.Equal(t, sortStars, cycleSortField(sortUpdated))
}

func TestToggleSortDir(t *testing.T) {
	assert.Equal(t, sortDesc, toggleSortDir(sortAsc))
	assert.Equal(t, sortAsc, toggleSortDir(sortDesc))
}

func TestCycleFilterField(t *testing.T) {
	assert.Equal(t, filterAuthor, cycleFilterField(filterNone))
	assert.Equal(t, filterCompany, cycleFilterField(filterAuthor))
	assert.Equal(t, filterNone, cycleFilterField(filterCompany))
}

func TestSortFieldLabel(t *testing.T) {
	assert.Equal(t, "stars", sortFieldLabel(sortStars))
	assert.Equal(t, "name", sortFieldLabel(sortName))
	assert.Equal(t, "updated", sortFieldLabel(sortUpdated))
}

func TestSortDirSymbol(t *testing.T) {
	assert.Equal(t, "↑", sortDirSymbol(sortAsc))
	assert.Equal(t, "↓", sortDirSymbol(sortDesc))
}

func TestFilterFieldLabel(t *testing.T) {
	assert.Equal(t, "off", filterFieldLabel(filterNone))
	assert.Equal(t, "author", filterFieldLabel(filterAuthor))
	assert.Equal(t, "company", filterFieldLabel(filterCompany))
}

func TestApplySortFilter_FiltersThenSorts(t *testing.T) {
	got := applySortFilter(samplePackages(), sortStars, sortDesc, "bit", "")
	require.Len(t, got, 2)
	// Both bitnami, sorted by stars desc: nginx(150) then redis(50).
	assert.Equal(t, "nginx", got[0].Name)
	assert.Equal(t, "redis", got[1].Name)
}

func TestApplySortFilter_NoFilterSortsAll(t *testing.T) {
	got := applySortFilter(samplePackages(), sortName, sortAsc, "", "")
	require.Len(t, got, 4)
	assert.Equal(t, "argo-cd", got[0].Name)
}

func TestNextUniqueValue_Cycles(t *testing.T) {
	values := []string{"a", "b", "c"}
	assert.Equal(t, "a", nextUniqueValue(values, ""))
	assert.Equal(t, "b", nextUniqueValue(values, "a"))
	assert.Equal(t, "c", nextUniqueValue(values, "b"))
	assert.Equal(t, "a", nextUniqueValue(values, "c"))
}

func TestNextUniqueValue_Empty(t *testing.T) {
	assert.Equal(t, "", nextUniqueValue(nil, ""))
	assert.Equal(t, "", nextUniqueValue([]string{}, "x"))
}
