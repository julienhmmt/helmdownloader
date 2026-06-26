package tui

import (
	"sort"
	"strings"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
)

// sortField selects which Package field to sort by.
type sortField int

const (
	sortStars sortField = iota
	sortName
	sortUpdated
)

// sortDir selects ascending or descending order.
type sortDir int

const (
	sortAsc sortDir = iota
	sortDesc
)

// filterField selects which Package field to filter by substring.
type filterField int

const (
	filterNone filterField = iota
	filterAuthor
	filterCompany
)

// sortPackages returns a copy of packages sorted by field in the given
// direction. The input slice is not mutated. Ties preserve input order
// (sort.SliceStable) in both directions — descending uses a flipped
// comparator rather than a post-sort reversal, so tie order is stable.
func sortPackages(packages []artifacthub.Package, field sortField, dir sortDir) []artifacthub.Package {
	out := make([]artifacthub.Package, len(packages))
	copy(out, packages)
	cmp := func(i, j int) bool {
		switch field {
		case sortName:
			if dir == sortDesc {
				return out[i].Name > out[j].Name
			}
			return out[i].Name < out[j].Name
		case sortUpdated:
			if dir == sortDesc {
				return out[i].LastUpdated > out[j].LastUpdated
			}
			return out[i].LastUpdated < out[j].LastUpdated
		default: // sortStars
			if dir == sortDesc {
				return out[i].Stars > out[j].Stars
			}
			return out[i].Stars < out[j].Stars
		}
	}
	sort.SliceStable(out, cmp)
	return out
}

// filterPackages returns the packages whose author matches the author
// substring and whose company matches the company substring. Matching is
// case-insensitive. A package with no organization is matched by author only.
// Empty filter strings match everything.
func filterPackages(packages []artifacthub.Package, author, company string) []artifacthub.Package {
	authorLower := strings.ToLower(author)
	companyLower := strings.ToLower(company)
	out := make([]artifacthub.Package, 0, len(packages))
	for _, pkg := range packages {
		if authorLower != "" && !strings.Contains(strings.ToLower(pkg.Author), authorLower) {
			continue
		}
		if companyLower != "" && !companyMatches(pkg, companyLower) {
			continue
		}
		out = append(out, pkg)
	}
	return out
}

// companyMatches reports whether the package's organization name or display
// name contains the given lowercased substring.
func companyMatches(pkg artifacthub.Package, lower string) bool {
	return strings.Contains(strings.ToLower(pkg.Organization), lower) ||
		strings.Contains(strings.ToLower(pkg.OrganizationDisplayName), lower)
}

// uniqueAuthors returns the distinct non-empty author values from packages,
// preserving first-seen order.
func uniqueAuthors(packages []artifacthub.Package) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if pkg.Author == "" || seen[pkg.Author] {
			continue
		}
		seen[pkg.Author] = true
		out = append(out, pkg.Author)
	}
	return out
}

// uniqueCompanies returns the distinct non-empty organization names from
// packages, preserving first-seen order.
func uniqueCompanies(packages []artifacthub.Package) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if pkg.Organization == "" || seen[pkg.Organization] {
			continue
		}
		seen[pkg.Organization] = true
		out = append(out, pkg.Organization)
	}
	return out
}

// applySortFilter filters then sorts the packages, returning the result.
func applySortFilter(packages []artifacthub.Package, field sortField, dir sortDir, author, company string) []artifacthub.Package {
	return sortPackages(filterPackages(packages, author, company), field, dir)
}

// cycleSortField advances: stars → name → updated → stars.
func cycleSortField(f sortField) sortField {
	return (f + 1) % (sortUpdated + 1)
}

// toggleSortDir flips between ascending and descending.
func toggleSortDir(d sortDir) sortDir {
	if d == sortAsc {
		return sortDesc
	}
	return sortAsc
}

// cycleFilterField advances: none → author → company → none.
func cycleFilterField(f filterField) filterField {
	return (f + 1) % (filterCompany + 1)
}

// nextUniqueValue returns the value after current in values, wrapping to the
// first element. If current is empty, returns the first element. If values is
// empty, returns an empty string.
func nextUniqueValue(values []string, current string) string {
	if len(values) == 0 {
		return ""
	}
	for i, v := range values {
		if v == current {
			next := i + 1
			if next >= len(values) {
				next = 0
			}
			return values[next]
		}
	}
	return values[0]
}

// sortFieldLabel returns a human-readable label for a sort field.
func sortFieldLabel(f sortField) string {
	switch f {
	case sortName:
		return "name"
	case sortUpdated:
		return "updated"
	default:
		return "stars"
	}
}

// sortDirSymbol returns a compact arrow symbol for the sort direction.
func sortDirSymbol(d sortDir) string {
	if d == sortAsc {
		return "↑"
	}
	return "↓"
}

// filterFieldLabel returns a human-readable label for a filter field.
func filterFieldLabel(f filterField) string {
	switch f {
	case filterAuthor:
		return "author"
	case filterCompany:
		return "company"
	default:
		return "off"
	}
}
