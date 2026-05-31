package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/julienhmmt/helmdownloader/internal/config"
	"github.com/julienhmmt/helmdownloader/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSaver records calls and optionally fails for refs in failRefs. It tracks
// the peak number of concurrent Save calls so tests can assert on parallelism.
type fakeSaver struct {
	failRefs map[string]bool
	delay    time.Duration

	mu       sync.Mutex
	inFlight int
	peak     int
}

func (f *fakeSaver) Save(ctx context.Context, srcRef, destRef, destPath string) error {
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.peak {
		f.peak = f.inFlight
	}
	f.mu.Unlock()

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	if f.failRefs[srcRef] {
		return fmt.Errorf("boom: %s", srcRef)
	}
	return nil
}

func newTestPipeline(saver imageSaver, concurrency int) *Pipeline {
	return &Pipeline{
		cfg:    config.Config{RegistryPrefix: "rgy.local", Concurrency: concurrency},
		puller: saver,
		logger: log.Discard(),
	}
}

func TestDownload_PreservesInputOrder(t *testing.T) {
	refs := []string{"a/x:1", "b/y:2", "c/z:3", "d/w:4"}
	saver := &fakeSaver{delay: 2 * time.Millisecond}
	pl := newTestPipeline(saver, 4)

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil)
	require.NoError(t, err)
	assert.Empty(t, failures)
	require.Len(t, entries, len(refs))
	for i, ref := range refs {
		assert.Equal(t, ref, entries[i].SourceRef, "entry %d out of order", i)
	}
}

func TestDownload_PartitionsFailures(t *testing.T) {
	refs := []string{"ok/one:1", "bad/two:2", "ok/three:3"}
	saver := &fakeSaver{failRefs: map[string]bool{"bad/two:2": true}}
	pl := newTestPipeline(saver, 2)

	entries, failures, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil)
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

	_, _, err := pl.Download(context.Background(), Prepared{WorkDir: t.TempDir()}, refs, nil)
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
		})
	require.NoError(t, err)
	assert.Equal(t, int32(len(refs)), calls)
	assert.Equal(t, int32(len(refs)), maxCurrent)
}

func TestConcurrency_FloorsAtOne(t *testing.T) {
	pl := newTestPipeline(&fakeSaver{}, 0)
	assert.Equal(t, 1, pl.concurrency())
	pl = newTestPipeline(&fakeSaver{}, -5)
	assert.Equal(t, 1, pl.concurrency())
	pl = newTestPipeline(&fakeSaver{}, 8)
	assert.Equal(t, 8, pl.concurrency())
}
