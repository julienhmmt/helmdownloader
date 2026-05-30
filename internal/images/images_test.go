package images_test

import (
	"testing"

	"github.com/julienhmmt/helmdownloader/internal/images"
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
			prefix: "rgy01.kalos.tdmc",
			want:   "rgy01.kalos.tdmc/quay.io/argoproj/argocd:v3.2.6",
		},
		{
			name:   "docker hub official shorthand",
			ref:    "redis:8.2.2-alpine",
			prefix: "rgy01.kalos.tdmc",
			want:   "rgy01.kalos.tdmc/docker.io/library/redis:8.2.2-alpine",
		},
		{
			name:   "docker hub namespaced shorthand",
			ref:    "mattermost/mattermost-team-edition:10.9.1",
			prefix: "rgy01.kalos.tdmc",
			want:   "rgy01.kalos.tdmc/docker.io/mattermost/mattermost-team-edition:10.9.1",
		},
		{
			name:   "digest reference",
			ref:    "ghcr.io/dexidp/dex@sha256:abcdef",
			prefix: "rgy01.kalos.tdmc",
			want:   "rgy01.kalos.tdmc/ghcr.io/dexidp/dex@sha256:abcdef",
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
