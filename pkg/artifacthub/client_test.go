package artifacthub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestSearch_ParsesPackages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/packages/search", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("kind"))
		assert.Equal(t, "argo-cd", r.URL.Query().Get("ts_query_web"))
		_, _ = w.Write([]byte(`{"packages":[
			{"name":"argo-cd","version":"5.0.0","app_version":"v2","description":"d","stars":42,"deprecated":false,
			 "repository":{"name":"argo","url":"https://argoproj.github.io/argo-helm","official":true}}
		]}`))
	}))
	defer srv.Close()

	client := artifacthub.New(srv.URL, log.Discard())
	pkgs, err := client.Search(context.Background(), "argo-cd", 20)
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "argo-cd", pkgs[0].Name)
	assert.Equal(t, "argo", pkgs[0].RepoName)
	assert.Equal(t, "https://argoproj.github.io/argo-helm", pkgs[0].RepoURL)
	assert.Equal(t, 42, pkgs[0].Stars)
	assert.True(t, pkgs[0].Official)
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

	client := artifacthub.New(srv.URL, log.Discard())
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

	client := artifacthub.New(srv.URL, log.Discard())
	_, err := client.Search(context.Background(), "x", 5)
	assert.Error(t, err)
}

func TestSearch_TransportErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // server is down: request fails at transport level

	client := artifacthub.New(srv.URL, log.Discard())
	_, err := client.Search(context.Background(), "x", 5)
	assert.Error(t, err)
}
