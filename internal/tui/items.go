package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

// packageItem adapts an ArtifactHub package to the bubbles list interface.
type packageItem struct {
	pkg     artifacthub.Package
	palette palette
}

// Title renders the package's primary line: name plus optional badges.
// Badges are plain text (no lipgloss) so the list's FilterMatch still works.
func (i packageItem) Title() string {
	parts := []string{i.pkg.Name}
	if i.pkg.Official {
		parts = append(parts, "· official")
	}
	if i.pkg.Deprecated {
		parts = append(parts, "(deprecated)")
	}
	return strings.Join(parts, " ")
}

// Description renders the secondary meta line: stars, repo, publisher, and
// optional app version. Free-text chart descriptions are omitted so the list
// stays scannable under stress (operators can open ArtifactHub for prose).
func (i packageItem) Description() string {
	publisher := i.publisher()
	star := lipgloss.NewStyle().Foreground(i.palette.accent).Bold(true).Render(fmt.Sprintf("★ %d", i.pkg.Stars))
	metaStyle := lipgloss.NewStyle().Foreground(i.palette.secondary)
	metaParts := []string{
		metaStyle.Render("repo:" + i.pkg.RepoName),
		metaStyle.Render("by:" + publisher),
	}
	if i.pkg.AppVersion != "" {
		metaParts = append(metaParts, metaStyle.Render("app:"+i.pkg.AppVersion))
	}
	sep := lipgloss.NewStyle().Foreground(i.palette.faint).Render(" · ")
	return fmt.Sprintf("%s  %s", star, strings.Join(metaParts, sep))
}

// publisher returns the best human-readable publisher name for the package.
func (i packageItem) publisher() string {
	if i.pkg.OrganizationDisplayName != "" {
		return i.pkg.OrganizationDisplayName
	}
	if i.pkg.Author != "" {
		return i.pkg.Author
	}
	return i.pkg.RepoName
}

// FilterValue is the text used by the list's fuzzy filter.
func (i packageItem) FilterValue() string {
	return i.pkg.Name + " " + i.pkg.RepoName + " " + i.pkg.Author + " " + i.pkg.Organization + " " + i.pkg.OrganizationDisplayName
}

// versionItem adapts an ArtifactHub version to the bubbles list interface.
type versionItem struct {
	version artifacthub.Version
}

// Title renders the version string.
func (i versionItem) Title() string {
	tag := ""
	if i.version.Prerelease {
		tag = " (prerelease)"
	}
	return i.version.Version + tag
}

// Description renders the app version backing this chart version.
func (i versionItem) Description() string {
	return "app version: " + i.version.AppVersion
}

// FilterValue is the text used by the list's fuzzy filter.
func (i versionItem) FilterValue() string {
	return i.version.Version
}
