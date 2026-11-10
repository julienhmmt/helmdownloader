package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"

	"github.com/julienhmmt/helmdownloader/pkg/log"
)

// stallingRegistry wraps an in-memory registry. While stall is set, blob GETs
// get a 200 with headers flushed and then never send a body byte.
type stallingRegistry struct {
	inner http.Handler
	stall atomic.Bool
}

func (s *stallingRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.stall.Load() && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/sha256:") {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
		return
	}
	s.inner.ServeHTTP(w, r)
}

// newStallFixture starts a registry with one pushed 1 MiB image and returns
// the image ref plus the stall switch. Stall is off when it returns.
func newStallFixture(t *testing.T) (string, *atomic.Bool) {
	t.Helper()
	reg := &stallingRegistry{inner: ggcrregistry.New()}
	srv := httptest.NewServer(reg)
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")
	ref := host + "/test/img:1"

	img, err := random.Image(1<<20, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	if err := crane.Push(img, ref); err != nil {
		t.Fatalf("push: %v", err)
	}
	return ref, &reg.stall
}

func TestStallFixture_PullWorksWhenNotStalling(t *testing.T) {
	ref, _ := newStallFixture(t)
	dest := filepath.Join(t.TempDir(), "img.tar")

	p := NewPuller("linux/amd64", "", false, log.Discard())
	digest, err := p.Save(context.Background(), ref, "docker.io/test/img:1", dest, nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if digest == "" {
		t.Fatal("expected a digest")
	}
}

func TestSave_StalledBlobDoesNotReturnOnItsOwn(t *testing.T) {
	ref, stall := newStallFixture(t)
	dest := filepath.Join(t.TempDir(), "img.tar")
	stall.Store(true)

	const deadline = 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	p := NewPuller("linux/amd64", "", false, log.Discard())
	start := time.Now()
	_, err := p.Save(ctx, ref, "docker.io/test/img:1", dest, nil)
	elapsed := time.Since(start)

	t.Logf("Save returned after %v with err=%v", elapsed, err)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected Save to end only on the context deadline, got err=%v after %v", err, elapsed)
	}
	if elapsed < deadline-100*time.Millisecond {
		t.Fatalf("Save returned early after %v", elapsed)
	}
}
