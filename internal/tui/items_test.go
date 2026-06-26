package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

func TestPackageItem_Title(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{Name: "argo-cd"}}
	assert.Equal(t, "argo-cd", item.Title())

	item = packageItem{pkg: artifacthub.Package{Name: "old", Deprecated: true}}
	assert.Equal(t, "old (deprecated)", item.Title())
}

func TestPackageItem_Description(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{
		Name:                    "argo-cd",
		RepoName:                "argo",
		Stars:                   200,
		AppVersion:              "2.9.3",
		Description:             "A declarative continuous deployment tool for Kubernetes.",
		Author:                  "jdoe",
		OrganizationDisplayName: "Argo Project",
	}}
	desc := item.Description()
	assert.Contains(t, desc, "★ 200")
	assert.Contains(t, desc, "repo:argo")
	assert.Contains(t, desc, "by:Argo Project")
	assert.Contains(t, desc, "app:2.9.3")
	assert.Contains(t, desc, "A declarative continuous deployment")
	// The description should be colored via ANSI escapes; the presence of a
	// reset sequence indicates inline styling was applied.
	assert.Contains(t, desc, "\x1b[")
}

func TestPackageItem_Description_FallsBackToRepoName(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{
		Name:     "redis",
		RepoName: "bitnami",
		Stars:    50,
	}}
	desc := item.Description()
	assert.Contains(t, desc, "by:bitnami")
	assert.NotContains(t, desc, "app:")
}

func TestPackageItem_FilterValue(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{
		Name:                    "argo-cd",
		RepoName:                "argo",
		Author:                  "jdoe",
		Organization:            "argoproj",
		OrganizationDisplayName: "Argo Project",
	}}
	got := item.FilterValue()
	for _, want := range []string{"argo-cd", "argo", "jdoe", "argoproj", "Argo Project"} {
		assert.True(t, strings.Contains(got, want), "FilterValue missing %q", want)
	}
}
