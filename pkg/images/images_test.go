package images_test

import (
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/images"
	"github.com/stretchr/testify/assert"
)

func TestExtract_FindsImagesAndDeduplicates(t *testing.T) {
	manifests := `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - name: init
          image: busybox:1.31.1
      containers:
        - name: app
          image: quay.io/argoproj/argocd:v3.2.6
        - name: app-dup
          image: quay.io/argoproj/argocd:v3.2.6
---
apiVersion: v1
kind: Pod
spec:
  containers:
    - image: docker.io/redis:8.2.2-alpine
`
	got := images.Extract(manifests)
	refs := make([]string, 0, len(got))
	for _, img := range got {
		assert.True(t, img.Selected)
		refs = append(refs, img.Ref)
	}
	assert.Equal(t, []string{
		"busybox:1.31.1",
		"docker.io/redis:8.2.2-alpine",
		"quay.io/argoproj/argocd:v3.2.6",
	}, refs)
}

func TestExtract_IgnoresNonImageValues(t *testing.T) {
	manifests := `
kind: Deployment
spec:
  image: ""
  imagePullPolicy: IfNotPresent
  containers:
    - image: "{{ .Values.image }}"
    - image: nginx:1.27
`
	got := images.Extract(manifests)
	assert.Len(t, got, 1)
	assert.Equal(t, "nginx:1.27", got[0].Ref)
}

func TestExtract_SplitRegistryRepositoryTag(t *testing.T) {
	// Numeric tag (YAML float), registry+repository, and a digest-only entry.
	values := `
controller:
  image:
    registry: registry.k8s.io
    repository: ingress-nginx/controller
    tag: "v1.11.2"
defaultBackend:
  image:
    repository: nginx
    tag: 1.27
admissionWebhooks:
  patch:
    image:
      registry: docker.io
      repository: library/busybox
      digest: sha256:deadbeef
`
	got := images.Extract(values)
	refs := make([]string, 0, len(got))
	for _, img := range got {
		refs = append(refs, img.Ref)
	}
	assert.Equal(t, []string{
		"docker.io/library/busybox@sha256:deadbeef",
		"nginx:1.27",
		"registry.k8s.io/ingress-nginx/controller:v1.11.2",
	}, refs)
}

func TestExtract_SplitFormIgnoresIncompleteBlocks(t *testing.T) {
	// No repository -> not an image; pullPolicy-only block is noise.
	values := `
image:
  pullPolicy: IfNotPresent
  tag: latest
sidecar:
  image:
    repository: ""
`
	got := images.Extract(values)
	assert.Empty(t, got)
}

func TestExtract_MergesMultipleSources(t *testing.T) {
	manifests := `
kind: Pod
spec:
  containers:
    - image: nginx:1.27
`
	values := `
image:
  repository: nginx
  tag: "1.27"
extra:
  image:
    repository: redis
    tag: "7"
`
	got := images.Extract(manifests, values)
	refs := make([]string, 0, len(got))
	for _, img := range got {
		refs = append(refs, img.Ref)
	}
	// nginx:1.27 appears in both sources but is deduplicated.
	assert.Equal(t, []string{"nginx:1.27", "redis:7"}, refs)
}

func TestRetag(t *testing.T) {
	tests := []struct {
		name   string
		ref    string
		prefix string
		want   string
	}{
		{
			name:   "fully qualified registry",
			ref:    "quay.io/argoproj/argocd:v3.2.6",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/quay.io/argoproj/argocd:v3.2.6",
		},
		{
			name:   "docker hub official shorthand",
			ref:    "redis:8.2.2-alpine",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/docker.io/library/redis:8.2.2-alpine",
		},
		{
			name:   "docker hub namespaced shorthand",
			ref:    "mattermost/mattermost-team-edition:10.9.1",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/docker.io/mattermost/mattermost-team-edition:10.9.1",
		},
		{
			// A digest cannot be a tarball tag, so digest-only refs fall back
			// to "latest" on the destination.
			name:   "digest reference falls back to latest",
			ref:    "ghcr.io/dexidp/dex@sha256:abcdef",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/ghcr.io/dexidp/dex:latest",
		},
		{
			// "repo:tag@digest" mirrors to the tag; the digest is dropped.
			name:   "tag and digest keeps the tag",
			ref:    "mattermost/mattermost-team-edition:11.7.2@sha256:fe5312",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/docker.io/mattermost/mattermost-team-edition:11.7.2",
		},
		{
			name:   "untagged ref falls back to latest",
			ref:    "ghcr.io/dexidp/dex",
			prefix: "rgy01.domain.local",
			want:   "rgy01.domain.local/ghcr.io/dexidp/dex:latest",
		},
		{
			name:   "empty prefix returns normalized ref",
			ref:    "redis:7",
			prefix: "",
			want:   "docker.io/library/redis:7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, images.Retag(tt.ref, tt.prefix))
		})
	}
}

func TestPullRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "tag only",
			ref:  "quay.io/argoproj/argocd:v3.2.6",
			want: "quay.io/argoproj/argocd:v3.2.6",
		},
		{
			name: "digest preferred over tag",
			ref:  "mattermost/mattermost-team-edition:11.7.2@sha256:fe5312",
			want: "docker.io/mattermost/mattermost-team-edition@sha256:fe5312",
		},
		{
			name: "digest only",
			ref:  "ghcr.io/dexidp/dex@sha256:abcdef",
			want: "ghcr.io/dexidp/dex@sha256:abcdef",
		},
		{
			name: "docker hub shorthand expands",
			ref:  "redis:7",
			want: "docker.io/library/redis:7",
		},
		{
			name: "untagged shorthand",
			ref:  "redis",
			want: "docker.io/library/redis",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, images.PullRef(tt.ref))
		})
	}
}
