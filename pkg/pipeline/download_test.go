package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienhmmt/helmdownloader/pkg/artifacthub"
	"github.com/julienhmmt/helmdownloader/pkg/config"
	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/julienhmmt/helmdownloader/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSaver records calls and optionally fails for refs in failRefs. It tracks
// the peak number of concurrent Save calls so tests can assert on parallelism.
// failUntil maps a ref to the number of times it should fail before succeeding,
// modelling transient errors that recover on retry.
type fakeSaver struct {
	failRefs  map[string]bool
	failUntil map[string]int
	delay     time.Duration

	mu       sync.Mutex
	inFlight int
	peak     int
	attempts map[string]int
}

func (f *fakeSaver) Save(ctx context.Context, srcRef, destRef, destPath string, onBytes registry.BytesFunc) (string, error) {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.peak {
		f.peak = f.inFlight
	}
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[srcRef]++
	attempt := f.attempts[srcRef]
	f.mu.Unlock()

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	if f.failRefs[srcRef] {
		return "", fmt.Errorf("boom: %s", srcRef)
	}
	if n, ok := f.failUntil[srcRef]; ok && attempt <= n {
		return "", fmt.Errorf("transient %d: %s", attempt, srcRef)
	}
	return "sha256:fake", nil
}

func (f *fakeSaver) attemptCount(ref string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[ref]
}

func newTestPipeline(saver imageSaver, concurrency int) *Pipeline {
	return &Pipeline{
		cfg:            config.Config{RegistryPrefix: "rgy.local", Concurrency: concurrency, Retries: 2},
		puller:         saver,
		logger:         log.Discard(),
		retryBaseDelay: time.Millisecond,
	}
}

func TestDownload_PreservesInputOrder(t *testing.T) {
	refs := []string{"a/x:1", "b/y:2", "c/z:3", "d/w:4"}
	saver := &fakeSaver{delay: 2 * time.Millisecond}
	pl := newTestPipeline(saver, 4)

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, failures)
	require.Len(t, entries, len(refs))
	for i, ref := range refs {
		assert.Equal(t, ref, entries[i].SourceRef, "entry %d out of order", i)
	}
}

func TestDownload_PartitionsFailures(t *testing.T) {
	refs := []string{"ok/one:1", "bad/two:2", "ok/three:3"}
	// Save sees the normalized pull ref; Docker Hub shorthand gains a host.
	saver := &fakeSaver{failRefs: map[string]bool{"docker.io/bad/two:2": true}}
	pl := newTestPipeline(saver, 2)

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil, nil)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Len(t, failures, 1)
	assert.Equal(t, "ok/one:1", entries[0].SourceRef)
	assert.Equal(t, "ok/three:3", entries[1].SourceRef)
	assert.Equal(t, "bad/two:2", failures[0].Ref)
	assert.Error(t, failures[0].Err)
}

func TestDownload_RespectsConcurrencyLimit(t *testing.T) {
	refs := make([]string, 12)
	for i := range refs {
		refs[i] = fmt.Sprintf("repo/img:%d", i)
	}
	saver := &fakeSaver{delay: 10 * time.Millisecond}
	pl := newTestPipeline(saver, 3)

	_, _, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, saver.peak, 3, "exceeded concurrency limit")
	assert.Greater(t, saver.peak, 1, "did not run in parallel")
}

func TestDownload_ReportsProgressOncePerImage(t *testing.T) {
	refs := []string{"a:1", "b:2", "c:3"}
	saver := &fakeSaver{}
	pl := newTestPipeline(saver, 4)

	var calls int32
	maxCurrent := int32(0)
	_, _, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs,
		func(current, total int, ref string, perr error) {
			atomic.AddInt32(&calls, 1)
			if int32(current) > atomic.LoadInt32(&maxCurrent) {
				atomic.StoreInt32(&maxCurrent, int32(current))
			}
			assert.Equal(t, len(refs), total)
		}, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(len(refs)), calls)
	assert.Equal(t, int32(len(refs)), maxCurrent)
}

func TestDownload_ResumeReusesExistingTarball(t *testing.T) {
	refs := []string{"repo/cached:1", "repo/fresh:2"}
	saver := &fakeSaver{}
	pl := newTestPipeline(saver, 2)
	pl.cfg.Resume = true
	workDir := t.TempDir()

	// Pre-seed a tarball + digest sidecar for the first ref, as a prior run would.
	imagesDir := filepath.Join(workDir, "images")
	require.NoError(t, os.MkdirAll(imagesDir, 0o755))
	cachedTar := filepath.Join(imagesDir, tarballName("repo/cached:1"))
	require.NoError(t, os.WriteFile(cachedTar, []byte("cached"), 0o644))
	require.NoError(t, os.WriteFile(cachedTar+".digest", []byte("sha256:cached"), 0o644))

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: workDir}, refs, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, failures)
	require.Len(t, entries, 2)
	// The cached ref is reused (no Save call) and keeps its recorded digest.
	assert.Equal(t, 0, saver.attemptCount("docker.io/repo/cached:1"))
	assert.Equal(t, "sha256:cached", entries[0].Digest)
	// The fresh ref is pulled normally.
	assert.Equal(t, 1, saver.attemptCount("docker.io/repo/fresh:2"))
}

func TestDownload_RetriesTransientFailures(t *testing.T) {
	refs := []string{"flaky/img:1"}
	saver := &fakeSaver{failUntil: map[string]int{"docker.io/flaky/img:1": 2}}
	pl := newTestPipeline(saver, 1) // retries default to 2 -> 3 attempts

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, failures)
	require.Len(t, entries, 1)
	assert.Equal(t, 3, saver.attemptCount("docker.io/flaky/img:1"))
}

func TestDownload_GivesUpAfterRetryBudget(t *testing.T) {
	refs := []string{"dead/img:1"}
	saver := &fakeSaver{failRefs: map[string]bool{"docker.io/dead/img:1": true}}
	pl := newTestPipeline(saver, 1)
	pl.cfg.Retries = 1 // 2 attempts total

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.Len(t, failures, 1)
	assert.Equal(t, 2, saver.attemptCount("docker.io/dead/img:1"))
}

func TestSaveWithRetry_StopsOnCancelledContext(t *testing.T) {
	saver := &fakeSaver{failRefs: map[string]bool{"x:1": true}}
	pl := newTestPipeline(saver, 1)
	pl.cfg.Retries = 5
	pl.retryBaseDelay = time.Hour // would block if backoff were reached

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pl.saveWithRetry(ctx, "x:1", "rgy.local/x:1", t.TempDir()+"/x.tar", nil)
	assert.Error(t, err)
	// One attempt runs, then the cancelled context aborts before any backoff.
	assert.Equal(t, 1, saver.attemptCount("x:1"))
}

func TestRetries_FloorsAtZero(t *testing.T) {
	pl := newTestPipeline(&fakeSaver{}, 1)
	pl.cfg.Retries = -3
	assert.Equal(t, 0, pl.retries())
	pl.cfg.Retries = 4
	assert.Equal(t, 4, pl.retries())
}

func TestBundle_RequiresAtLeastOneImage(t *testing.T) {
	pl := newTestPipeline(&fakeSaver{}, 1)
	_, err := pl.Bundle(Prepared{}, artifacthub.Package{Name: "c"}, "1.0.0", nil)
	assert.ErrorContains(t, err, "no images")
}

func TestTarballName_SanitizesUnsafeChars(t *testing.T) {
	assert.Equal(t, "quay.io_argoproj_argocd_v3.2.6.tar", tarballName("quay.io/argoproj/argocd:v3.2.6"))
	assert.Equal(t, "redis_7.tar", tarballName("redis:7"))
	assert.Equal(t, "ghcr.io_dexidp_dex_sha256_abc.tar", tarballName("ghcr.io/dexidp/dex@sha256:abc"))
}

func TestConcurrency_FloorsAtOne(t *testing.T) {
	pl := newTestPipeline(&fakeSaver{}, 0)
	assert.Equal(t, 1, pl.concurrency())
	pl = newTestPipeline(&fakeSaver{}, -5)
	assert.Equal(t, 1, pl.concurrency())
	pl = newTestPipeline(&fakeSaver{}, 8)
	assert.Equal(t, 8, pl.concurrency())
}
