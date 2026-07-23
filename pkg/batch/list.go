// Package batch runs the download pipeline headlessly over a list of charts,
// so helmdownloader can be automated (CI, cron) without the interactive TUI.
package batch

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartRef identifies one chart to download, by ArtifactHub repository and
// chart name. Version is optional; empty means the latest published version.
type ChartRef struct {
	Repo    string
	Name    string
	Version string
}

// String renders the ref as "repo/name" plus an optional "@version".
func (r ChartRef) String() string {
	s := r.Repo + "/" + r.Name
	if r.Version != "" {
		s += "@" + r.Version
	}
	return s
}

// listFile is the YAML schema of a batch list. Each entry names a chart as
// "repo/name" (ArtifactHub coordinates) with an optional pinned version.
type listFile struct {
	Charts []struct {
		Chart   string `yaml:"chart"`
		Version string `yaml:"version"`
	} `yaml:"charts"`
}

// ParseList decodes a YAML batch list into chart references. It fails fast on
// malformed YAML, an empty list, or an entry whose chart is not "repo/name".
func ParseList(data []byte) ([]ChartRef, error) {
	var lf listFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse chart list: %w", err)
	}
	if len(lf.Charts) == 0 {
		return nil, fmt.Errorf("chart list is empty: expected a non-empty 'charts:' sequence")
	}
	refs := make([]ChartRef, 0, len(lf.Charts))
	for i, c := range lf.Charts {
		repo, name, ok := splitChart(c.Chart)
		if !ok {
			return nil, fmt.Errorf("chart entry %d: %q is not in 'repo/name' form", i+1, c.Chart)
		}
		refs = append(refs, ChartRef{Repo: repo, Name: name, Version: strings.TrimSpace(c.Version)})
	}
	return refs, nil
}

// splitChart splits "repo/name" on the first slash, trimming spaces. Both parts
// must be non-empty; anything else is rejected.
func splitChart(chart string) (repo, name string, ok bool) {
	repo, name, found := strings.Cut(strings.TrimSpace(chart), "/")
	repo, name = strings.TrimSpace(repo), strings.TrimSpace(name)
	if !found || repo == "" || name == "" {
		return "", "", false
	}
	return repo, name, true
}
