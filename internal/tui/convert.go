package tui

import (
	"charm.land/bubbles/v2/list"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

// packagesToItems wraps packages as list items.
func packagesToItems(packages []artifacthub.Package) []list.Item {
	items := make([]list.Item, 0, len(packages))
	for _, pkg := range packages {
		items = append(items, packageItem{pkg: pkg})
	}
	return items
}

// versionsToItems wraps versions as list items.
func versionsToItems(versions []artifacthub.Version) []list.Item {
	items := make([]list.Item, 0, len(versions))
	for _, version := range versions {
		items = append(items, versionItem{version: version})
	}
	return items
}
