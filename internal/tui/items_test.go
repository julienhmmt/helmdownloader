package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

func TestPackageItem_Title(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{Name: "argo-cd"}}
	assert.Equal(t, "argo-cd", item.Title())

	item = packageItem{pkg: artifacthub.Package{Name: "old", Deprecated: true}}
	assert.Equal(t, "old (deprecated)", item.Title())

	item = packageItem{pkg: artifacthub.Package{Name: "argo-cd", Official: true}}
	assert.Equal(t, "argo-cd · official", item.Title())

	item = packageItem{pkg: artifacthub.Package{Name: "old", Official: true, Deprecated: true}}
	assert.Equal(t, "old · official (deprecated)", item.Title())
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
	// Free-text chart description is intentionally omitted for scannability.
	assert.NotContains(t, desc, "A declarative continuous deployment")
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

func TestRelativeAge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	day := int64(24 * 3600)
	tests := []struct {
		name string
		ts   int64
		want string
	}{
		{name: "unset", ts: 0, want: ""},
		{name: "negative", ts: -5, want: ""},
		{name: "now", ts: now.Unix(), want: "today"},
		{name: "hours ago", ts: now.Unix() - 5*3600, want: "today"},
		{name: "days ago", ts: now.Unix() - 12*day, want: "12d ago"},
		{name: "months ago", ts: now.Unix() - 90*day, want: "3mo ago"},
		{name: "years ago", ts: now.Unix() - 800*day, want: "2y ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, relativeAge(tt.ts, now))
		})
	}
}

func TestPackageItem_Description_UpdatedAge(t *testing.T) {
	item := packageItem{pkg: artifacthub.Package{
		Name:        "argo-cd",
		RepoName:    "argo",
		LastUpdated: time.Now().Unix() - 48*3600,
	}}
	assert.Contains(t, item.Description(), "updated:")

	item.pkg.LastUpdated = 0
	assert.NotContains(t, item.Description(), "updated:")
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
