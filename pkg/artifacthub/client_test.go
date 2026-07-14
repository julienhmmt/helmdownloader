package artifacthub_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/log"
)

func TestIsOCI(t *testing.T) {
	assert.True(t, artifacthub.Package{RepoURL: "oci://registry.local/charts"}.IsOCI())
	assert.False(t, artifacthub.Package{RepoURL: "https://charts.example.com"}.IsOCI())
}

func mustClient(t *testing.T, baseURL, proxy string) *artifacthub.Client {
	t.Helper()
	client, err := artifacthub.New(baseURL, proxy, log.Discard())
	require.NoError(t, err)
	return client
}

func TestSearch_ParsesPackages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/packages/search", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("kind"))
		assert.Equal(t, "argo-cd", r.URL.Query().Get("ts_query_web"))
		_, _ = w.Write([]byte(`{"packages":[
			{"name":"argo-cd","version":"5.0.0","app_version":"v2","description":"d","stars":42,"deprecated":false,"ts":1700000000,
			 "repository":{"name":"argo","display_name":"Argo","url":"https://argoproj.github.io/argo-helm","official":true,
			  "user_alias":"jdoe","organization_name":"argoproj","organization_display_name":"Argo Project"}}
		]}`))
	}))
	defer srv.Close()

	client := mustClient(t, srv.URL, "")
	pkgs, err := client.Search(context.Background(), "argo-cd", 20)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "argo-cd", pkgs[0].Name)
	assert.Equal(t, "argo", pkgs[0].RepoName)
	assert.Equal(t, "Argo", pkgs[0].RepoDisplayName)
	assert.Equal(t, "https://argoproj.github.io/argo-helm", pkgs[0].RepoURL)
	assert.Equal(t, 42, pkgs[0].Stars)
	assert.True(t, pkgs[0].Official)
	assert.Equal(t, "jdoe", pkgs[0].Author)
	assert.Equal(t, "argoproj", pkgs[0].Organization)
	assert.Equal(t, "Argo Project", pkgs[0].OrganizationDisplayName)
	assert.Equal(t, int64(1700000000), pkgs[0].LastUpdated)
}

func TestVersions_ParsesAndEscapesPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/packages/helm/argo/argo-cd", r.URL.Path)
		_, _ = w.Write([]byte(`{"available_versions":[
			{"version":"5.0.0","app_version":"v2","prerelease":false,"ts":100},
			{"version":"4.0.0","app_version":"v1","prerelease":true,"ts":50}
		]}`))
	}))
	defer srv.Close()

	client := mustClient(t, srv.URL, "")
	versions, err := client.Versions(context.Background(), "argo", "argo-cd")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, "5.0.0", versions[0].Version)
	assert.True(t, versions[1].Prerelease)
	assert.Equal(t, int64(100), versions[0].Timestamp)
}

func TestGetJSON_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := mustClient(t, srv.URL, "")
	_, err := client.Search(context.Background(), "x", 5)
	assert.Error(t, err)
}

func TestSearch_TransportErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // server is down: request fails at transport level

	client := mustClient(t, srv.URL, "")
	_, err := client.Search(context.Background(), "x", 5)
	assert.Error(t, err)
}

func TestSearch_RejectsOversizedBody(t *testing.T) {
	// An oversized response body (maxArtifactHubBody+1 bytes of spaces) is
	// truncated by the io.LimitReader cap, so the JSON decoder sees a
	// truncated stream and errors instead of consuming unbounded memory.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(make([]byte, 16<<20+1))
	}))
	defer srv.Close()

	client := mustClient(t, srv.URL, "")
	_, err := client.Search(context.Background(), "x", 5)
	assert.Error(t, err)
}

func TestNew_InvalidProxy(t *testing.T) {
	_, err := artifacthub.New("https://example.com", "://bad", log.Discard())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy")
}

func TestNew_EmptyProxyUsesDefaultTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"packages":[]}`))
	}))
	defer srv.Close()
	client, err := artifacthub.New(srv.URL, "", log.Discard())
	require.NoError(t, err)
	_, err = client.Search(context.Background(), "x", 1)
	require.NoError(t, err)
}

func TestNew_ProxyIsUsed(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/packages/search", r.URL.Path)
		_, _ = w.Write([]byte(`{"packages":[]}`))
	}))
	defer backend.Close()

	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		// Forward absolute-form HTTP proxy requests to the backend.
		req, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
		require.NoError(t, err)
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer proxy.Close()

	client, err := artifacthub.New(backend.URL, proxy.URL, log.Discard())
	require.NoError(t, err)
	_, err = client.Search(context.Background(), "x", 1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, proxyHits.Load(), int32(1))
}
