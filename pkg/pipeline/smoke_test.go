package pipeline_test

import (
	"context"
	"os"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/helm"
	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipUnlessSmoke skips network/helm-dependent smoke tests in -short mode so
// the unit suite stays runnable offline. Run them with: go test -run Smoke .
func skipUnlessSmoke(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping network/helm smoke test in -short mode")
	}
}

func TestSmoke_ArtifactHubSearch(t *testing.T) {
	skipUnlessSmoke(t)
	client := artifacthub.New("https://artifacthub.io", log.Discard())
	packages, err := client.Search(context.Background(), "argo-cd", 20)
	require.NoError(t, err)
	require.NotEmpty(t, packages)
	t.Logf("found %d packages", len(packages))
	for _, p := range packages {
		t.Logf("  %s repo:%s url:%s", p.Name, p.RepoName, p.RepoURL)
	}
}

func TestSmoke_HelmPullAndExtract(t *testing.T) {
	skipUnlessSmoke(t)
	workDir, err := os.MkdirTemp("", "helmdownloader-smoke-")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	client := helm.New("helm", "", log.Discard())
	// Use latest ingress-nginx chart
	pull, err := client.Pull(context.Background(), "ingress-nginx",
		"https://kubernetes.github.io/ingress-nginx",
		"4.15.1", workDir, false)
	require.NoError(t, err)
	assert.FileExists(t, pull.ChartPath)

	manifests, err := client.Template(context.Background(), pull.ChartPath)
	require.NoError(t, err)
	assert.Contains(t, manifests, "Deployment")

	imgs := images.Extract(manifests)
	require.NotEmpty(t, imgs)
	t.Logf("found %d images:", len(imgs))
	for _, img := range imgs {
		t.Logf("  %s", img.Ref)
	}
}

func TestSmoke_PipelinePrepare(t *testing.T) {
	skipUnlessSmoke(t)
	client := artifacthub.New("https://artifacthub.io", log.Discard())
	packages, err := client.Search(context.Background(), "argo-cd", 20)
	require.NoError(t, err)
	require.NotEmpty(t, packages)

	// Prefer a classic HTTP repo over OCI for template extraction
	var pkg artifacthub.Package
	for _, p := range packages {
		if p.Name == "argo-cd" && !p.IsOCI() {
			pkg = p
			break
		}
	}
	if pkg.Name == "" {
		for _, p := range packages {
			if p.Name == "argo-cd" {
				pkg = p
				break
			}
		}
	}
	require.NotEmpty(t, pkg.Name, "could not find argo-cd")
	t.Logf("selected: %s from repo:%s url:%s oci:%v", pkg.Name, pkg.RepoName, pkg.RepoURL, pkg.IsOCI())

	versions, err := client.Versions(context.Background(), pkg.RepoName, pkg.Name)
	require.NoError(t, err)
	require.NotEmpty(t, versions)
	version := versions[0].Version
	t.Logf("latest version: %s (app %s)", version, versions[0].AppVersion)

	cfg := config.Config{
		RegistryPrefix: "my.registry.local",
		Platform:       "linux/amd64",
		OutputDir:      "/tmp/helmdownloader-smoke-output",
		ArtifactHubURL: "https://artifacthub.io",
		SearchLimit:    20,
		HelmBin:        "helm",
	}

	pl := pipeline.New(cfg, log.Discard())
	prepared, err := pl.Prepare(context.Background(), pkg, version)
	require.NoError(t, err)
	require.NotEmpty(t, prepared.ChartPath)
	require.FileExists(t, prepared.ChartPath)

	t.Logf("chart: %s", prepared.ChartPath)
	t.Logf("images found: %d", len(prepared.Images))
	for _, img := range prepared.Images {
		t.Logf("  %s -> %s", img.Ref, images.Retag(img.Ref, cfg.RegistryPrefix))
	}

	_ = os.RemoveAll(prepared.WorkDir)
	_ = os.RemoveAll(cfg.OutputDir)
}
