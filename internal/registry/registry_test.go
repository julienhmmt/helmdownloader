package registry_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/julienhmmt/helmdownloader/internal/log"
	"github.com/julienhmmt/helmdownloader/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestSave_InvalidProxyErrorsBeforeNetwork(t *testing.T) {
	p := registry.NewPuller("linux/amd64", "://missing-scheme", log.Discard())
	err := p.Save(context.Background(), "redis:7", "rgy.local/redis:7",
		filepath.Join(t.TempDir(), "out.tar"))
	assert.ErrorContains(t, err, "proxy")
}

func TestNewPuller_DefaultsPlatform(t *testing.T) {
	// An empty platform defaults to linux/amd64, so an invalid platform string
	// is the only way Save reports a platform parse error. Confirm the default
	// path does not error on platform parsing by using a bad proxy to short out
	// before any network call.
	p := registry.NewPuller("", "://bad", log.Discard())
	err := p.Save(context.Background(), "redis:7", "rgy.local/redis:7",
		filepath.Join(t.TempDir(), "out.tar"))
	assert.ErrorContains(t, err, "proxy")
}
