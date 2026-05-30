package tui

import (
	"fmt"

	"github.com/julienhmmt/helmdownloader/internal/artifacthub"
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
	return fmt.Sprintf("%s  ★%d%s", i.pkg.Name, i.pkg.Stars, suffix)
}

// Description renders the package's secondary line.
func (i packageItem) Description() string {
	return fmt.Sprintf("repo:%s  app:%s  %s", i.pkg.RepoName, i.pkg.AppVersion, i.pkg.Description)
}

// FilterValue is the text used by the list's fuzzy filter.
func (i packageItem) FilterValue() string {
	return i.pkg.Name + " " + i.pkg.RepoName
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
