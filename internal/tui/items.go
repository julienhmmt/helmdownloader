package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

// packageItem adapts an ArtifactHub package to the bubbles list interface.
type packageItem struct {
	pkg artifacthub.Package
}

// Title renders the package's primary line.
func (i packageItem) Title() string {
	suffix := ""
	if i.pkg.Deprecated {
		suffix = " (deprecated)"
	}
	return i.pkg.Name + suffix
}

// Description renders the package's secondary line: stars, metadata, and the
// short description. Empty fields are omitted so the line never trails with
// "app:" or a dangling separator.
func (i packageItem) Description() string {
	publisher := i.publisher()
	star := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render(fmt.Sprintf("★ %d", i.pkg.Stars))

	metaStyle := lipgloss.NewStyle().Foreground(colorSecondary)
	metaParts := []string{
		metaStyle.Render("repo:" + i.pkg.RepoName),
		metaStyle.Render("by:" + publisher),
	}
	if i.pkg.AppVersion != "" {
		metaParts = append(metaParts, metaStyle.Render("app:"+i.pkg.AppVersion))
	}
	sep := lipgloss.NewStyle().Foreground(colorFaint).Render(" · ")
	meta := strings.Join(metaParts, sep)

	if i.pkg.Description == "" {
		return fmt.Sprintf("%s  %s", star, meta)
	}
	dot := lipgloss.NewStyle().Foreground(colorFaint).Render("· ")
	desc := dot + lipgloss.NewStyle().Foreground(colorMuted).Render(i.pkg.Description)
	return fmt.Sprintf("%s  %s  %s", star, meta, desc)
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
