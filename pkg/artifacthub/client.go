// Package artifacthub is a minimal client for the ArtifactHub API, used to
// search Helm charts and list their available versions.
package artifacthub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienhmmt/helmdownloader/pkg/log"
)

const helmKind = "0"

// maxArtifactHubBody caps the size of an ArtifactHub response body before
// decoding, so a runaway or compromised hub response cannot exhaust memory.
// 16 MiB is well above any real search/versions payload.
const maxArtifactHubBody = 16 << 20

// Package is a single Helm chart entry returned by a search.
type Package struct {
	Name        string
	RepoName    string
	RepoURL     string
	Version     string
	AppVersion  string
	Description string
	Stars       int
	Official    bool
	Deprecated  bool
}

// IsOCI reports whether the chart is hosted in an OCI registry.
func (p Package) IsOCI() bool {
	return strings.HasPrefix(p.RepoURL, "oci://")
}

// Version is one published revision of a chart.
type Version struct {
	Version    string
	AppVersion string
	Prerelease bool
	Timestamp  int64
}

// Client talks to the ArtifactHub HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
	logger  *log.Logger
}

// New returns a Client targeting baseURL (e.g. "https://artifacthub.io").
func New(baseURL string, logger *log.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
		logger:  logger,
	}
}

// Search returns Helm charts matching query, capped at limit results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Package, error) {
	c.logger.Infof("searching ArtifactHub for %q (limit %d)", query, limit)
	q := url.Values{}
	q.Set("kind", helmKind)
	q.Set("ts_query_web", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("facets", "false")
	endpoint := fmt.Sprintf("%s/api/v1/packages/search?%s", c.baseURL, q.Encode())
	c.logger.Debugf("GET %s", endpoint)
	var payload searchResponse
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	packages := make([]Package, 0, len(payload.Packages))
	for _, raw := range payload.Packages {
		packages = append(packages, raw.toPackage())
	}
	c.logger.Infof("found %d packages", len(packages))
	return packages, nil
}

// Versions returns every published version of the chart identified by
// repoName/name, newest first.
func (c *Client) Versions(ctx context.Context, repoName, name string) ([]Version, error) {
	c.logger.Infof("fetching versions for %s/%s", repoName, name)
	endpoint := fmt.Sprintf("%s/api/v1/packages/helm/%s/%s",
		c.baseURL, url.PathEscape(repoName), url.PathEscape(name))
	c.logger.Debugf("GET %s", endpoint)
	var payload detailResponse
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	versions := make([]Version, 0, len(payload.AvailableVersions))
	for _, raw := range payload.AvailableVersions {
		versions = append(versions, raw.toVersion())
	}
	c.logger.Infof("found %d versions", len(versions))
	return versions, nil
}

// getJSON performs a GET request and decodes the JSON body into out.
func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artifacthub: unexpected status %d for %s", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxArtifactHubBody)).Decode(out)
}

type searchResponse struct {
	Packages []rawPackage `json:"packages"`
}

type rawRepository struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Official bool   `json:"official"`
}

type rawPackage struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	AppVersion  string        `json:"app_version"`
	Description string        `json:"description"`
	Stars       int           `json:"stars"`
	Deprecated  bool          `json:"deprecated"`
	Repository  rawRepository `json:"repository"`
}

func (r rawPackage) toPackage() Package {
	return Package{
		Name:        r.Name,
		RepoName:    r.Repository.Name,
		RepoURL:     r.Repository.URL,
		Version:     r.Version,
		AppVersion:  r.AppVersion,
		Description: r.Description,
		Stars:       r.Stars,
		Official:    r.Repository.Official,
		Deprecated:  r.Deprecated,
	}
}

type detailResponse struct {
	AvailableVersions []rawVersion `json:"available_versions"`
}

type rawVersion struct {
	Version    string `json:"version"`
	AppVersion string `json:"app_version"`
	Prerelease bool   `json:"prerelease"`
	Timestamp  int64  `json:"ts"`
}

func (r rawVersion) toVersion() Version {
	return Version(r)
}
