package registry

import (
	"context"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOpts_AuthAddsOneOption(t *testing.T) {
	ctx := context.Background()
	noAuth := NewPuller("linux/amd64", "", false, log.Discard())
	withAuth := NewPuller("linux/amd64", "", true, log.Discard())
	base, err := noAuth.buildOpts(ctx)
	require.NoError(t, err)
	authed, err := withAuth.buildOpts(ctx)
	require.NoError(t, err)
	assert.Len(t, authed, len(base)+1, "auth should add exactly one crane option")
}

func TestBuildOpts_ProxyAndAuthTogether(t *testing.T) {
	ctx := context.Background()
	p := NewPuller("linux/amd64", "http://proxy:3128", true, log.Discard())
	opts, err := p.buildOpts(ctx)
	require.NoError(t, err)
	// platform + proxy transport + auth + context = 4
	assert.Len(t, opts, 4)
}

func TestBuildOpts_BadPlatformErrors(t *testing.T) {
	p := NewPuller("not/a/platform/extra", "", false, log.Discard())
	_, err := p.buildOpts(context.Background())
	assert.Error(t, err)
}
